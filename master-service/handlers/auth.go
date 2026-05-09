// handlers/auth.go - 用户认证相关 API
package handlers

import (
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/remote-desktop/master-service/config"
	"github.com/remote-desktop/master-service/database"
	"github.com/remote-desktop/master-service/middleware"
	"github.com/remote-desktop/master-service/models"
)

type AuthHandler struct {
	jwt    *middleware.JWTMiddleware
	config *config.LDAPConfig
}

func NewAuthHandler(jwtCfg *middleware.JWTMiddleware, ldapCfg *config.LDAPConfig) *AuthHandler {
	return &AuthHandler{jwt: jwtCfg, config: ldapCfg}
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6,max=128"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         UserInfo  `json:"user"`
}

type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Source   string `json:"source"`
}

func verifySystemPassword(username, password string) bool {
	cmd := exec.Command("/usr/bin/python3", "/app/auth-helper.py", username, password)
	err := cmd.Run()
	return err == nil
}

func (h *AuthHandler) issueToken(c *gin.Context, user models.User) {
	accessExpiry := 15
	refreshExpiry := 7
	accessToken, err := h.jwt.GenerateToken(user.ID.String(), user.Username, user.Role, accessExpiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token 签发失败"})
		return
	}
	refreshToken, err := h.jwt.GenerateRefreshToken(user.ID.String(), refreshExpiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Refresh Token 签发失败"})
		return
	}
	c.JSON(http.StatusOK, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresAt:    time.Now().Add(time.Duration(accessExpiry) * time.Minute),
		User: UserInfo{ID: user.ID.String(), Username: user.Username, Role: user.Role, Source: user.Source},
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var existing models.User
	if err := database.DB.Where("username = ?", req.Username).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}
	user := models.User{Username: req.Username, PasswordHash: string(hash), Source: "local", Role: "user"}
	if err := database.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户创建失败"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "注册成功", "user": UserInfo{ID: user.ID.String(), Username: user.Username, Role: user.Role, Source: user.Source}})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Username = strings.ToLower(req.Username)

	var user models.User
	err := database.DB.Where("username = ?", req.Username).First(&user).Error
	if err != nil {
		if verifySystemPassword(req.Username, req.Password) {
			hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
			user = models.User{Username: req.Username, PasswordHash: string(hash), Role: "user", Source: "system"}
			if err := database.DB.Create(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "系统用户同步失败"})
				return
			}
			h.issueToken(c, user)
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	authenticated := false
	switch user.Source {
	case "local", "system":
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err == nil {
			authenticated = true
		}
		if !authenticated && user.Source == "system" {
			if verifySystemPassword(req.Username, req.Password) {
				authenticated = true
				newHash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
				database.DB.Model(&user).Update("password_hash", string(newHash))
			}
		}
	case "ldap":
		c.JSON(http.StatusNotImplemented, gin.H{"error": "LDAP 登录暂未实现"})
		return
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "未知的用户来源"})
		return
	}

	if !authenticated {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	h.issueToken(c, user)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	claims, err := h.jwt.ParseRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Refresh Token 无效"})
		return
	}
	userID := claims.Subject
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}
	accessExpiry := 15
	accessToken, err := h.jwt.GenerateToken(user.ID.String(), user.Username, user.Role, accessExpiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token 签发失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": accessToken, "token_type": "Bearer", "expires_at": time.Now().Add(time.Duration(accessExpiry) * time.Minute)})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")
	role, _ := c.Get("role")
	c.JSON(http.StatusOK, gin.H{"user_id": userID, "username": username, "role": role})
}
