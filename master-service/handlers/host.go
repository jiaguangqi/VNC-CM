// handlers/host.go - 宿主机管理 API

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/remote-desktop/master-service/database"
	"github.com/remote-desktop/master-service/models"
	"github.com/remote-desktop/master-service/services"
)

// HostHandler 宿主机处理器
type HostHandler struct {
	encryptor *services.EncryptionService
}

// NewHostHandler 创建宿主机处理器
func NewHostHandler(encryptor *services.EncryptionService) *HostHandler {
	return &HostHandler{encryptor: encryptor}
}

// CreateHostRequest 添加宿主机请求
type CreateHostRequest struct {
	Hostname       string `json:"hostname" binding:"required"`
	IPAddress      string `json:"ip_address" binding:"required,ip"`
	OSType         string `json:"os_type" binding:"required,oneof=linux windows"`
	MaxSessions    int    `json:"max_sessions" binding:"required,min=1,max=1000"`
	SSHUsername    string `json:"ssh_username"`
	SSHPort        int    `json:"ssh_port" binding:"min=1,max=65535"`
	SSHAuthType    string `json:"ssh_auth_type" binding:"oneof=password key"`
	SSHCredential  string `json:"ssh_credential"` // 明文密码或私钥，加密后存储
	SSHPublicKey   string `json:"ssh_public_key"`
	Region         string `json:"region"`
	AZ             string `json:"az"`
	CPUCores       int    `json:"cpu_cores"`
	TotalRAMMB     int64  `json:"total_ram_mb"`
}

// CreateHost 添加新宿主机（仅管理员）
func (h *HostHandler) CreateHost(c *gin.Context) {
	var req CreateHostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 加密 SSH 凭据
	var encryptedCredential string
	if req.SSHCredential != "" {
		enc, err := h.encryptor.Encrypt(req.SSHCredential)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "凭据加密失败"})
			return
		}
		encryptedCredential = enc
	}

	// 生成 Agent Token（用于 Host Agent mTLS 注册）
	agentToken := uuid.New().String()

	host := models.Host{
		Hostname:               req.Hostname,
		IPAddress:              req.IPAddress,
		OSType:                 req.OSType,
		MaxSessions:            req.MaxSessions,
		CurrentSessions:        0,
		Status:                 "init",
		AgentToken:             agentToken,
		SSHUsername:            req.SSHUsername,
		SSHPort:                req.SSHPort,
		SSHAuthType:            req.SSHAuthType,
		SSHCredentialEncrypted: encryptedCredential,
		SSHPublicKey:           req.SSHPublicKey,
		Region:                 req.Region,
		AZ:                     req.AZ,
		CPUCores:               req.CPUCores,
		TotalRAMMB:             req.TotalRAMMB,
	}

	if err := database.DB.Create(&host).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "宿主机创建失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "宿主机添加成功",
		"host_id":     host.ID.String(),
		"agent_token": agentToken,
	})
}

// ListHosts 获取宿主机列表（支持分页和状态过滤）
func (h *HostHandler) ListHosts(c *gin.Context) {
	status := c.Query("status")
	region := c.Query("region")

	var hosts []models.Host
	query := database.DB.Model(&models.Host{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if region != "" {
		query = query.Where("region = ?", region)
	}

	if err := query.Find(&hosts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 不返回加密凭据
	type HostResponse struct {
		ID              string `json:"id"`
		Hostname        string `json:"hostname"`
		IPAddress       string `json:"ip_address"`
		OSType          string `json:"os_type"`
		MaxSessions     int    `json:"max_sessions"`
		CurrentSessions int    `json:"current_sessions"`
		Status          string `json:"status"`
		SSHUsername     string `json:"ssh_username,omitempty"`
		SSHPort         int    `json:"ssh_port,omitempty"`
		SSHAuthType     string `json:"ssh_auth_type,omitempty"`
		Region          string `json:"region"`
		AZ              string `json:"az"`
		CPUCores        int    `json:"cpu_cores"`
		TotalRAMMB      int64  `json:"total_ram_mb"`
		CreatedAt       string `json:"created_at"`
	}

	var resp []HostResponse
	for _, h := range hosts {
		resp = append(resp, HostResponse{
			ID:              h.ID.String(),
			Hostname:        h.Hostname,
			IPAddress:       h.IPAddress,
			OSType:          h.OSType,
			MaxSessions:     h.MaxSessions,
			CurrentSessions: h.CurrentSessions,
			Status:          h.Status,
			SSHUsername:     h.SSHUsername,
			SSHPort:         h.SSHPort,
			SSHAuthType:     h.SSHAuthType,
			Region:          h.Region,
			AZ:              h.AZ,
			CPUCores:        h.CPUCores,
			TotalRAMMB:      h.TotalRAMMB,
			CreatedAt:       h.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	c.JSON(http.StatusOK, gin.H{"hosts": resp})
}

// GetHost 获取单个宿主机详情
func (h *HostHandler) GetHost(c *gin.Context) {
	hostID := c.Param("id")
	var host models.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "宿主机不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":               host.ID.String(),
		"hostname":         host.Hostname,
		"ip_address":       host.IPAddress,
		"os_type":          host.OSType,
		"max_sessions":     host.MaxSessions,
		"current_sessions": host.CurrentSessions,
		"status":           host.Status,
		"ssh_username":     host.SSHUsername,
		"ssh_port":         host.SSHPort,
		"region":           host.Region,
		"az":               host.AZ,
	})
}

// UpdateHostRequest 更新宿主机请求
type UpdateHostRequest struct {
	MaxSessions   *int    `json:"max_sessions,omitempty"`
	Status        *string `json:"status,omitempty" binding:"omitempty,oneof=healthy full offline maintenance"`
	SSHUsername   *string `json:"ssh_username,omitempty"`
	SSHCredential *string `json:"ssh_credential,omitempty"` // 新凭据，加密后更新
	SSHPublicKey  *string `json:"ssh_public_key,omitempty"`
}

// UpdateHost 更新宿主机配置（仅管理员）
func (h *HostHandler) UpdateHost(c *gin.Context) {
	hostID := c.Param("id")
	var req UpdateHostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var host models.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "宿主机不存在"})
		return
	}

	updates := make(map[string]interface{})
	if req.MaxSessions != nil {
		updates["max_sessions"] = *req.MaxSessions
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.SSHUsername != nil {
		updates["ssh_username"] = *req.SSHUsername
	}
	if req.SSHPublicKey != nil {
		updates["ssh_public_key"] = *req.SSHPublicKey
	}
	if req.SSHCredential != nil && *req.SSHCredential != "" {
		enc, err := h.encryptor.Encrypt(*req.SSHCredential)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "凭据加密失败"})
			return
		}
		updates["ssh_credential_encrypted"] = enc
	}

	if err := database.DB.Model(&host).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "宿主机更新成功"})
}

// DeleteHost 删除宿主机（仅管理员）
func (h *HostHandler) DeleteHost(c *gin.Context) {
	hostID := c.Param("id")

	// Check if host exists and has running sessions
	var host models.Host
	if err := database.DB.First(&host, "id = ?", hostID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "宿主机不存在"})
		return
	}

	if host.CurrentSessions > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error": "该宿主机上有正在运行的桌面，无法删除",
			"running_sessions": host.CurrentSessions,
		})
		return
	}

	if err := database.DB.Where("id = ?", hostID).Delete(&models.Host{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "宿主机已删除"})
}
