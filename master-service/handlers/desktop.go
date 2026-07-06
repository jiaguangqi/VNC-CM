// handlers/desktop.go - 桌面会话管理 API
package handlers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
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
	DesktopEnv string `json:"desktop_env" binding:"omitempty,oneof=gnome xfce"`
	Protocol   string `json:"protocol" binding:"required,oneof=vnc rdp spice"`
	Resolution string `json:"resolution" binding:"required"`
	ColorDepth int    `json:"color_depth" binding:"omitempty,oneof=8 16 24"`
	VncBackend string `json:"vnc_backend" binding:"omitempty,oneof=turbovnc tigervnc"`
	HostID     string `json:"host_id" binding:"omitempty,uuid"`
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
	if req.HostID != "" {
		if err := database.DB.
			Where("id = ? AND status = ? AND current_sessions < max_sessions", req.HostID, "healthy").
			First(&host).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "指定宿主机不可用或会话已满"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "宿主机查询失败"})
			return
		}
	} else {
		var hosts []models.Host
		if err := database.DB.Where("status = ? AND current_sessions < max_sessions", "healthy").
			Order("current_sessions ASC").
			Find(&hosts).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "宿主机查询失败"})
			return
		}

		if len(hosts) == 0 {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "当前无可用宿主机，请稍后再试"})
			return
		}

		minSessions := hosts[0].CurrentSessions
		var candidates []models.Host
		for _, h := range hosts {
			if h.CurrentSessions == minSessions {
				candidates = append(candidates, h)
			}
		}
		host = candidates[rand.Intn(len(candidates))]
	}

	if host.SSHUsername == "" || host.SSHCredentialEncrypted == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "宿主机 SSH 凭据未配置，无法创建桌面"})
		return
	}

	availableDisplays, err := services.AvailableDisplaysForHost(host.ID, host.MaxSessions)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	display, port, wsPort, err := h.selectAvailableDisplayAndPorts(host, availableDisplays)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	vncPassword := generateRandomPassword(8)

	desktopEnv := req.DesktopEnv
	if desktopEnv == "" || desktopEnv == "gnome" {
		desktopEnv = "gnome"
	}

	vncBackend := req.VncBackend
	if vncBackend == "" {
		vncBackend = "tigervnc"
	}

	colorDepth := req.ColorDepth
	if colorDepth == 0 {
		colorDepth = 24
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
		ColorDepth:     colorDepth,
		Status:         models.SessionStatusStarting,
		ConnectionInfo: string(connInfoJSON),
	}

	if err := database.DB.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "会话记录创建失败"})
		return
	}

	if err := h.startVNCOnHost(host, display, port, wsPort, req.Resolution, colorDepth, vncPassword, linuxUser, desktopEnv, vncBackend); err != nil {
		connInfo["error"] = err.Error()
		errorConnInfoJSON, _ := json.Marshal(connInfo)
		database.DB.Model(&session).Updates(map[string]interface{}{
			"status":          models.SessionStatusError,
			"connection_info": string(errorConnInfoJSON),
		})
		_, _ = h.stopVNCOnHost(host, display, wsPort, linuxUser, vncBackend)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "启动 VNC 会话失败: " + err.Error(), "session_id": session.ID.String()})
		return
	}

	if err := database.DB.Model(&session).Update("status", models.SessionStatusRunning).Error; err != nil {
		_, _ = h.stopVNCOnHost(host, display, wsPort, linuxUser, vncBackend)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "会话状态更新失败"})
		return
	}
	session.Status = models.SessionStatusRunning

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
		Preload("User").
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

	previousStatus := session.Status
	if previousStatus == models.SessionStatusTerminated {
		c.JSON(http.StatusOK, gin.H{"message": "桌面会话已关闭"})
		return
	}

	database.DB.Model(&session).Update("status", models.SessionStatusStopping)

	if display > 0 {
		_, _ = h.stopVNCOnHost(session.Host, display, wsPort, session.User.Username, session.VncBackend)
	}

	database.DB.Model(&session).Update("status", models.SessionStatusTerminated)

	if previousStatus == models.SessionStatusRunning && session.Host.CurrentSessions > 0 {
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

	previousStatus := session.Status
	if !models.IsTerminalSessionStatus(previousStatus) {
		database.DB.Model(&session).Update("status", models.SessionStatusStopping)
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
		_, _ = h.stopVNCOnHost(session.Host, display, wsPort, session.User.Username, session.VncBackend)
	}

	if previousStatus == models.SessionStatusRunning && session.Host.CurrentSessions > 0 {
		database.DB.Model(&session.Host).Update("current_sessions", gorm.Expr("current_sessions - 1"))
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
		query := database.DB.Where("id = ? AND status IN ?", id, []string{models.SessionStatusRunning, models.SessionStatusStarting}).Preload("Host").Preload("User")
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

		previousStatus := session.Status
		database.DB.Model(&session).Update("status", models.SessionStatusStopping)

		if display > 0 {
			_, _ = h.stopVNCOnHost(session.Host, display, wsPort, session.User.Username, session.VncBackend)
		}

		database.DB.Model(&session).Update("status", models.SessionStatusTerminated)
		if previousStatus == models.SessionStatusRunning && session.Host.CurrentSessions > 0 {
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
		query := database.DB.Where("id = ? AND status = ?", id, models.SessionStatusTerminated).Preload("Host").Preload("User")
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
			_, _ = h.stopVNCOnHost(session.Host, display, wsPort, session.User.Username, session.VncBackend)
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

func (h *DesktopHandler) selectAvailableDisplayAndPorts(host models.Host, displays []int) (int, int, int, error) {
	for _, display := range displays {
		port := 5900 + display
		wsPort := 6100 + display
		if err := h.ensureRemotePortsFree(host, port, wsPort); err == nil {
			return display, port, wsPort, nil
		}
	}
	return 0, 0, 0, fmt.Errorf("宿主机没有可用的 VNC/websockify 端口")
}

func (h *DesktopHandler) ensureRemotePortsFree(host models.Host, ports ...int) error {
	client, err := h.dialHostSSH(host)
	if err != nil {
		return err
	}
	defer client.Close()

	checks := make([]string, 0, len(ports))
	for _, port := range ports {
		checks = append(checks, fmt.Sprintf(`if command -v ss >/dev/null 2>&1; then ! ss -ltn | grep -Eq "[:.]%d[[:space:]]"; else ! netstat -ltn 2>/dev/null | grep -Eq "[:.]%d[[:space:]]"; fi`, port, port))
	}
	cmd := strings.Join(checks, " && ")

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH session 失败: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return fmt.Errorf("端口已占用或检查失败: %w, output: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (h *DesktopHandler) dialHostSSH(host models.Host) (*ssh.Client, error) {
	cred, err := h.encryptor.Decrypt(host.SSHCredentialEncrypted)
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

// startVNCOnHost 在宿主机上启动 VNC 会话（以 linuxUser 身份运行）
func (h *DesktopHandler) startVNCOnHost(host models.Host, display, port, wsPort int, resolution string, colorDepth int, password, linuxUser, desktopEnv, vncBackend string) error {
	client, err := h.dialHostSSH(host)
	if err != nil {
		return err
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
		startCmdTemplate = fmt.Sprintf("su - %s -c '%s :%%d -geometry %%s -depth %%d -SecurityTypes VncAuth >/dev/null 2>&1 && echo success'", linuxUser, vncBin)
	} else {
		startCmdTemplate = fmt.Sprintf("su - %s -c '%s :%%d -geometry %%s -depth %%d -securitytypes None,Vnc >/dev/null 2>&1 && echo success'", linuxUser, vncBin)
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
	startCmd := fmt.Sprintf(startCmdTemplate, display, resolution, colorDepth)
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

type desktopCleanupResult struct {
	RemoteConnected           bool     `json:"remote_connected"`
	VNCStopAttempted          bool     `json:"vnc_stop_attempted"`
	WebsockifyStopAttempted   bool     `json:"websockify_stop_attempted"`
	VNCStopOutput             string   `json:"vnc_stop_output,omitempty"`
	WebsockifyStopOutput      string   `json:"websockify_stop_output,omitempty"`
	NonFatalErrors            []string `json:"non_fatal_errors,omitempty"`
	TerminalConnectionFailure string   `json:"terminal_connection_failure,omitempty"`
}

// stopVNCOnHost 在宿主机上停止 VNC 会话。进程不存在视为清理成功，便于重复执行。
func (h *DesktopHandler) stopVNCOnHost(host models.Host, display, wsPort int, linuxUser, vncBackend string) (desktopCleanupResult, error) {
	result := desktopCleanupResult{}

	client, err := h.dialHostSSH(host)
	if err != nil {
		result.TerminalConnectionFailure = err.Error()
		return result, err
	}
	defer client.Close()
	result.RemoteConnected = true

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
		result.NonFatalErrors = append(result.NonFatalErrors, fmt.Sprintf("创建 VNC 清理 SSH session 失败: %v", err))
	} else {
		result.VNCStopAttempted = true
		output, cmdErr := session.CombinedOutput(stopCmd)
		result.VNCStopOutput = strings.TrimSpace(string(output))
		if cmdErr != nil {
			result.NonFatalErrors = append(result.NonFatalErrors, fmt.Sprintf("VNC 清理命令异常: %v", cmdErr))
		}
		session.Close()
	}

	// 停止 websockify
	killCmd := fmt.Sprintf("pkill -f 'websockify.*%d' >/dev/null 2>&1 || true", wsPort)
	session, err = client.NewSession()
	if err != nil {
		result.NonFatalErrors = append(result.NonFatalErrors, fmt.Sprintf("创建 websockify 清理 SSH session 失败: %v", err))
		return result, nil
	}
	result.WebsockifyStopAttempted = true
	output, cmdErr := session.CombinedOutput(killCmd)
	result.WebsockifyStopOutput = strings.TrimSpace(string(output))
	if cmdErr != nil {
		result.NonFatalErrors = append(result.NonFatalErrors, fmt.Sprintf("websockify 清理命令异常: %v", cmdErr))
	}
	session.Close()

	return result, nil
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
