package cache

import (
	"context"
	"errors"
	"sync"
	"time"
)

// entry 是一个缓存条目，存值及其过期时间（零值表示永不过期）。
type entry struct {
	value     []byte
	expiresAt time.Time
}

// Memory 是进程内缓存：单个 map + 读写锁，仅用于单机开发。
// 多实例部署时各实例缓存互不可见；需要共享缓存请用 valkey 后端。
type Memory struct {
	mu sync.RWMutex
	m  map[string]entry
}

// NewMemory 创建进程内缓存。
func NewMemory() *Memory {
	return &Memory{m: make(map[string]entry)}
}

// Get 读取值；key 不存在或已过期时返回 ErrMiss。
func (c *Memory) Get(_ context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()

	if !ok {
		return nil, ErrMiss
	}
	// 已过期：返回 Miss（懒删除，由 Set/Delete 或后续触摸时清理）
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		return nil, ErrMiss
	}
	return e.value, nil
}

// Set 写入值并设置过期时间；ttl<=0 表示永不过期。
func (c *Memory) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	e := entry{value: value}
	if ttl > 0 {
		e.expiresAt = time.Now().Add(ttl)
	}
	c.mu.Lock()
	c.m[key] = e
	c.mu.Unlock()
	return nil
}

// Delete 删除 key；不存在也返回 nil（幂等）。
func (c *Memory) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	delete(c.m, key)
	c.mu.Unlock()
	return nil
}

// Incr 进程内实现的占位：返回未实现错误，避免限流逻辑误用单机缓存。
// 阶段 2 由 valkey 后端提供真正实现。
func (c *Memory) Incr(_ context.Context, _ string) (int64, error) {
	return 0, errors.New("incr is not supported by memory cache; use valkey backend")
}
