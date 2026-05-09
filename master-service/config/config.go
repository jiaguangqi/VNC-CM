// config/config.go - 主控服务配置定义

package config

import (
	"os"
	"strconv"
)

// Config 应用全局配置
type Config struct {
	Database   DatabaseConfig   // 数据库配置
	Server     ServerConfig     // HTTP/gRPC 服务配置
	JWT        JWTConfig        // JWT 令牌配置
	Encryption EncryptionConfig // 加密凭据配置
	LDAP       LDAPConfig       // LDAP 集成配置（可选）
}

// DatabaseConfig PostgreSQL 配置
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// ServerConfig HTTP 与 gRPC 监听配置
type ServerConfig struct {
	HTTPPort     string // HTTP REST API 端口
	GRPCPort     string // gRPC 端口（Host Agent 连接）
	ReadTimeout  int    // 读取超时（秒）
	WriteTimeout int    // 写入超时（秒）
}

// JWTConfig JWT 签发配置
type JWTConfig struct {
	Secret         string // 签名密钥
	AccessExpiry   int    // Access Token 有效期（分钟）
	RefreshExpiry  int    // Refresh Token 有效期（天）
	Issuer         string // 签发者标识
}

// EncryptionConfig AES-256-GCM 凭据加密配置
type EncryptionConfig struct {
	MasterKey string // 从环境变量 CREDENTIAL_MASTER_KEY 读取
}

// LDAPConfig LDAP/AD 对接配置
type LDAPConfig struct {
	Enabled    bool   // 是否启用 LDAP
	ServerURL  string // ldap:// 或 ldaps://
	BindDN     string // 绑定 DN
	BindPass   string // 绑定密码
	BaseDN     string // 搜索 Base DN
	UserFilter string // 用户搜索过滤器，如 (uid=%s)
}

// Load 从环境变量加载配置
func Load() *Config {
	return &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getIntEnv("DB_PORT", 5432),
			User:     getEnv("DB_USER", "rdp"),
			Password: getEnv("DB_PASSWORD", "rdp123"),
			DBName:   getEnv("DB_NAME", "remote_desktop"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Server: ServerConfig{
			HTTPPort:     getEnv("HTTP_PORT", ":8080"),
			GRPCPort:     getEnv("GRPC_PORT", ":9090"),
			ReadTimeout:  getIntEnv("READ_TIMEOUT", 30),
			WriteTimeout: getIntEnv("WRITE_TIMEOUT", 30),
		},
		JWT: JWTConfig{
			Secret:        getEnv("JWT_SECRET", "change-me-in-production-32byte!"),
			AccessExpiry:  getIntEnv("JWT_ACCESS_EXPIRY", 15),
			RefreshExpiry: getIntEnv("JWT_REFRESH_EXPIRY", 7),
			Issuer:        getEnv("JWT_ISSUER", "remote-desktop-platform"),
		},
		Encryption: EncryptionConfig{
			MasterKey: getEnv("CREDENTIAL_MASTER_KEY", ""),
		},
		LDAP: LDAPConfig{
			Enabled:    getBoolEnv("LDAP_ENABLED", false),
			ServerURL:  getEnv("LDAP_SERVER_URL", ""),
			BindDN:     getEnv("LDAP_BIND_DN", ""),
			BindPass:   getEnv("LDAP_BIND_PASS", ""),
			BaseDN:     getEnv("LDAP_BASE_DN", ""),
			UserFilter: getEnv("LDAP_USER_FILTER", "(uid=%s)"),
		},
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getIntEnv(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getBoolEnv(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultVal
}
