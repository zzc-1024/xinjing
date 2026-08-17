package models

import "time"

// RefreshToken 的 granted_to 字段取值（记录该 refresh token 授权给谁）。
const (
	// GrantedToSelf 表示授权给用户本人使用。
	GrantedToSelf = "self"
	// GrantedToThirdParty 表示授权给第三方平台/客户端使用。
	GrantedToThirdParty = "third_party"
)

// RefreshToken 是长期「换发短期 JWT」的凭证。
// 只存 token 的 SHA-256 哈希（json:"-" 表示序列化时不输出），绝不明文存储。
// 它自身不携带任何业务信息：user_id、scopes、授权对象等都落在本表的各列里，
// 由服务端实时查询，从而支持「精确吊销、改权限即时生效」。
type RefreshToken struct {
	BaseModel
	UserID      string     `gorm:"not null;index" json:"user_id"`
	TokenHash   string     `gorm:"not null;uniqueIndex" json:"-"`
	GrantedTo   string     `gorm:"not null;default:self" json:"granted_to"`
	Audience    string     `json:"audience"` // granted_to=third_party 时的第三方标识
	Scopes      []string   `gorm:"serializer:json" json:"scopes"`
	ExpiresAt   time.Time  `json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	RotatedFrom string     `json:"rotated_from,omitempty"` // 被本 token 替换的旧 token ID
}
