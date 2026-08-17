package models

// Plugin 是一个可热插拔的插件定义。
type Plugin struct {
	BaseModel
	Name         string   `gorm:"not null" json:"name"`
	Version      string   `gorm:"not null" json:"version"`
	Digest       string   `gorm:"not null" json:"digest"` // 插件产物 sha256
	Capabilities []string `gorm:"serializer:json" json:"capabilities"`
	ConfigSchema string   `json:"config_schema"` // JSON Schema 文本
	Status       string   `json:"status"`        // active / disabled
}

// PluginInstance 是插件的启用实例，带运行时配置与执行顺序。
type PluginInstance struct {
	BaseModel
	PluginID  string         `gorm:"not null;index" json:"plugin_id"`
	Config    map[string]any `gorm:"serializer:json" json:"config"`
	Enabled   bool           `json:"enabled"`
	SortOrder int            `json:"sort_order"`
	Status    string         `json:"status"`
}
