// handlers/desktop.go - 桌面会话管理 API
package handlers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/remote-desktop/master-service/database"
	"github.com/remote-desktop/master-service/models"
	"github.com/remote-desktop/master-service/services"
)

type DesktopHandler struct {
	encryptor *services.EncryptionService
}

func NewDesktopHandler(encryptor *services.EncryptionService) *DesktopHandler {
	return &DesktopHandler{encryptor: encryptor}
}

type CreateDesktopRequest struct {
	DesktopEnv string
	Protocol   string `json:"protocol" binding:"required,oneof=vnc rdp spice"`
	Resolution string `json:"resolution" binding:"required"`
	VncBackend string `json:"vnc_backend" binding:"omitempty,oneof=turbovnc tigervnc"`
}

type DesktopResponse struct {
	ID             string                 `json:"id"`
	Protocol       string                 `json:"protocol"`
	VncBackend     string                 `json:"vnc_backend,omitempty"`
	Resolution     string                 `json:"resolution"`
	Status         string                 `json:"status"`
	Username       string                 `json:"username,omitempty"`
	HostID         string                 `json:"host_id"`
	HostIP         string                 `json:"host_ip"`
	HostName       string                 `json:"host_name"`
	Port           int                    `json:"port"`
	VncPassword    string                 `json:"vnc_password,omitempty"`
	ConnectionInfo map[string]interface{} `json:"connection_info,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
}

func (h *DesktopHandler) ListDesktops(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(string)
	role, _ := c.Get("role")
	roleStr := ""
	if r, ok := role.(string); ok {
		roleStr = r
	}

	query := database.DB
	if roleStr != "admin" {
		query = query.Where("user_id = ?", uid)
	}

	var sessions []models.Session
	if err := query.
		Preload("Host").
		Preload("User").
		Order("created_at DESC").
		Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]DesktopResponse, 0)
	for _, s := range sessions {
		dr := DesktopResponse{
			ID:         s.ID.String(),
			Protocol:   s.Protocol,
			Resolution: s.Resolution,
			Status:     s.Status,
			HostID:     s.HostID.String(),
			CreatedAt:  s.CreatedAt,
		}
		if roleStr == "admin" && s.User.ID != uuid.Nil {
			dr.Username = s.User.Username
		}
		if s.Host.ID != uuid.Nil {
			dr.HostIP = s.Host.IPAddress
			dr.HostName = s.Host.Hostname
		}
		if s.ConnectionInfo != "" {
			var connInfo map[string]interface{}
			if err := json.Unmarshal([]byte(s.ConnectionInfo), &connInfo); err == nil {
				dr.ConnectionInfo = connInfo
				if port, ok := connInfo["port"].(float64); ok {
					dr.Port = int(port)
				}
			}
		}
		result = append(result, dr)
	}

	c.JSON(http.StatusOK, result)
}

func (h *DesktopHandler) GetDesktopDetail(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(string)
	sessionID := c.Param("id")

	var session models.Session
	if err := database.DB.Where("id = ? AND user_id = ?", sessionID, uid).
		Preload("Host").
		First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "桌面会话不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	dr := DesktopResponse{
		ID:         session.ID.String(),
		Protocol:   session.Protocol,
		Resolution: session.Resolution,
		Status:     session.Status,
		HostID:     session.HostID.String(),
		HostIP:     session.Host.IPAddress,
		HostName:   session.Host.Hostname,
		CreatedAt:  session.CreatedAt,
	}

	if session.ConnectionInfo != "" {
		var connInfo map[string]interface{}
		if err := json.Unmarshal([]byte(session.ConnectionInfo), &connInfo); err == nil {
			dr.ConnectionInfo = connInfo
			if port, ok := connInfo["port"].(float64); ok {
				dr.Port = int(port)
			}
		}
	}

	c.JSON(http.StatusOK, dr)
}

func (h *DesktopHandler) CreateDesktop(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(string)

	// 获取用户信息
	var user models.User
	if err := database.DB.Where("id = ?", uid).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户信息查询失败"})
		return
	}
	linuxUser := user.Username

	var req CreateDesktopRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var host models.Host
	if err := database.DB.Where("status = ? AND current_sessions < max_sessions", "healthy").
		Order("current_sessions ASC").
		First(&host).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "当前无可用宿主机，请稍后再试"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "宿主机查询失败"})
		return
	}

	if host.SSHUsername == "" || host.SSHCredentialEncrypted == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "宿主机 SSH 凭据未配置，无法创建桌面"})
		return
	}

	var maxDisplay int
	database.DB.Raw("SELECT COALESCE(MAX(CAST(connection_info->>'display' AS INTEGER)), 0) FROM sessions WHERE host_id = ?", host.ID).Scan(&maxDisplay)
	display := maxDisplay + 1
	if display < 1 {
		display = 1
	}
	port := 5900 + display
	wsPort := 6100 + display

	vncPassword := generateRandomPassword(8)

	desktopEnv := req.DesktopEnv
	if desktopEnv == "" || desktopEnv == "gnome" {
		desktopEnv = "gnome"
	}

	vncBackend := req.VncBackend
	if vncBackend == "" {
		vncBackend = "turbovnc"
	}

	if err := h.startVNCOnHost(host, display, port, wsPort, req.Resolution, vncPassword, linuxUser, desktopEnv, vncBackend); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "启动 VNC 会话失败: " + err.Error()})
		return
	}

	uidUUID, _ := uuid.Parse(uid)
	connInfo := map[string]interface{}{
		"port":     port,
		"display":  display,
		"password": vncPassword,
		"url":      fmt.Sprintf("vnc://%s:%d", host.IPAddress, port),
		"web_url":  fmt.Sprintf("http://%s:%d/vnc.html?autoconnect=true&host=%s&port=%d&password=%s", host.IPAddress, wsPort, host.IPAddress, wsPort, vncPassword),
	}
	connInfoJSON, _ := json.Marshal(connInfo)

	session := models.Session{
		UserID:         uidUUID,
		HostID:         host.ID,
		Protocol:       req.Protocol,
		VncBackend:     vncBackend,
		Resolution:     req.Resolution,
		Status:         "running",
		ConnectionInfo: string(connInfoJSON),
	}

	if err := database.DB.Create(&session).Error; err != nil {
		_ = h.stopVNCOnHost(host, display, wsPort, linuxUser, vncBackend)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "会话记录创建失败"})
		return
	}

	database.DB.Model(&host).Update("current_sessions", gorm.Expr("current_sessions + 1"))

	c.JSON(http.StatusCreated, DesktopResponse{
		ID:             session.ID.String(),
		Protocol:       session.Protocol,
		VncBackend:     session.VncBackend,
		Resolution:     session.Resolution,
		Status:         session.Status,
		HostID:         host.ID.String(),
		HostIP:         host.IPAddress,
		HostName:       host.Hostname,
		Port:           port,
		VncPassword:    vncPassword,
		ConnectionInfo: connInfo,
		CreatedAt:      session.CreatedAt,
	})
}

func (h *DesktopHandler) CloseDesktop(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(string)
	sessionID := c.Param("id")

	var session models.Session
	if err := database.DB.Where("id = ? AND user_id = ?", sessionID, uid).
		Preload("Host").
		First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "桌面会话不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	var display int
	var wsPort int
	if session.ConnectionInfo != "" {
		var connInfo map[string]interface{}
		if err := json.Unmarshal([]byte(session.ConnectionInfo), &connInfo); err == nil {
			if d, ok := connInfo["display"].(float64); ok {
				display = int(d)
			}
			if p, ok := connInfo["port"].(float64); ok {
				wsPort = int(p) + 200
			}
		}
	}

	if display > 0 {
		_ = h.stopVNCOnHost(session.Host, display, wsPort, session.User.Username, session.VncBackend)
	}

	database.DB.Model(&session).Update("status", "terminated")

	if session.Host.CurrentSessions > 0 {
		database.DB.Model(&session.Host).Update("current_sessions", gorm.Expr("current_sessions - 1"))
	}

	c.JSON(http.StatusOK, gin.H{"message": "桌面会话已关闭"})
}

func (h *DesktopHandler) DeleteDesktop(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(string)
	role, _ := c.Get("role")
	roleStr := ""
	if r, ok := role.(string); ok {
		roleStr = r
	}
	sessionID := c.Param("id")

	var session models.Session
	query := database.DB.Where("id = ?", sessionID).Preload("Host").Preload("User")
	if roleStr != "admin" {
		query = query.Where("user_id = ?", uid)
	}
	if err := query.First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "桌面会话不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 清理宿主机上的残留进程和端口（即使状态是 terminated 也要清理，防止残留）
	var display int
	var wsPort int
	if session.ConnectionInfo != "" {
		var connInfo map[string]interface{}
		if err := json.Unmarshal([]byte(session.ConnectionInfo), &connInfo); err == nil {
			if d, ok := connInfo["display"].(float64); ok {
				display = int(d)
			}
			if p, ok := connInfo["port"].(float64); ok {
				wsPort = int(p) + 200
			}
		}
	}
	if display > 0 && session.Host.ID != uuid.Nil {
		_ = h.stopVNCOnHost(session.Host, display, wsPort, session.User.Username, session.VncBackend)
	}

	if err := database.DB.Delete(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "桌面记录及宿主机进程已清理"})
}

// BatchTerminateDesktops 批量关闭桌面
func (h *DesktopHandler) BatchTerminateDesktops(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(string)
	role, _ := c.Get("role")
	roleStr := ""
	if r, ok := role.(string); ok {
		roleStr = r
	}

	var req struct {
		IDs []string `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var success []string
	var failed []string

	for _, id := range req.IDs {
		var session models.Session
		query := database.DB.Where("id = ? AND status = ?", id, "running").Preload("Host").Preload("User")
		if roleStr != "admin" {
			query = query.Where("user_id = ?", uid)
		}
		if err := query.First(&session).Error; err != nil {
			failed = append(failed, id)
			continue
		}

		var display int
		var wsPort int
		if session.ConnectionInfo != "" {
			var connInfo map[string]interface{}
			if err := json.Unmarshal([]byte(session.ConnectionInfo), &connInfo); err == nil {
				if d, ok := connInfo["display"].(float64); ok {
					display = int(d)
				}
				if p, ok := connInfo["port"].(float64); ok {
					wsPort = int(p) + 200
				}
			}
		}

		if display > 0 {
			_ = h.stopVNCOnHost(session.Host, display, wsPort, session.User.Username, session.VncBackend)
		}

		database.DB.Model(&session).Update("status", "terminated")
		if session.Host.CurrentSessions > 0 {
			database.DB.Model(&session.Host).Update("current_sessions", gorm.Expr("current_sessions - 1"))
		}
		success = append(success, id)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "批量关闭完成",
		"success":      success,
		"failed":       failed,
		"successCount": len(success),
		"failedCount":  len(failed),
	})
}

// BatchDeleteDesktops 批量删除桌面（仅 terminated 状态）
func (h *DesktopHandler) BatchDeleteDesktops(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(string)
	role, _ := c.Get("role")
	roleStr := ""
	if r, ok := role.(string); ok {
		roleStr = r
	}

	var req struct {
		IDs []string `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var success []string
	var failed []string

	for _, id := range req.IDs {
		var session models.Session
		query := database.DB.Where("id = ? AND status = ?", id, "terminated").Preload("Host").Preload("User")
		if roleStr != "admin" {
			query = query.Where("user_id = ?", uid)
		}
		if err := query.First(&session).Error; err != nil {
			failed = append(failed, id)
			continue
		}

		// 清理宿主机残留
		var display int
		var wsPort int
		if session.ConnectionInfo != "" {
			var connInfo map[string]interface{}
			if err := json.Unmarshal([]byte(session.ConnectionInfo), &connInfo); err == nil {
				if d, ok := connInfo["display"].(float64); ok {
					display = int(d)
				}
				if p, ok := connInfo["port"].(float64); ok {
					wsPort = int(p) + 200
				}
			}
		}
		if display > 0 && session.Host.ID != uuid.Nil {
			_ = h.stopVNCOnHost(session.Host, display, wsPort, session.User.Username, session.VncBackend)
		}

		if err := database.DB.Delete(&session).Error; err != nil {
			failed = append(failed, id)
			continue
		}
		success = append(success, id)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "批量删除完成",
		"success":      success,
		"failed":       failed,
		"successCount": len(success),
		"failedCount":  len(failed),
	})
}

// startVNCOnHost 在宿主机上启动 VNC 会话（以 linuxUser 身份运行）

// // startVNCOnHost 在宿主机上启动 VNC 会话（以 linuxUser 身份运行）
func (h *DesktopHandler) startVNCOnHost(host models.Host, display, port, wsPort int, resolution, password, linuxUser, desktopEnv, vncBackend string) error {
	cred, err := h.encryptor.Decrypt(host.SSHCredentialEncrypted)
	if err != nil {
		return fmt.Errorf("解密凭据失败: %w", err)
	}

	var authMethods []ssh.AuthMethod
	if host.SSHAuthType == "password" {
		authMethods = append(authMethods, ssh.Password(cred))
	} else {
		signer, err := ssh.ParsePrivateKey([]byte(cred))
		if err != nil {
			return fmt.Errorf("解析私钥失败: %w", err)
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
		return fmt.Errorf("SSH 连接失败: %w", err)
	}
	defer client.Close()

	// 检查目标用户是否存在
	checkCmd := fmt.Sprintf("id %s", linuxUser)
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	output, err := session.CombinedOutput(checkCmd)
	session.Close()
	if err != nil {
		return fmt.Errorf("宿主机上不存在用户 %s: %s", linuxUser, string(output))
	}

	// 根据 VNC 后端选择工具和参数
	var vncBin string
	var vncPassBin string
	switch vncBackend {
	case "tigervnc":
		vncBin = "vncserver"
		vncPassBin = "vncpasswd"
	default: // turbovnc
		vncBin = "/opt/TurboVNC/bin/vncserver"
		vncPassBin = "/opt/TurboVNC/bin/vncpasswd"
	}

	var startCmdTemplate string
	if vncBackend == "tigervnc" {
		startCmdTemplate = fmt.Sprintf("su - %s -c '%s :%%d -geometry %%s -depth 24 -localhost no -SecurityTypes VncAuth >/dev/null 2>&1 && echo success'", linuxUser, vncBin)
	} else {
		startCmdTemplate = fmt.Sprintf("su - %s -c '%s :%%d -geometry %%s -depth 24 -securitytypes None,Vnc >/dev/null 2>&1 && echo success'", linuxUser, vncBin)
	}

	// 设置 VNC 密码（以 root 为目标用户创建）
	vncPassCmd := fmt.Sprintf("mkdir -p /home/%s/.vnc && echo '%s' | %s -f > /home/%s/.vnc/passwd && chown %s:%s /home/%s/.vnc/passwd && chmod 600 /home/%s/.vnc/passwd", linuxUser, password, vncPassBin, linuxUser, linuxUser, linuxUser, linuxUser, linuxUser)

	// 创建 xstartup 桌面环境启动脚本（使用 printf 避免 SSH heredoc 问题）
	var sessionCmd string
	switch desktopEnv {
	case "xfce":
		sessionCmd = "startxfce4"
	default:
		// GNOME
		sessionCmd = "gnome-session"
	}
	xstartupCmd := fmt.Sprintf("printf '%%s\n' '#!/bin/sh' 'unset SESSION_MANAGER' 'unset DBUS_SESSION_BUS_ADDRESS' 'exec %s' > /home/%s/.vnc/xstartup && chmod 755 /home/%s/.vnc/xstartup && chown %s:%s /home/%s/.vnc/xstartup", sessionCmd, linuxUser, linuxUser, linuxUser, linuxUser, linuxUser)

	session, err = client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	output, err = session.CombinedOutput(vncPassCmd)
	session.Close()
	if err != nil {
		return fmt.Errorf("设置 VNC 密码失败: %w, output: %s", err, string(output))
	}

	// 创建 xstartup
	session, err = client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	_, err = session.CombinedOutput(xstartupCmd)
	session.Close()
	if err != nil {
		return fmt.Errorf("创建 xstartup 失败: %w", err)
	}

	// 启动 vncserver（以目标用户身份）
	startCmd := fmt.Sprintf(startCmdTemplate, display, resolution)
	session, err = client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	output, err = session.CombinedOutput(startCmd)
	session.Close()
	if err != nil {
		return fmt.Errorf("启动 vncserver 失败: %w, output: %s", err, string(output))
	}

	// 启动 websockify（仍以 root 运行，需要监听端口）
	wsCmd := fmt.Sprintf("nohup websockify --web=/opt/noVNC --cert=/dev/null %d localhost:%d >/dev/null 2>&1 &", wsPort, port)
	session, err = client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	_, err = session.CombinedOutput(wsCmd)
	session.Close()
	if err != nil {
		return fmt.Errorf("启动 websockify 失败: %w", err)
	}

	return nil
}

// stopVNCOnHost 在宿主机上停止 VNC 会话
func (h *DesktopHandler) stopVNCOnHost(host models.Host, display, wsPort int, linuxUser, vncBackend string) error {
	cred, err := h.encryptor.Decrypt(host.SSHCredentialEncrypted)
	if err != nil {
		return fmt.Errorf("解密凭据失败: %w", err)
	}

	var authMethods []ssh.AuthMethod
	if host.SSHAuthType == "password" {
		authMethods = append(authMethods, ssh.Password(cred))
	} else {
		signer, err := ssh.ParsePrivateKey([]byte(cred))
		if err != nil {
			return fmt.Errorf("解析私钥失败: %w", err)
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
		return fmt.Errorf("SSH 连接失败: %w", err)
	}
	defer client.Close()

	// 停止 vncserver（以目标用户身份）
	var vncKillPath string
	if vncBackend == "tigervnc" {
		vncKillPath = "vncserver"
	} else {
		vncKillPath = "/opt/TurboVNC/bin/vncserver"
	}
	stopCmd := fmt.Sprintf("su - %s -c '%s -kill :%d' >/dev/null 2>&1 || true", linuxUser, vncKillPath, display)
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	_, _ = session.CombinedOutput(stopCmd)
	session.Close()

	// 停止 websockify
	killCmd := fmt.Sprintf("pkill -f 'websockify.*%d' >/dev/null 2>&1 || true", wsPort)
	session, err = client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	_, _ = session.CombinedOutput(killCmd)
	session.Close()

	return nil
}

func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		rand.Seed(time.Now().UnixNano())
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
