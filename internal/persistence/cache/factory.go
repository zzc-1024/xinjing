package cache

import (
	"fmt"

	"xinjing/internal/config"
)

// New 根据配置构造缓存实现；当前仅支持 memory，valkey 留待阶段 2。
func New(cfg *config.Config) (Cache, error) {
	switch cfg.CacheBackend {
	case "memory", "":
		return NewMemory(), nil
	default:
		return nil, fmt.Errorf("unsupported cache backend %q (支持: memory)", cfg.CacheBackend)
	}
}

// 保证编译期检查：Memory 实现满足 Cache 接口。
var _ Cache = (*Memory)(nil)
