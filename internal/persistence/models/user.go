package models

// User 是平台用户。
type User struct {
	BaseModel
	Name  string `gorm:"not null" json:"name"`
	Email string `gorm:"not null;uniqueIndex" json:"email"`
}
