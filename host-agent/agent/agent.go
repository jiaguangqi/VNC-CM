// agent/agent.go - Host Agent 核心逻辑 (WebSocket JSON 版)

package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/user"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
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
	writeMu    sync.Mutex
	started    bool
	seq        int64
	failOnce   sync.Once
	desktopMu  sync.Mutex
	desktops   map[string]*trackedDesktop
}

// protocol message types (对齐 master-service/grpc/server.go)
type agentMessage struct {
	Type      string          `json:"type"`
	HostID    string          `json:"host_id,omitempty"`
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
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

type trackedDesktop struct {
	SessionID  string
	WSPort     int
	LastBytes  uint64
	LastSample time.Time
	PeakBps    int64
}

type sessionBandwidthReportPayload struct {
	Sessions []sessionBandwidthSample `json:"sessions"`
}

type sessionBandwidthSample struct {
	SessionID    string `json:"session_id"`
	CurrentBps   int64  `json:"current_bps"`
	PeakBps      int64  `json:"peak_bps"`
	TotalBytes   uint64 `json:"total_bytes"`
	SampledAtUTC string `json:"sampled_at"`
}

type masterInstruction struct {
	InstructionID string          `json:"instruction_id"`
	Timestamp     int64           `json:"timestamp"`
	Type          string          `json:"type"` // create_desktop | terminate_desktop | update_config
	Payload       json.RawMessage `json:"payload"`
}

type createDesktopPayload struct {
	SessionID          string `json:"session_id"`
	Username           string `json:"username"`
	Protocol           string `json:"protocol"`
	Resolution         string `json:"resolution"`
	ColorDepth         int    `json:"color_depth"`
	Display            int    `json:"display"`
	Port               int    `json:"port"`
	WSPort             int    `json:"ws_port"`
	Password           string `json:"password"`
	DesktopEnv         string `json:"desktop_env"`
	VNCBackend         string `json:"vnc_backend"`
	PerformanceProfile string `json:"performance_profile"`
	VNCOptions         string `json:"vnc_options"`
	TimeoutMinutes     int    `json:"timeout_minutes"`
	RequireGPU         bool   `json:"require_gpu"`
	RequestedGPUCount  int    `json:"requested_gpu_count"`
}

type terminateDesktopPayload struct {
	SessionID  string `json:"session_id"`
	Username   string `json:"username"`
	Display    int    `json:"display"`
	WSPort     int    `json:"ws_port"`
	VNCBackend string `json:"vnc_backend"`
	Force      bool   `json:"force"`
}

type desktopUpdatePayload struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
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
		config:   cfg,
		stopCh:   make(chan struct{}),
		desktops: make(map[string]*trackedDesktop),
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
			a.sendBandwidthReport()
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
	if err := a.writeJSON(msg); err != nil {
		a.failConnection("发送心跳失败", err)
	}
}

// sendResourceReport 发送资源报告
func (a *Agent) sendResourceReport() {
	report := resourceReportPayload{
		CPUUsagePercent:      float32(getCPUUsage()),
		AvailableRAMMB:       getAvailableRAM(),
		ActiveSessions:       a.activeDesktopCount(),
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
	if err := a.writeJSON(msg); err != nil {
		a.failConnection("发送资源报告失败", err)
	}
}

func (a *Agent) sendBandwidthReport() {
	now := time.Now().UTC()
	samples := a.sampleBandwidth(now)
	if len(samples) == 0 {
		return
	}
	payload, _ := json.Marshal(sessionBandwidthReportPayload{Sessions: samples})
	msg := agentMessage{
		Type:      "session_bandwidth",
		HostID:    a.hostID,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}
	if err := a.writeJSON(msg); err != nil {
		a.failConnection("发送带宽报告失败", err)
	}
}

func (a *Agent) trackDesktop(payload createDesktopPayload) {
	if payload.Protocol != "vnc" || payload.WSPort <= 0 {
		return
	}
	a.desktopMu.Lock()
	defer a.desktopMu.Unlock()
	a.desktops[payload.SessionID] = &trackedDesktop{
		SessionID:  payload.SessionID,
		WSPort:     payload.WSPort,
		LastSample: time.Now().UTC(),
	}
}

func (a *Agent) untrackDesktop(sessionID string) {
	a.desktopMu.Lock()
	defer a.desktopMu.Unlock()
	delete(a.desktops, sessionID)
}

func (a *Agent) activeDesktopCount() int {
	a.desktopMu.Lock()
	defer a.desktopMu.Unlock()
	return len(a.desktops)
}

func (a *Agent) sampleBandwidth(now time.Time) []sessionBandwidthSample {
	a.desktopMu.Lock()
	defer a.desktopMu.Unlock()

	samples := make([]sessionBandwidthSample, 0, len(a.desktops))
	for _, desktop := range a.desktops {
		totalBytes, ok := sampleSocketBytes(desktop.WSPort)
		if !ok {
			samples = append(samples, sessionBandwidthSample{
				SessionID:    desktop.SessionID,
				CurrentBps:   0,
				PeakBps:      desktop.PeakBps,
				TotalBytes:   desktop.LastBytes,
				SampledAtUTC: now.Format(time.RFC3339),
			})
			continue
		}

		var currentBps int64
		elapsed := now.Sub(desktop.LastSample).Seconds()
		if elapsed > 0 && totalBytes >= desktop.LastBytes {
			currentBps = int64(float64(totalBytes-desktop.LastBytes) * 8 / elapsed)
		}
		if currentBps > desktop.PeakBps {
			desktop.PeakBps = currentBps
		}
		desktop.LastBytes = totalBytes
		desktop.LastSample = now

		samples = append(samples, sessionBandwidthSample{
			SessionID:    desktop.SessionID,
			CurrentBps:   currentBps,
			PeakBps:      desktop.PeakBps,
			TotalBytes:   totalBytes,
			SampledAtUTC: now.Format(time.RFC3339),
		})
	}
	return samples
}

var socketBytesPattern = regexp.MustCompile(`bytes_(?:sent|received):(\d+)`)

func sampleSocketBytes(wsPort int) (uint64, bool) {
	output, err := runShell(fmt.Sprintf("ss -tinH 'sport = :%d' 2>/dev/null || true", wsPort))
	if err != nil || output == "" {
		return 0, false
	}
	matches := socketBytesPattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return 0, false
	}
	var total uint64
	for _, match := range matches {
		value, err := strconv.ParseUint(match[1], 10, 64)
		if err == nil {
			total += value
		}
	}
	return total, true
}

func (a *Agent) writeJSON(v interface{}) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return a.conn.WriteJSON(v)
}

func (a *Agent) sendDesktopUpdate(sessionID, status, errMsg string) {
	payload, _ := json.Marshal(desktopUpdatePayload{SessionID: sessionID, Status: status, Error: errMsg})
	msg := agentMessage{
		Type:      "desktop_update",
		HostID:    a.hostID,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}
	if err := a.writeJSON(msg); err != nil {
		a.failConnection("发送桌面状态失败", err)
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
					a.failConnection("读取指令失败", err)
				}
			}
			a.handleInstruction(&inst)
		}
	}
}

func (a *Agent) failConnection(context string, err error) {
	a.failOnce.Do(func() {
		log.Printf("%s: %v; 退出并等待 systemd 重启重连", context, err)
		if a.conn != nil {
			_ = a.conn.Close()
		}
		os.Exit(1)
	})
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
		if err := a.CreateDesktop(payload); err != nil {
			log.Printf("创建桌面失败: %v", err)
			a.sendDesktopUpdate(payload.SessionID, "error", err.Error())
		} else {
			a.trackDesktop(payload)
			a.sendDesktopUpdate(payload.SessionID, "running", "")
		}

	case "terminate_desktop":
		var payload terminateDesktopPayload
		if err := json.Unmarshal(inst.Payload, &payload); err != nil {
			log.Printf("解析 terminate_desktop 失败: %v", err)
			return
		}
		if err := a.TerminateDesktop(payload); err != nil {
			log.Printf("终止桌面失败: %v", err)
			a.sendDesktopUpdate(payload.SessionID, "error", err.Error())
		} else {
			a.untrackDesktop(payload.SessionID)
			a.sendDesktopUpdate(payload.SessionID, "terminated", "")
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
func (a *Agent) CreateDesktop(payload createDesktopPayload) error {
	exists, err := a.ValidateLocalUser(payload.Username)
	if err != nil || !exists {
		return fmt.Errorf("本地用户校验失败: %w", err)
	}

	switch payload.Protocol {
	case "vnc":
		return a.createVNCDesktop(payload)
	case "rdp":
		return a.createRDPDesktop(payload.SessionID, payload.Username, payload.Resolution, payload.ColorDepth)
	default:
		return fmt.Errorf("不支持的协议: %s", payload.Protocol)
	}
}

func (a *Agent) createVNCDesktop(payload createDesktopPayload) error {
	return a.createLinuxVNCDesktop(payload)
}

func (a *Agent) createRDPDesktop(sessionID, username, resolution string, colorDepth int) error {
	log.Printf("[RDP] 创建桌面 session=%s user=%s", sessionID, username)
	// TODO: Windows API / xrdp
	return nil
}

// TerminateDesktop 终止桌面会话
func (a *Agent) TerminateDesktop(payload terminateDesktopPayload) error {
	log.Printf("终止桌面 session=%s force=%v", payload.SessionID, payload.Force)
	return a.terminateLinuxVNCDesktop(payload)
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
