package models

// User 是平台用户。
type User struct {
	BaseModel
	Name  string `gorm:"not null" json:"name"`
	Email string `gorm:"not null;uniqueIndex" json:"email"`
	// PasswordHash 存储密码的 bcrypt 加盐哈希（json:"-" 表示序列化时不输出，避免泄露）。
	// 只存哈希、绝不明文，校验由 auth 包的 VerifyPassword 完成。
	PasswordHash string `gorm:"not null" json:"-"`
}
