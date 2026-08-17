package models

// Route 把 API 路径 + 方法绑定到函数。
type Route struct {
	BaseModel
	FunctionID        string  `gorm:"not null;index" json:"function_id"`
	Path              string  `gorm:"not null" json:"path"`
	Method            string  `gorm:"not null" json:"method"`
	AuthRequired      bool    `json:"auth_required"`
	RateLimitPolicyID *string `json:"rate_limit_policy_id,omitempty"`
	Status            string  `json:"status"` // active / disabled
}

// RateLimitPolicy 是限流策略（token bucket 参数）。
type RateLimitPolicy struct {
	BaseModel
	Name       string `gorm:"not null" json:"name"`
	LimitCount int    `json:"limit_count"` // 窗口内允许的请求数
	WindowSec  int    `json:"window_sec"`  // 窗口时长（秒）
	Burst      int    `json:"burst"`       // 突发容量
	Scope      string `json:"scope"`       // per-key / per-route / global
}
