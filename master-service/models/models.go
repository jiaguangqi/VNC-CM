// models/models.go - GORM 数据模型定义

package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User 用户模型，支持本地与 LDAP 双源
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	Source       string    `gorm:"size:16;not null;default:'local'" json:"source"` // local, ldap
	Role         string    `gorm:"size:16;not null;default:'user'" json:"role"`    // user, admin, superadmin
	PasswordHash string    `gorm:"size:256" json:"-"`                              // 本地用户存储 bcrypt 哈希
	LDAPDN       string    `gorm:"size:512" json:"ldap_dn,omitempty"`              // LDAP 用户的 Distinguished Name
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// Host 桌面宿主机模型
type Host struct {
	ID                     uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Hostname               string    `gorm:"size:128;not null" json:"hostname"`
	IPAddress              string    `gorm:"size:45;not null" json:"ip_address"`
	OSType                 string    `gorm:"size:16;not null" json:"os_type"` // linux, windows
	MaxSessions            int       `gorm:"not null;default:10" json:"max_sessions"`
	CurrentSessions        int       `gorm:"not null;default:0" json:"current_sessions"`
	Status                 string    `gorm:"size:16;not null;default:'init'" json:"status"` // init, healthy, full, offline, maintenance
	AgentToken             string    `gorm:"size:256;uniqueIndex" json:"-"`                 // Host Agent mTLS 认证 Token
	SSHUsername            string    `gorm:"size:64" json:"ssh_username"`
	SSHPort                int       `gorm:"default:22" json:"ssh_port"`
	SSHAuthType            string    `gorm:"size:16" json:"ssh_auth_type"`                               // password, key
	SSHCredentialEncrypted string    `gorm:"type:text" json:"-"`                                         // AES-256-GCM 加密后的密码或私钥
	SSHPublicKey           string    `gorm:"type:text" json:"ssh_public_key,omitempty"`                   // 公钥指纹
	Region                 string    `gorm:"size:32" json:"region"`
	AZ                     string    `gorm:"size:32" json:"az"`
	CPUCores               int       `json:"cpu_cores"`
	TotalRAMMB             int64     `json:"total_ram_mb"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
	DeletedAt              gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Sessions []Session `gorm:"foreignKey:HostID" json:"-"`
}

// Session 远程桌面会话模型
type Session struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID        uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	HostID        uuid.UUID      `gorm:"type:uuid;not null;index" json:"host_id"`
	Protocol      string         `gorm:"size:16;not null" json:"protocol"` // vnc, rdp, spice, x2go, pcoip
	VncBackend    string         `gorm:"size:16;not null;default:'turbovnc'" json:"vnc_backend"` // turbovnc, tigervnc
	Resolution    string         `gorm:"size:16;default:'1920x1080'" json:"resolution"`
	ColorDepth    int            `gorm:"default:24" json:"color_depth"`
	Status        string         `gorm:"size:16;not null;default:'pending'" json:"status"` // pending, starting, running, idle, terminated, error
	ConnectionInfo string        `gorm:"type:jsonb" json:"connection_info,omitempty"`     // JSON 对象：port, password, native_url
	ExpiresAt     *time.Time     `json:"expires_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	User           User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Host           Host           `gorm:"foreignKey:HostID" json:"host,omitempty"`
	Collaborations []Collaboration `gorm:"foreignKey:SessionID" json:"-"`
}

// Collaboration 协同协助邀请模型
type Collaboration struct {
	ID           uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SessionID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"session_id"`
	OwnerID      uuid.UUID      `gorm:"type:uuid;not null" json:"owner_id"`
	InviteeID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"invitee_id"`
	Role         string         `gorm:"size:16;not null" json:"role"`      // viewer, controller
	Status       string         `gorm:"size:16;not null" json:"status"`    // pending, active, ended
	ShareToken   string         `gorm:"size:64;uniqueIndex" json:"share_token"` // 共享令牌（用于无密码 Web 访问）
	WSProxyPort  int            `json:"ws_proxy_port"`                      // 协作 WebSocket 代理端口
	HostIP       string         `gorm:"size:45" json:"host_ip"`             // 宿主节点 IP（缓存）
	VncPort      int            `json:"vnc_port"`                           // VNC 端口（缓存）
	VncPassword  string         `gorm:"size:64" json:"-"`                   // VNC 密码（加密存储，WebSocket 代理使用）
	CreatedAt    time.Time      `json:"created_at"`
	EndedAt      *time.Time     `json:"ended_at,omitempty"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Session Session `gorm:"foreignKey:SessionID" json:"session,omitempty"`
	Owner   User    `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Invitee User    `gorm:"foreignKey:InviteeID" json:"invitee,omitempty"`
}

// AuditLog 审计日志模型
type AuditLog struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ActorID   uuid.UUID `gorm:"type:uuid;index" json:"actor_id"`
	Action    string    `gorm:"size:64;not null;index" json:"action"`           // login, logout, desktop_create, desktop_access, collaboration_invite, admin_monitor, ssh_connect...
	TargetType string   `gorm:"size:32" json:"target_type"`                     // user, host, session, collaboration
	TargetID  uuid.UUID `gorm:"type:uuid;index" json:"target_id,omitempty"`
	Metadata  string    `gorm:"type:jsonb" json:"metadata,omitempty"`           // JSON 扩展字段
	IPAddress string    `gorm:"size:45" json:"ip_address"`
	Timestamp time.Time `gorm:"not null;default:now()" json:"timestamp"`
}

// TableName 指定 AuditLog 使用分区表（业务层创建分区）
func (AuditLog) TableName() string {
	return "audit_logs"
}

// BeforeCreate UUID 自动填充兜底
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (h *Host) BeforeCreate(tx *gorm.DB) error {
	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	return nil
}

func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

func (c *Collaboration) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

func (a *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
