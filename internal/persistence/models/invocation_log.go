package models

import "time"

// InvocationLog 是函数调用记录：只追加、不可变、不软删除（无 UpdatedAt/DeletedAt）。
// 主键 UUIDv7 由 RegisterIDCallbacks 全局回调统一生成。
type InvocationLog struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	FunctionID string    `gorm:"not null;index" json:"function_id"`
	RouteID    *string   `json:"route_id,omitempty"`
	TraceID    string    `json:"trace_id"`
	StatusCode int       `json:"status_code"`
	DurationMs int64     `json:"duration_ms"`
	Error      string    `json:"error"`
	CreatedAt  time.Time `json:"created_at"`
}
