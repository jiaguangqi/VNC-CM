package services

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/remote-desktop/master-service/database"
	"github.com/remote-desktop/master-service/models"
)

type DesktopHealthMonitor struct {
	encryptor *EncryptionService
	interval  time.Duration
	stopCh    chan struct{}
}

func NewDesktopHealthMonitor(encryptor *EncryptionService, interval time.Duration) *DesktopHealthMonitor {
	if interval <= 0 {
		interval = time.Minute
	}
	return &DesktopHealthMonitor{
		encryptor: encryptor,
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

func (m *DesktopHealthMonitor) Start() {
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		m.reconcileActiveSessions()
		for {
			select {
			case <-ticker.C:
				m.reconcileActiveSessions()
			case <-m.stopCh:
				return
			}
		}
	}()
}

func (m *DesktopHealthMonitor) Stop() {
	close(m.stopCh)
}

func (m *DesktopHealthMonitor) reconcileActiveSessions() {
	var sessions []models.Session
	if err := database.DB.
		Where("status IN ?", []string{models.SessionStatusRunning, models.SessionStatusError}).
		Preload("Host").
		Preload("User").
		Find(&sessions).Error; err != nil {
		log.Printf("桌面健康检查查询失败: %v", err)
		return
	}

	for _, session := range sessions {
		if session.Host.AgentManaged && (session.Host.SSHUsername == "" || session.Host.SSHCredentialEncrypted == "") {
			continue
		}

		display, vncPort, wsPort, err := sessionDisplayAndPorts(session)
		if err != nil {
			if session.Status == models.SessionStatusRunning {
				m.markSessionError(session, fmt.Sprintf("连接信息无效: %v", err))
			}
			continue
		}

		if err := m.checkRemoteDesktop(session.Host, session.User.Username, display, vncPort, wsPort); err != nil {
			if session.Status == models.SessionStatusRunning {
				m.markSessionError(session, fmt.Sprintf("桌面进程健康检查失败: %v", err))
			}
			continue
		}

		if session.Status == models.SessionStatusError {
			m.markSessionRecovered(session)
		}
	}
}

func sessionDisplayAndPorts(session models.Session) (int, int, int, error) {
	if session.ConnectionInfo == "" {
		return 0, 0, 0, fmt.Errorf("connection_info 为空")
	}

	var connInfo map[string]interface{}
	if err := json.Unmarshal([]byte(session.ConnectionInfo), &connInfo); err != nil {
		return 0, 0, 0, err
	}

	displayValue, ok := connInfo["display"]
	if !ok {
		return 0, 0, 0, fmt.Errorf("缺少 display")
	}
	display, ok := numericJSONValueToInt(displayValue)
	if !ok || display <= 0 {
		return 0, 0, 0, fmt.Errorf("display 无效")
	}

	vncPortValue, ok := connInfo["port"]
	if !ok {
		return 0, 0, 0, fmt.Errorf("缺少 port")
	}
	vncPort, ok := numericJSONValueToInt(vncPortValue)
	if !ok || vncPort <= 0 {
		return 0, 0, 0, fmt.Errorf("port 无效")
	}

	wsPort := vncPort + 200
	if wsPortValue, hasWSPort := connInfo["ws_port"]; hasWSPort {
		wsPort, ok = numericJSONValueToInt(wsPortValue)
		if !ok || wsPort <= 0 {
			return 0, 0, 0, fmt.Errorf("ws_port 无效")
		}
	}

	return display, vncPort, wsPort, nil
}

func numericJSONValueToInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case json.Number:
		i, err := v.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

func (m *DesktopHealthMonitor) checkRemoteDesktop(host models.Host, username string, display, vncPort, wsPort int) error {
	client, err := m.dialHost(host)
	if err != nil {
		return err
	}
	defer client.Close()

	cmd := buildDesktopHealthCommand(username, display, vncPort, wsPort)
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return fmt.Errorf("%w, output: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (m *DesktopHealthMonitor) dialHost(host models.Host) (*ssh.Client, error) {
	cred, err := m.encryptor.Decrypt(host.SSHCredentialEncrypted)
	if err != nil {
		return nil, fmt.Errorf("解密凭据失败: %w", err)
	}

	var authMethods []ssh.AuthMethod
	if host.SSHAuthType == "password" {
		authMethods = append(authMethods, ssh.Password(cred))
	} else {
		signer, err := ssh.ParsePrivateKey([]byte(cred))
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	config := &ssh.ClientConfig{
		User:            host.SSHUsername,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host.IPAddress, host.SSHPort)
	if host.SSHPort == 0 {
		addr = fmt.Sprintf("%s:22", host.IPAddress)
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH 连接失败: %w", err)
	}
	return client, nil
}

func buildDesktopHealthCommand(username string, display, vncPort, wsPort int) string {
	userArg := shellQuote(username)
	displayArg := shellQuote(":" + strconv.Itoa(display))

	return fmt.Sprintf(`user=%s; display=%s; vncport=%d; wsport=%d; check_listen() { if command -v ss >/dev/null 2>&1; then ss -ltn | grep -Eq "[:.]$1[[:space:]]"; else netstat -ltn 2>/dev/null | grep -Eq "[:.]$1[[:space:]]"; fi; }; ps -u "$user" -o args= | grep -Ev "grep" | grep -E "(Xvnc|Xtigervnc|vncserver).*${display}" >/dev/null || { echo "vnc display ${display} not running"; exit 11; }; check_listen "$wsport" || { nohup websockify --web=/opt/noVNC --cert=/dev/null "$wsport" "localhost:$vncport" >/dev/null 2>&1 & sleep 1; }; check_listen "$wsport" || { echo "websockify port ${wsport} not listening"; exit 12; }`, userArg, displayArg, vncPort, wsPort)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (m *DesktopHealthMonitor) markSessionError(session models.Session, message string) {
	connInfo := make(map[string]interface{})
	if session.ConnectionInfo != "" {
		_ = json.Unmarshal([]byte(session.ConnectionInfo), &connInfo)
	}
	connInfo["error"] = message
	connInfo["last_health_check_at"] = time.Now().UTC().Format(time.RFC3339)
	connInfoJSON, _ := json.Marshal(connInfo)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.Session{}).
			Where("id = ? AND status = ?", session.ID, models.SessionStatusRunning).
			Updates(map[string]interface{}{
				"status":          models.SessionStatusError,
				"connection_info": string(connInfoJSON),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if session.Host.CurrentSessions > 0 {
			return tx.Model(&models.Host{}).
				Where("id = ? AND current_sessions > 0", session.HostID).
				Update("current_sessions", gorm.Expr("current_sessions - 1")).Error
		}
		return nil
	})
	if err != nil {
		log.Printf("标记桌面异常失败 session_id=%s: %v", session.ID, err)
		return
	}

	log.Printf("桌面健康检查异常 session_id=%s host=%s: %s", session.ID, session.Host.Hostname, message)
}

func (m *DesktopHealthMonitor) markSessionRecovered(session models.Session) {
	connInfo := make(map[string]interface{})
	if session.ConnectionInfo != "" {
		_ = json.Unmarshal([]byte(session.ConnectionInfo), &connInfo)
	}
	delete(connInfo, "error")
	connInfo["last_health_check_at"] = time.Now().UTC().Format(time.RFC3339)
	connInfoJSON, _ := json.Marshal(connInfo)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.Session{}).
			Where("id = ? AND status = ?", session.ID, models.SessionStatusError).
			Updates(map[string]interface{}{
				"status":          models.SessionStatusRunning,
				"connection_info": string(connInfoJSON),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return tx.Model(&models.Host{}).
			Where("id = ? AND current_sessions < max_sessions", session.HostID).
			Update("current_sessions", gorm.Expr("current_sessions + 1")).Error
	})
	if err != nil {
		log.Printf("恢复桌面运行状态失败 session_id=%s: %v", session.ID, err)
		return
	}

	log.Printf("桌面健康检查恢复 session_id=%s host=%s", session.ID, session.Host.Hostname)
}
