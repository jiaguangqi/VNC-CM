// grpc/server.go - Master Service WebSocket Host Agent 服务端
// 使用 WebSocket 替代 gRPC，绕过 protoc 编译依赖，快速跑通核心链路

package grpc

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/remote-desktop/master-service/database"
	"github.com/remote-desktop/master-service/models"
)

// AgentMessage Host Agent 发来的 JSON 消息
type AgentMessage struct {
	HostID   string          `json:"host_id"`
	Timestamp int64          `json:"timestamp"`
	Type     string          `json:"type"` // "register" | "heartbeat" | "resource_report" | "desktop_update"
	Payload  json.RawMessage `json:"payload"`
}

// RegisterPayload 注册请求体
type RegisterPayload struct {
	Hostname    string   `json:"hostname"`
	IPAddress   string   `json:"ip_address"`
	OSType      string   `json:"os_type"`
	CPUCores    int      `json:"cpu_cores"`
	TotalRAMMB  int64    `json:"total_ram_mb"`
	MaxSessions int      `json:"max_sessions"`
	Region      string   `json:"region"`
	AZ          string   `json:"az"`
}

// HeartbeatPayload 心跳体
type HeartbeatPayload struct {
	Sequence int64 `json:"sequence"`
}

// ResourceReportPayload 资源报告体
type ResourceReportPayload struct {
	CPUUsagePercent      float32 `json:"cpu_usage_percent"`
	AvailableRAMMB       int64   `json:"available_ram_mb"`
	ActiveSessions       int     `json:"active_sessions"`
	GPUUsagePercent      float32 `json:"gpu_usage_percent"`
	AvailableGPUMemoryMB int64   `json:"available_gpu_memory_mb"`
	DiskUsagePercent     float32 `json:"disk_usage_percent"`
}

// MasterInstruction Master 下发的指令
type MasterInstruction struct {
	InstructionID string          `json:"instruction_id"`
	Timestamp     int64           `json:"timestamp"`
	Type          string          `json:"type"` // "create_desktop" | "terminate_desktop" | "update_config"
	Payload       json.RawMessage `json:"payload"`
}

// HostAgentServer WebSocket Agent 管理器
type HostAgentServer struct {
	mu         sync.RWMutex
	upgrader   websocket.Upgrader
	streams    map[string]*websocket.Conn // host_id -> ws conn
	hostStatus map[string]string          // host_id -> status
	cmdQueue   map[string]chan *MasterInstruction // host_id -> 指令队列
}

// NewHostAgentServer 创建代理服务器
func NewHostAgentServer() *HostAgentServer {
	return &HostAgentServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
		},
		streams:    make(map[string]*websocket.Conn),
		hostStatus: make(map[string]string),
		cmdQueue:   make(map[string]chan *MasterInstruction),
	}
}

// HandleWebSocket Agent WebSocket 接入入口
func (s *HostAgentServer) HandleWebSocket(c *gin.Context) {
	ws, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v", err)
		return
	}
	defer ws.Close()

	// 等待第一条 register 消息
	var firstMsg AgentMessage
	if err := ws.ReadJSON(&firstMsg); err != nil {
		log.Printf("读取注册消息失败: %v", err)
		return
	}
	if firstMsg.Type != "register" {
		ws.WriteJSON(gin.H{"error": "第一条消息必须是 register"})
		return
	}

	var reg RegisterPayload
	if err := json.Unmarshal(firstMsg.Payload, &reg); err != nil {
		ws.WriteJSON(gin.H{"error": "注册消息解析失败"})
		return
	}

	// 在数据库中查找或创建 Host 记录
	var host models.Host
	result := database.DB.Where("hostname = ?", reg.Hostname).First(&host)
	if result.Error != nil {
		// 新注册
		host = models.Host{
			Hostname:    reg.Hostname,
			IPAddress:   reg.IPAddress,
			OSType:      reg.OSType,
			CPUCores:    reg.CPUCores,
			TotalRAMMB:  reg.TotalRAMMB,
			MaxSessions: reg.MaxSessions,
			Status:      "healthy",
			AgentToken:  generateAgentToken(),
			Region:      reg.Region,
			AZ:          reg.AZ,
		}
		if err := database.DB.Create(&host).Error; err != nil {
			ws.WriteJSON(gin.H{"success": false, "error": err.Error()})
			return
		}
	} else {
		// 已存在，更新 IP 及其他信息
		database.DB.Model(&host).Updates(map[string]interface{}{
			"ip_address":   reg.IPAddress,
			"os_type":      reg.OSType,
			"cpu_cores":    reg.CPUCores,
			"total_ram_mb": reg.TotalRAMMB,
			"max_sessions": reg.MaxSessions,
			"status":       "healthy",
			"region":       reg.Region,
			"az":           reg.AZ,
		})
	}

	hostID := host.ID.String()

	// 注册到内存映射
	s.mu.Lock()
	s.streams[hostID] = ws
	s.hostStatus[hostID] = "healthy"
	s.cmdQueue[hostID] = make(chan *MasterInstruction, 10)
	s.mu.Unlock()

	// 响应注册成功
	ws.WriteJSON(gin.H{
		"success":     true,
		"host_id":     hostID,
		"agent_token": host.AgentToken,
	})

	log.Printf("Host 注册成功: host_id=%s hostname=%s", hostID, reg.Hostname)

	// 启动双工：读 + 写
	done := make(chan struct{})

	// 读循环
	go func() {
		defer close(done)
		for {
			var msg AgentMessage
			if err := ws.ReadJSON(&msg); err != nil {
				log.Printf("读取 Agent 消息失败 host_id=%s: %v", hostID, err)
				return
			}
			s.handleAgentMessage(hostID, &msg)
		}
	}()

	// 写循环：从指令队列推送给 Agent
	go func() {
		for {
			select {
			case <-done:
				return
			case cmd := <-s.cmdQueue[hostID]:
				if err := ws.WriteJSON(cmd); err != nil {
					log.Printf("下发指令失败 host_id=%s: %v", hostID, err)
					return
				}
			}
		}
	}()

	<-done
	s.unregisterHost(hostID)
}

// handleAgentMessage 处理 Agent 上报的消息
func (s *HostAgentServer) handleAgentMessage(hostID string, msg *AgentMessage) {
	switch msg.Type {
	case "heartbeat":
		var hb HeartbeatPayload
		json.Unmarshal(msg.Payload, &hb)
		log.Printf("心跳 host_id=%s seq=%d", hostID, hb.Sequence)
		// 更新最后活跃时间（可扩展存 Redis）

	case "resource_report":
		var report ResourceReportPayload
		json.Unmarshal(msg.Payload, &report)
		log.Printf("资源报告 host_id=%s cpu=%.1f%% sessions=%d",
			hostID, report.CPUUsagePercent, report.ActiveSessions)
		s.updateHostStatus(hostID, report.ActiveSessions)

	case "desktop_update":
		// 更新桌面状态
		log.Printf("桌面更新 host_id=%s payload=%s", hostID, string(msg.Payload))
		// TODO: 更新 sessions 表
	}
}

// unregisterHost Agent 断开时清理
func (s *HostAgentServer) unregisterHost(hostID string) {
	s.mu.Lock()
	delete(s.streams, hostID)
	delete(s.hostStatus, hostID)
	close(s.cmdQueue[hostID])
	delete(s.cmdQueue, hostID)
	s.mu.Unlock()

	database.DB.Model(&models.Host{}).Where("id = ?", hostID).Update("status", "offline")
	log.Printf("Host 离线: host_id=%s", hostID)
}

// updateHostStatus 更新宿主机资源状态
func (s *HostAgentServer) updateHostStatus(hostID string, activeSessions int) {
	var host models.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		return
	}
	updates := map[string]interface{}{"current_sessions": activeSessions}
	if activeSessions >= host.MaxSessions {
		updates["status"] = "full"
	} else if host.Status == "full" || host.Status == "offline" {
		updates["status"] = "healthy"
	}
	database.DB.Model(&host).Updates(updates)
}

// SendInstructionToHost 向指定宿主机下发指令（供 HTTP handler 调用）
func (s *HostAgentServer) SendInstructionToHost(hostID string, inst *MasterInstruction) error {
	s.mu.RLock()
	ch, ok := s.cmdQueue[hostID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("host %s 不在线", hostID)
	}
	select {
	case ch <- inst:
		return nil
	default:
		return fmt.Errorf("host %s 指令队列已满", hostID)
	}
}

// generateAgentToken 生成随机 Token
func generateAgentToken() string {
	return fmt.Sprintf("agent_%d", time.Now().UnixNano())
}
