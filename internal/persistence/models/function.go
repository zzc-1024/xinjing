package models

// Function 是用户上传的云函数定义。
type Function struct {
	BaseModel
	UserID      string            `gorm:"not null;index" json:"user_id"`
	Name        string            `gorm:"not null" json:"name"`
	Runtime     string            `json:"runtime"` // wasm / go / ...
	Handler     string            `json:"handler"` // 入口函数名
	Description string            `json:"description"`
	EnvVars     map[string]string `gorm:"serializer:json" json:"env_vars"`
	TimeoutSec  int               `json:"timeout_sec"`
	MemoryMB    int               `json:"memory_mb"`
	Status      string            `json:"status"` // draft / active
}

// FunctionVersion 是函数的一个不可变版本，指向对象存储中的产物。
type FunctionVersion struct {
	BaseModel
	FunctionID  string `gorm:"not null;index" json:"function_id"`
	Version     string `gorm:"not null" json:"version"`
	ArtifactRef string `gorm:"not null" json:"artifact_ref"` // 对象存储中的 key
	Digest      string `gorm:"not null" json:"digest"`       // 产物 sha256
	BuildLog    string `json:"build_log"`
	Status      string `json:"status"` // building / ready / failed
}
