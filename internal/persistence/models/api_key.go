package models

import "time"

// API Key 的 status 字段取值常量。统一由本包定义，避免各处手写字符串。
const (
	// APIKeyStatusActive 表示密钥正常可用。
	APIKeyStatusActive = "active"
	// APIKeyStatusRevoked 表示密钥已被吊销，不再接受鉴权。
	APIKeyStatusRevoked = "revoked"
)

// APIKey 是用户的 API 访问凭证。
// KeyHash 只存密钥哈希（json:"-" 表示序列化时不输出，避免泄露）。
type APIKey struct {
	BaseModel
	UserID    string     `gorm:"not null;index" json:"user_id"`
	KeyHash   string     `gorm:"not null" json:"-"`
	Name      string     `json:"name"`
	Scopes    []string   `gorm:"serializer:json" json:"scopes"`
	Status    string     `gorm:"not null;default:active" json:"status"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
