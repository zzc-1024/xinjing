// Package models 定义持久化层的 GORM 数据模型。
// 表结构由 migrate 包的 SQL 迁移创建；模型负责与数据库行做对象映射。
package models

import (
	"reflect"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel 是所有业务模型的公共字段：
// UUIDv7 主键 + 审计时间戳 + 软删除标记。
// 其他模型通过匿名嵌入它来复用这些字段。
type BaseModel struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// RegisterIDCallbacks 注册一个全局的 BeforeCreate 回调：
// 对所有「主键字段名为 ID、类型为 string、且当前为空」的模型，
// 在插入前自动生成时间有序的 UUIDv7。
//
// 为什么用全局回调而不是嵌入结构体的钩子？
// GORM 只调用「模型自身」的 BeforeCreate 方法，不会递归调用嵌入结构体的方法，
// 所以在 BaseModel 上定义钩子不会生效。全局回调能对所有模型一视同仁。
func RegisterIDCallbacks(db *gorm.DB) error {
	return db.Callback().Create().Before("gorm:create").Register("models:gen_uuid7", func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}
		// 拿到当前正在插入的模型值（*User、*APIKey 等）
		rv := reflect.ValueOf(tx.Statement.Dest)
		if rv.Kind() == reflect.Ptr {
			rv = rv.Elem()
		}
		if rv.Kind() != reflect.Struct {
			return
		}
		field := rv.FieldByName("ID")
		if !field.IsValid() || field.Kind() != reflect.String || !field.CanSet() {
			return
		}
		if field.String() == "" {
			field.SetString(uuid.Must(uuid.NewV7()).String())
		}
	})
}
