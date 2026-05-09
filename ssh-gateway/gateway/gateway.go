// gateway/gateway.go - SSH Gateway 核心逻辑

package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"

	"github.com/golang-jwt/jwt/v5"
)

// Config SSH Gateway 配置
type Config struct {
	JWTSecret string
	WSPath    string
}

// SSHGateway WebSSH 网关
type SSHGateway struct {
	config      *Config
	upgrader    websocket.Upgrader
	ticketStore sync.Map // ssh_ticket -> TicketInfo 的内存缓存
}

// TicketInfo 从 Master 下发的连接票据信息
type TicketInfo struct {
	HostIP       string `json:"host_ip"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	AuthType     string `json:"auth_type"`     // password / key
	Credential   string `json:"credential"`    // 解密后的密码或私钥（仅内存持有）
	PublicKey    string `json:"public_key"`    // 公钥指纹
	ExpiresAt    int64  `json:"expires_at"`    // Unix timestamp
	AdminID      string `json:"admin_id"`
}

// ConnectionTicketClaims JWT 中的 claims
type ConnectionTicketClaims struct {
	HostID    string `json:"host_id"`
	AdminID   string `json:"admin_id"`
	Username  string `json:"username"`
	AuthType  string `json:"auth_type"`
	TicketID  string `json:"ticket_id"`
	jwt.RegisteredClaims
}

// New 创建 SSH Gateway
func New(cfg *Config) *SSHGateway {
	return &SSHGateway{
		config: cfg,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 生产环境应限制源域名
			},
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
		},
	}
}

// HandleWebSocket WebSocket 接入处理
func (g *SSHGateway) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 1. 从 query 参数获取 ssh_ticket
	ticketStr := r.URL.Query().Get("ticket")
	if ticketStr == "" {
		http.Error(w, `{"error":"缺少 ticket 参数"}`, http.StatusBadRequest)
		return
	}

	// 2. 验证 JWT ticket
	claims, err := g.validateTicket(ticketStr)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusUnauthorized)
		return
	}

	// 3. 从内存中查找 TicketInfo（Master 在签发 ticket 时已通过内部接口下发）
	ticketInfo, ok := g.getTicketInfo(claims.TicketID)
	if !ok {
		http.Error(w, `{"error":"ticket 已过期或不存在"}`, http.StatusUnauthorized)
		return
	}

	// 4. 升级 WebSocket
	ws, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v", err)
		return
	}
	defer ws.Close()

	// 5. 建立 SSH 连接
	sshClient, err := g.connectSSH(ticketInfo)
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("SSH 连接失败: %v\r\n", err)))
		return
	}
	defer sshClient.Close()

	// 6. 创建 SSH session
	sshSession, err := sshClient.NewSession()
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("SSH Session 创建失败: %v\r\n", err)))
		return
	}
	defer sshSession.Close()

	// 7. 申请 pty
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sshSession.RequestPty("xterm-256color", 80, 24, modes); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("PTY 申请失败: %v\r\n", err)))
		return
	}

	// 8. 启动 shell
	stdin, err := sshSession.StdinPipe()
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("StdinPipe 失败: %v\r\n", err)))
		return
	}
	stdout, err := sshSession.StdoutPipe()
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("StdoutPipe 失败: %v\r\n", err)))
		return
	}
	stderr, err := sshSession.StderrPipe()
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("StderrPipe 失败: %v\r\n", err)))
		return
	}

	if err := sshSession.Shell(); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Shell 启动失败: %v\r\n", err)))
		return
	}

	log.Printf("SSH WebSocket 会话已建立: admin=%s host=%s", ticketInfo.AdminID, ticketInfo.HostIP)

	// 9. 启动双向数据转发
	var wg sync.WaitGroup
	wg.Add(3)

	// WebSocket -> SSH stdin
	go func() {
		defer wg.Done()
		for {
			msgType, data, err := ws.ReadMessage()
			if err != nil {
				return
			}
			if msgType == websocket.TextMessage {
				// 命令黑名单检查
				if g.isBlacklisted(string(data)) {
					ws.WriteMessage(websocket.TextMessage, []byte("\r\n[WARNING] 该命令已被系统拦截\r\n"))
					continue
				}
				stdin.Write(data)
			}
		}
	}()

	// SSH stdout -> WebSocket
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				return
			}
			ws.WriteMessage(websocket.BinaryMessage, buf[:n])
		}
	}()

	// SSH stderr -> WebSocket
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				return
			}
			ws.WriteMessage(websocket.BinaryMessage, buf[:n])
		}
	}()

	wg.Wait()
	log.Printf("SSH WebSocket 会话已关闭: admin=%s host=%s", ticketInfo.AdminID, ticketInfo.HostIP)
}

// StoreTicket 由 Master Service 调用，存储 ticket 信息到内存
func (g *SSHGateway) StoreTicket(ticketID string, info *TicketInfo) {
	g.ticketStore.Store(ticketID, info)
	// 定时清理过期 ticket
	go func() {
		time.Sleep(60 * time.Second)
		g.ticketStore.Delete(ticketID)
	}()
}

func (g *SSHGateway) getTicketInfo(ticketID string) (*TicketInfo, bool) {
	val, ok := g.ticketStore.Load(ticketID)
	if !ok {
		return nil, false
	}
	info, ok := val.(*TicketInfo)
	return info, ok
}

// validateTicket 校验 JWT ssh_ticket
func (g *SSHGateway) validateTicket(tokenStr string) (*ConnectionTicketClaims, error) {
	claims := &ConnectionTicketClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(g.config.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("ticket 无效或已过期")
	}
	if time.Now().After(claims.ExpiresAt.Time) {
		return nil, fmt.Errorf("ticket 已过期")
	}
	return claims, nil
}

// connectSSH 建立到目标主机的 SSH 连接
func (g *SSHGateway) connectSSH(info *TicketInfo) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            info.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 生产环境应校验 known_hosts
		Timeout:         10 * time.Second,
	}

	switch info.AuthType {
	case "password":
		config.Auth = []ssh.AuthMethod{
			ssh.Password(info.Credential),
		}
	case "key":
		signer, err := ssh.ParsePrivateKey([]byte(info.Credential))
		if err != nil {
			return nil, fmt.Errorf("私钥解析失败: %w", err)
		}
		config.Auth = []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		}
	default:
		return nil, fmt.Errorf("不支持的认证类型: %s", info.AuthType)
	}

	addr := fmt.Sprintf("%s:%d", info.HostIP, info.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH 连接失败: %w", err)
	}
	return client, nil
}

// 命令黑名单（高危命令正则）
var commandBlacklist = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*rm\s+.*-rf\s*/`),
	regexp.MustCompile(`(?i)^\s*mkfs\.`),
	regexp.MustCompile(`(?i)^\s*dd\s+.*of=/dev/[sh]d`),
	regexp.MustCompile(`(?i)^\s*fdisk\s+/dev/[sh]d`),
	regexp.MustCompile(`(?i)^\s*>\s*/dev/[sh]d`),
}

func (g *SSHGateway) isBlacklisted(cmd string) bool {
	for _, re := range commandBlacklist {
		if re.MatchString(cmd) {
			return true
		}
	}
	return false
}
