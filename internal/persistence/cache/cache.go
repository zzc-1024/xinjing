// Package cache 提供缓存抽象：
// 业务代码只依赖 Cache 接口，后端可替换——
// 开发用 memory（进程内），生产用 valkey（多机共享，阶段 2 引入）。
package cache

import (
	"context"
	"errors"
	"time"
)

// ErrMiss 表示缓存未命中（key 不存在或已过期）。
var ErrMiss = errors.New("cache miss")

// Cache 是缓存的抽象接口。所有实现必须并发安全。
type Cache interface {
	// Get 返回 key 对应的值；未命中返回 ErrMiss。
	Get(ctx context.Context, key string) ([]byte, error)
	// Set 写入 key 对应的值；ttl<=0 表示永不过期。
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Delete 删除 key；不存在不报错（幂等）。
	Delete(ctx context.Context, key string) error
	// Incr 把 key 的整数值加 1 并返回新值；key 不存在时从 0 开始。
	// 内存实现会失败；阶段 2 限流场景由 valkey 提供。此处仅为接口预留。
	Incr(ctx context.Context, key string) (int64, error)
}

// Config 是缓存的初始化配置。
type Config struct {
	Backend string // 后端: memory(默认)
	// Valkey 相关配置预留（阶段 2 使用）
	ValkeyAddr string
}
