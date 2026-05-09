package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/remote-desktop/master-service/database"
	"github.com/remote-desktop/master-service/models"
	"github.com/remote-desktop/master-service/services"
)

type CollaborationHandler struct {
	encryptor *services.EncryptionService
}

func NewCollaborationHandler(encryptor *services.EncryptionService) *CollaborationHandler {
	return &CollaborationHandler{encryptor: encryptor}
}

type InviteRequest struct {
	SessionID       string `json:"session_id" binding:"required"`
	InviteeID       string `json:"invitee_id"`
	InviteeUsername string `json:"invitee_username"`
	Role            string `json:"role" binding:"required,oneof=viewer controller"`
}

type CollaborationResponse struct {
	ID         string                `json:"id"`
	SessionID  string                `json:"session_id"`
	OwnerID    string                `json:"owner_id"`
	InviteeID  string                `json:"invitee_id"`
	Role       string                `json:"role"`
	Status     string                `json:"status"`
	ShareToken string                `json:"share_token"`
	ShareURL   string                `json:"share_url"`
	Session    *CollaborationSession `json:"session,omitempty"`
	Owner      *UserBrief            `json:"owner,omitempty"`
	Invitee    *UserBrief            `json:"invitee,omitempty"`
	CreatedAt  time.Time             `json:"created_at"`
}

type UserBrief struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type CollaborationSession struct {
	ID             string                 `json:"id"`
	Protocol       string                 `json:"protocol"`
	Resolution     string                 `json:"resolution"`
	ColorDepth     int                    `json:"color_depth"`
	Port           int                    `json:"port,omitempty"`
	Status         string                 `json:"status"`
	Host           map[string]interface{} `json:"host,omitempty"`
	ConnectionInfo map[string]interface{} `json:"connection_info,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
}

func generateShareToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func toCollaborationResponse(c *models.Collaboration, baseURL string) CollaborationResponse {
	cr := CollaborationResponse{
		ID:         c.ID.String(),
		SessionID:  c.SessionID.String(),
		OwnerID:    c.OwnerID.String(),
		InviteeID:  c.InviteeID.String(),
		Role:       c.Role,
		Status:     c.Status,
		ShareToken: c.ShareToken,
		ShareURL:   fmt.Sprintf("%s/share/%s", baseURL, c.ShareToken),
		CreatedAt:  c.CreatedAt,
	}
	if c.Owner.ID != uuid.Nil {
		cr.Owner = &UserBrief{ID: c.Owner.ID.String(), Username: c.Owner.Username}
	}
	if c.Invitee.ID != uuid.Nil {
		cr.Invitee = &UserBrief{ID: c.Invitee.ID.String(), Username: c.Invitee.Username}
	}
	if c.Session.ID != uuid.Nil {
		var connInfo map[string]interface{}
		if c.Session.ConnectionInfo != "" {
			_ = json.Unmarshal([]byte(c.Session.ConnectionInfo), &connInfo)
		}
		port := c.VncPort
		if p, ok := connInfo["port"].(float64); ok {
			port = int(p)
		}
		cr.Session = &CollaborationSession{
			ID:         c.Session.ID.String(),
			Protocol:   c.Session.Protocol,
			Resolution: c.Session.Resolution,
			ColorDepth: c.Session.ColorDepth,
			Port:       port,
			Status:     c.Session.Status,
			CreatedAt:  c.Session.CreatedAt,
		}
		if c.Session.Host.ID != uuid.Nil {
			cr.Session.Host = map[string]interface{}{
				"id":         c.Session.Host.ID.String(),
				"hostname":   c.Session.Host.Hostname,
				"ip":         c.Session.Host.IPAddress,
				"ip_address": c.Session.Host.IPAddress,
			}
		}
		cr.Session.ConnectionInfo = connInfo
	}
	return cr
}

func getBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	// Prefer X-Forwarded-Host from reverse proxy, fallback to Origin/Referer, then Request.Host
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.GetHeader("Origin")
		if host != "" {
			// Remove scheme prefix from Origin
			host = strings.TrimPrefix(host, "http://")
			host = strings.TrimPrefix(host, "https://")
		}
	}
	if host == "" {
		host = c.Request.Host
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

func getUserID(c *gin.Context) (uuid.UUID, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, fmt.Errorf("未认证")
	}
	uidStr := userID.(string)
	return uuid.Parse(uidStr)
}

func normalizeProxyPath(proxyPath string) string {
	trimmed := strings.TrimPrefix(proxyPath, "/")
	if trimmed == "" {
		return ""
	}
	return "/" + trimmed
}

func getRequestHostPort(c *gin.Context) (string, string) {
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	if host == "" {
		return "", ""
	}

	if strings.Contains(host, ",") {
		host = strings.TrimSpace(strings.Split(host, ",")[0])
	}

	if parsedHost, parsedPort, err := net.SplitHostPort(host); err == nil {
		return parsedHost, parsedPort
	}

	parts := strings.SplitN(host, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return host, ""
}

func (h *CollaborationHandler) getCollaborationPassword(collab models.Collaboration) string {
	if collab.VncPassword == "" {
		return ""
	}
	password, err := h.encryptor.Decrypt(collab.VncPassword)
	if err != nil {
		return ""
	}
	return password
}

func (h *CollaborationHandler) Invite(c *gin.Context) {
	uid, err := getUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var req InviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话 ID"})
		return
	}

	var inviteeID uuid.UUID
	if req.InviteeID != "" {
		// Try parse as UUID first
		parsedID, err := uuid.Parse(req.InviteeID)
		if err == nil {
			inviteeID = parsedID
		} else {
			// Not a UUID, try as username
			var inviteeUser models.User
			if err := database.DB.Where("username = ?", req.InviteeID).First(&inviteeUser).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
				return
			}
			inviteeID = inviteeUser.ID
		}
	} else if req.InviteeUsername != "" {
		var inviteeUser models.User
		if err := database.DB.Where("username = ?", req.InviteeUsername).First(&inviteeUser).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		inviteeID = inviteeUser.ID
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供被邀请用户 ID 或用户名"})
		return
	}

	var session models.Session
	if err := database.DB.Where("id = ? AND status IN ? AND user_id = ?", sessionID, []string{"running", "active"}, uid).First(&session).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权邀请或会话不存在"})
		return
	}

	if inviteeID == uid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能邀请自己"})
		return
	}

	var existingCount int64
	database.DB.Model(&models.Collaboration{}).Where("session_id = ? AND invitee_id = ? AND status = ?", sessionID, inviteeID, "active").Count(&existingCount)
	if existingCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "已存在与该用户的活跃协作"})
		return
	}

	var host models.Host
	if err := database.DB.First(&host, "id = ?", session.HostID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询宿主机失败"})
		return
	}

	var connInfo map[string]interface{}
	if session.ConnectionInfo != "" {
		_ = json.Unmarshal([]byte(session.ConnectionInfo), &connInfo)
	}

	vncPort := 0
	if p, ok := connInfo["port"].(float64); ok {
		vncPort = int(p)
	}

	vncPassword := ""
	if pwd, ok := connInfo["password"].(string); ok && pwd != "" {
		encPwd, err := h.encryptor.Encrypt(pwd)
		if err == nil {
			vncPassword = encPwd
		}
	}

	token := generateShareToken()

	collab := models.Collaboration{
		SessionID:   sessionID,
		OwnerID:     uid,
		InviteeID:   inviteeID,
		Role:        req.Role,
		Status:      "active",
		ShareToken:  token,
		HostIP:      host.IPAddress,
		VncPort:     vncPort,
		VncPassword: vncPassword,
	}
	if err := database.DB.Create(&collab).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建协作记录失败"})
		return
	}

	database.DB.Preload("Session.Host").Preload("Owner").Preload("Invitee").First(&collab, "id = ?", collab.ID)
	c.JSON(http.StatusOK, toCollaborationResponse(&collab, getBaseURL(c)))
}

func (h *CollaborationHandler) ListMyInvites(c *gin.Context) {
	uid, err := getUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var collabs []models.Collaboration
	if err := database.DB.Where("owner_id = ? AND status = ?", uid, "active").
		Preload("Session.Host").Preload("Invitee").
		Order("created_at DESC").
		Find(&collabs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	base := getBaseURL(c)
	result := make([]CollaborationResponse, len(collabs))
	for i := range collabs {
		result[i] = toCollaborationResponse(&collabs[i], base)
	}
	c.JSON(http.StatusOK, result)
}

func (h *CollaborationHandler) ListInvited(c *gin.Context) {
	uid, err := getUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var collabs []models.Collaboration
	if err := database.DB.Where("invitee_id = ? AND status = ?", uid, "active").
		Preload("Session.Host").Preload("Owner").
		Order("created_at DESC").
		Find(&collabs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	base := getBaseURL(c)
	result := make([]CollaborationResponse, len(collabs))
	for i := range collabs {
		result[i] = toCollaborationResponse(&collabs[i], base)
	}
	c.JSON(http.StatusOK, result)
}

func (h *CollaborationHandler) Stop(c *gin.Context) {
	uid, err := getUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	id := c.Param("id")
	collabID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 ID"})
		return
	}

	var collab models.Collaboration
	if err := database.DB.First(&collab, "id = ? AND (owner_id = ? OR invitee_id = ?)", collabID, uid, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "协作记录不存在"})
		return
	}

	now := time.Now()
	collab.Status = "ended"
	collab.EndedAt = &now
	if err := database.DB.Save(&collab).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已停止协助", "id": collab.ID})
}

func (h *CollaborationHandler) ValidateToken(c *gin.Context) {
	token := c.Param("token")
	var collab models.Collaboration
	if err := database.DB.Where("share_token = ? AND status = ?", token, "active").
		Preload("Session").Preload("Owner").First(&collab).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "协作会话不存在或已结束"})
		return
	}
	c.JSON(http.StatusOK, toCollaborationResponse(&collab, getBaseURL(c)))
}

func (h *CollaborationHandler) ShareProxy(c *gin.Context) {
	token := c.Param("token")
	proxyPath := normalizeProxyPath(c.Param("path"))

	var collab models.Collaboration
	if err := database.DB.Where("share_token = ? AND status = ?", token, "active").First(&collab).Error; err != nil {
		// noVNC static resources: /share/app/..., /share/core/..., /share/vendor/...
		if token == "app" || token == "core" || token == "vendor" {
			staticPath := filepath.Join("/opt/noVNC", token, strings.TrimPrefix(proxyPath, "/"))
			c.File(staticPath)
			return
		}
		c.String(http.StatusForbidden, "协作会话不存在或已结束")
		return
	}

	wsProxyPort := collab.WSProxyPort
	if wsProxyPort == 0 && collab.VncPort > 0 {
		// websockify 端口 = VNC端口 + 200 (系统约定)
		wsProxyPort = collab.VncPort + 200
	}
	if wsProxyPort == 0 {
		// Fallback: try from session
		var session models.Session
		database.DB.First(&session, "id = ?", collab.SessionID)
		var ci map[string]interface{}
		if session.ConnectionInfo != "" {
			_ = json.Unmarshal([]byte(session.ConnectionInfo), &ci)
		}
		if p, ok := ci["port"].(float64); ok {
			wsProxyPort = int(p) + 200
		}
	}
	if wsProxyPort == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "协作代理未就绪"})
		return
	}

	if proxyPath == "" {
		query := c.Request.URL.Query()
		updated := false

		if query.Get("autoconnect") == "" {
			query.Set("autoconnect", "true")
			updated = true
		}
		if query.Get("reconnect") == "" {
			query.Set("reconnect", "true")
			updated = true
		}
		if query.Get("path") == "" {
			query.Set("path", fmt.Sprintf("share/%s/websockify", token))
			updated = true
		}
		requestHost, requestPort := getRequestHostPort(c)
		if query.Get("host") == "" && requestHost != "" {
			query.Set("host", requestHost)
			updated = true
		}
		if query.Get("port") == "" {
			port := requestPort
			if port == "" {
				if c.Request.TLS != nil {
					port = "443"
				} else {
					port = "80"
				}
			}
			query.Set("port", port)
			updated = true
		}
		if collab.Role == "viewer" && query.Get("view_only") == "" {
			query.Set("view_only", "true")
			updated = true
		}
		if password := h.getCollaborationPassword(collab); password != "" && query.Get("password") == "" {
			query.Set("password", password)
			updated = true
		}

		if updated {
			c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/share/%s?%s", token, query.Encode()))
			return
		}
	}

	backendURL := fmt.Sprintf("http://%s:%d", collab.HostIP, wsProxyPort)
	targetURL, _ := url.Parse(backendURL)

	director := func(req *http.Request) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		if proxyPath != "" {
			req.URL.Path = proxyPath
		} else {
			req.URL.Path = "/vnc.html"
		}
		req.Header.Del("Authorization")
	}

	proxy := &httputil.ReverseProxy{Director: director}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func (h *CollaborationHandler) proxyWebSocket(c *gin.Context, targetURL *url.URL, token, proxyPath string) {
	var collab models.Collaboration
	if err := database.DB.Where("share_token = ? AND status = ?", token, "active").First(&collab).Error; err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	wsProxyPort := collab.WSProxyPort
	if wsProxyPort == 0 && collab.VncPort > 0 {
		wsProxyPort = collab.VncPort + 200
	}
	if wsProxyPort == 0 {
		var session models.Session
		database.DB.First(&session, "id = ?", collab.SessionID)
		var ci map[string]interface{}
		if session.ConnectionInfo != "" {
			_ = json.Unmarshal([]byte(session.ConnectionInfo), &ci)
		}
		if p, ok := ci["port"].(float64); ok {
			wsProxyPort = int(p) + 200
		}
	}
	if wsProxyPort == 0 {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	wsPath := "/websockify"
	if proxyPath != "" {
		wsPath = proxyPath
	}
	wsURL := fmt.Sprintf("ws://%s:%d%s", collab.HostIP, wsProxyPort, wsPath)
	backendURL, _ := url.Parse(wsURL)

	upgrader := websocket.Upgrader{
		CheckOrigin:     func(r *http.Request) bool { return true },
		Subprotocols:    []string{"binary"},
		ReadBufferSize:  32 * 1024,
		WriteBufferSize: 32 * 1024,
	}

	clientConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer clientConn.Close()

	dialer := websocket.Dialer{
		Subprotocols:    []string{"binary"},
		ReadBufferSize:  32 * 1024,
		WriteBufferSize: 32 * 1024,
	}
	backendConn, _, err := dialer.Dial(backendURL.String(), nil)
	if err != nil {
		clientConn.Close()
		return
	}
	defer backendConn.Close()

	errChan := make(chan error, 2)

	go func() {
		for {
			mt, msg, err := clientConn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if err := backendConn.WriteMessage(mt, msg); err != nil {
				errChan <- err
				return
			}
		}
	}()

	go func() {
		for {
			mt, msg, err := backendConn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if err := clientConn.WriteMessage(mt, msg); err != nil {
				errChan <- err
				return
			}
		}
	}()

	<-errChan
}
