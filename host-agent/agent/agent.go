// agent/agent.go - Host Agent 核心逻辑 (WebSocket JSON 版)

package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os/user"
	"runtime"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"net"
)

// Config Host Agent 配置
type Config struct {
	MasterAddr string // Master WebSocket 地址，如 ws://localhost:8080/ws/agent
	Hostname   string
	OSType     string // linux / windows
	Region     string
	AZ         string
}

// Agent Host Agent 结构体
type Agent struct {
	config     *Config
	conn       *websocket.Conn
	hostID     string
	agentToken string
	stopCh     chan struct{}
	started    bool
	seq        int64
}

// protocol message types (对齐 master-service/grpc/server.go)
type agentMessage struct {
	Type     string          `json:"type"`
	HostID   string          `json:"host_id,omitempty"`
	Timestamp int64          `json:"timestamp"`
	Payload  json.RawMessage `json:"payload"`
}

type registerPayload struct {
	Hostname    string `json:"hostname"`
	IPAddress   string `json:"ip_address"`
	OSType      string `json:"os_type"`
	CPUCores    int    `json:"cpu_cores"`
	TotalRAMMB  int64  `json:"total_ram_mb"`
	MaxSessions int    `json:"max_sessions"`
	Region      string `json:"region"`
	AZ          string `json:"az"`
}

type heartbeatPayload struct {
	Sequence int64 `json:"sequence"`
}

type resourceReportPayload struct {
	CPUUsagePercent      float32 `json:"cpu_usage_percent"`
	AvailableRAMMB       int64   `json:"available_ram_mb"`
	ActiveSessions       int     `json:"active_sessions"`
	GPUUsagePercent      float32 `json:"gpu_usage_percent"`
	AvailableGPUMemoryMB int64   `json:"available_gpu_memory_mb"`
	DiskUsagePercent     float32 `json:"disk_usage_percent"`
}

type masterInstruction struct {
	InstructionID string          `json:"instruction_id"`
	Timestamp     int64           `json:"timestamp"`
	Type          string          `json:"type"` // create_desktop | terminate_desktop | update_config
	Payload       json.RawMessage `json:"payload"`
}

type createDesktopPayload struct {
	SessionID         string `json:"session_id"`
	Username          string `json:"username"`
	Protocol          string `json:"protocol"`
	Resolution        string `json:"resolution"`
	ColorDepth        int    `json:"color_depth"`
	TimeoutMinutes    int    `json:"timeout_minutes"`
	RequireGPU        bool   `json:"require_gpu"`
	RequestedGPUCount int    `json:"requested_gpu_count"`
}

type terminateDesktopPayload struct {
	SessionID string `json:"session_id"`
	Force     bool   `json:"force"`
}

type registerResponse struct {
	Success    bool   `json:"success"`
	HostID     string `json:"host_id"`
	AgentToken string `json:"agent_token"`
	Error      string `json:"error,omitempty"`
}

// New 创建 Agent 实例
func New(cfg *Config) (*Agent, error) {
	return &Agent{
		config: cfg,
		stopCh: make(chan struct{}),
	}, nil
}

// Start 启动 Agent
func (a *Agent) Start() error {
	// 建立 WebSocket 连接
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.Dial(a.config.MasterAddr, nil)
	if err != nil {
		return fmt.Errorf("连接 Master WebSocket 失败: %w", err)
	}
	a.conn = conn

	// 发送注册消息
	if err := a.register(); err != nil {
		conn.Close()
		return err
	}

	log.Printf("Host 注册成功: host_id=%s", a.hostID)

	// 启动后台 goroutine
	go a.heartbeatLoop()
	go a.instructionLoop()

	a.started = true
	return nil
}

// Stop 优雅关闭
func (a *Agent) Stop() error {
	close(a.stopCh)
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}

// register 发送注册消息并等待响应
func (a *Agent) register() error {
	reg := registerPayload{
		Hostname:    a.config.Hostname,
		IPAddress:   getLocalIP(),
		OSType:      a.config.OSType,
		CPUCores:    getCPUCores(),
		TotalRAMMB:  getTotalRAM(),
		MaxSessions: 10, // 默认值，可通过配置覆盖
		Region:      a.config.Region,
		AZ:          a.config.AZ,
	}
	payload, _ := json.Marshal(reg)
	msg := agentMessage{
		Type:      "register",
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}

	if err := a.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("发送注册消息失败: %w", err)
	}

	// 读取响应
	var resp registerResponse
	if err := a.conn.ReadJSON(&resp); err != nil {
		return fmt.Errorf("读取注册响应失败: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("注册被拒绝: %s", resp.Error)
	}

	a.hostID = resp.HostID
	a.agentToken = resp.AgentToken
	return nil
}

// heartbeatLoop 周期性心跳上报
func (a *Agent) heartbeatLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.sendHeartbeat()
			a.sendResourceReport()
		case <-a.stopCh:
			return
		}
	}
}

// sendHeartbeat 发送心跳
func (a *Agent) sendHeartbeat() {
	a.seq++
	hb := heartbeatPayload{Sequence: a.seq}
	payload, _ := json.Marshal(hb)
	msg := agentMessage{
		Type:      "heartbeat",
		HostID:    a.hostID,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}
	if err := a.conn.WriteJSON(msg); err != nil {
		log.Printf("发送心跳失败: %v", err)
	}
}

// sendResourceReport 发送资源报告
func (a *Agent) sendResourceReport() {
	report := resourceReportPayload{
		CPUUsagePercent:      float32(getCPUUsage()),
		AvailableRAMMB:       getAvailableRAM(),
		ActiveSessions:       int(getActiveSessionCount()),
		GPUUsagePercent:      0, // TODO: 读取 GPU
		AvailableGPUMemoryMB: 0,
		DiskUsagePercent:     float32(getDiskUsage()),
	}
	payload, _ := json.Marshal(report)
	msg := agentMessage{
		Type:      "resource_report",
		HostID:    a.hostID,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}
	if err := a.conn.WriteJSON(msg); err != nil {
		log.Printf("发送资源报告失败: %v", err)
	}
}

// instructionLoop 监听 Master 下发的指令
func (a *Agent) instructionLoop() {
	for {
		select {
		case <-a.stopCh:
			return
		default:
			var inst masterInstruction
			if err := a.conn.ReadJSON(&inst); err != nil {
				select {
				case <-a.stopCh:
					return
				default:
					log.Printf("读取指令失败: %v", err)
					time.Sleep(2 * time.Second)
					continue
				}
			}
			a.handleInstruction(&inst)
		}
	}
}

// handleInstruction 处理 Master 下发的指令
func (a *Agent) handleInstruction(inst *masterInstruction) {
	log.Printf("收到指令: type=%s instruction_id=%s", inst.Type, inst.InstructionID)

	switch inst.Type {
	case "create_desktop":
		var payload createDesktopPayload
		if err := json.Unmarshal(inst.Payload, &payload); err != nil {
			log.Printf("解析 create_desktop 失败: %v", err)
			return
		}
		if err := a.CreateDesktop(payload.SessionID, payload.Username, payload.Protocol,
			payload.Resolution, payload.ColorDepth); err != nil {
			log.Printf("创建桌面失败: %v", err)
		}

	case "terminate_desktop":
		var payload terminateDesktopPayload
		if err := json.Unmarshal(inst.Payload, &payload); err != nil {
			log.Printf("解析 terminate_desktop 失败: %v", err)
			return
		}
		if err := a.TerminateDesktop(payload.SessionID, payload.Force); err != nil {
			log.Printf("终止桌面失败: %v", err)
		}

	case "update_config":
		log.Println("收到配置更新指令")
		// TODO: 应用新配置

	default:
		log.Printf("未知指令类型: %s", inst.Type)
	}
}

// ========= Desktop 生命周期管理（保留） =========

// CreateDesktop 在本地创建远程桌面会话
func (a *Agent) CreateDesktop(sessionID, username, protocol, resolution string, colorDepth int) error {
	exists, err := a.ValidateLocalUser(username)
	if err != nil || !exists {
		return fmt.Errorf("本地用户校验失败: %w", err)
	}

	switch protocol {
	case "vnc":
		return a.createVNCDesktop(sessionID, username, resolution, colorDepth)
	case "rdp":
		return a.createRDPDesktop(sessionID, username, resolution, colorDepth)
	default:
		return fmt.Errorf("不支持的协议: %s", protocol)
	}
}

func (a *Agent) createVNCDesktop(sessionID, username, resolution string, colorDepth int) error {
	log.Printf("[VNC] 创建桌面 session=%s user=%s", sessionID, username)
	// TODO: systemd-run --uid={username} vncserver...
	return nil
}

func (a *Agent) createRDPDesktop(sessionID, username, resolution string, colorDepth int) error {
	log.Printf("[RDP] 创建桌面 session=%s user=%s", sessionID, username)
	// TODO: Windows API / xrdp
	return nil
}

// TerminateDesktop 终止桌面会话
func (a *Agent) TerminateDesktop(sessionID string, force bool) error {
	log.Printf("终止桌面 session=%s force=%v", sessionID, force)
	// TODO: 查找进程并终止
	return nil
}

// ValidateLocalUser 校验本地 OS 用户是否存在
func (a *Agent) ValidateLocalUser(username string) (bool, error) {
	if runtime.GOOS == "windows" {
		return true, nil // TODO: Windows 实现
	}
	_, err := user.Lookup(username)
	if err != nil {
		return false, fmt.Errorf("用户 %s 不存在: %w", username, err)
	}
	return true, nil
}

// ========= 资源采集工具函数 =========

func getLocalIP() string {
	// 获取本机第一个非 loopback IPv4 地址
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range ifaces {
		// 跳过回环、未启用的接口
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				ip := v.IP
				if ip.IsLoopback() || ip.To4() == nil {
					continue
				}
				return ip.String()
			case *net.IPAddr:
				ip := v.IP
				if ip.IsLoopback() || ip.To4() == nil {
					continue
				}
				return ip.String()
			}
		}
	}
	return "127.0.0.1"
}

func getCPUCores() int {
	return runtime.NumCPU()
}

func getTotalRAM() int64 {
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err == nil {
		return int64(si.Totalram * uint64(si.Unit) / 1024 / 1024)
	}
	return 0
}

func getCPUUsage() float64 {
	return float64(rand.Intn(30)) + rand.Float64()
}

func getAvailableRAM() int64 {
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err == nil {
		return int64(si.Freeram * uint64(si.Unit) / 1024 / 1024)
	}
	return 0
}

func getDiskUsage() float64 {
	return float64(rand.Intn(50)) + rand.Float64()
}

func getActiveSessionCount() int32 {
	return 0 // TODO: 统计实际桌面会话数
}
