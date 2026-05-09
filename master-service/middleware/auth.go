// middleware/auth.go - JWT 认证与 RBAC 权限中间件

package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTMiddleware 从 Authorization header 解析并验证 JWT
type JWTMiddleware struct {
	secret []byte
	issuer string
}

// Claims 自定义 JWT Claims
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"` // user, admin, superadmin
	jwt.RegisteredClaims
}

// NewJWTMiddleware 创建 JWT 中间件
func NewJWTMiddleware(secret, issuer string) *JWTMiddleware {
	return &JWTMiddleware{
		secret: []byte(secret),
		issuer: issuer,
	}
}

// AuthRequired 认证中间件，解析 token 并注入上下文
func (m *JWTMiddleware) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "缺少 Authorization 头"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization 格式错误，应为 Bearer {token}"})
			return
		}

		tokenStr := parts[1]
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return m.secret, nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token 无效或已过期"})
			return
		}

		if claims.Issuer != m.issuer {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token 签发者不匹配"})
			return
		}

		// 将用户信息注入 gin 上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// GenerateToken 签发 Access Token
func (m *JWTMiddleware) GenerateToken(userID, username, role string, expiryMinutes int) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiryMinutes) * time.Minute)),
			Issuer:    m.issuer,
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// GenerateRefreshToken 签发 Refresh Token
func (m *JWTMiddleware) GenerateRefreshToken(userID string, expiryDays int) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiryDays) * 24 * time.Hour)),
		Issuer:    m.issuer,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// RoleRequired RBAC 角色校验中间件工厂
func RoleRequired(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "未获取到用户角色"})
			return
		}

		roleStr, ok := role.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "角色解析失败"})
			return
		}

		for _, r := range allowedRoles {
			if r == roleStr {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "权限不足，需要角色: " + strings.Join(allowedRoles, "/")})
	}
}

// AdminOnly 仅管理员可访问的快捷中间件
func AdminOnly() gin.HandlerFunc {
	return RoleRequired("admin", "superadmin")
}

// ParseRefreshToken 解析 Refresh Token
func (m *JWTMiddleware) ParseRefreshToken(tokenStr string) (*jwt.RegisteredClaims, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, err
	}
	return claims, nil
}
