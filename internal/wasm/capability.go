package wasm

import (
	"context"
	"fmt"
)

// Capability 是「宿主能力」接口：插件与云函数通过它访问宿主资源（日志、存储、HTTP 等）。
//
// 这是「不共用部分」的接口骨架：
//   - 插件系统：每种插件能力是 Capability 的一个实现，插件声明自己需要哪些能力；
//   - 云函数系统：函数按声明获得一组能力，多租户隔离也在此层实现（每个实例独立的能力集合）。
//
// 设计约定（与中间件/插件身份体系一致）：
//   - 每种能力有 Provider + Name（如 xinjing:log），不同提供者可有同名能力；
//   - 未来插件上传时声明 Capabilities（models.Plugin.Capabilities 字段已预留）。
type Capability interface {
	// Key 返回能力唯一身份，形如 "xinjing:log"。
	Key() string
	// Register 把能力注册到运行时（通过 rt.NewHostModuleBuilder 注入宿主函数）。
	// 在模块实例化之前调用；同一 Key 重复注册由注册表按冲突策略处理（后续实现）。
	Register(ctx context.Context, rt *Runtime) error
}

// CapabilityKey 是能力身份 = Provider + Name。
type CapabilityKey struct {
	Provider string
	Name     string
}

// String 返回 "provider:name" 形式的唯一键。
func (k CapabilityKey) String() string {
	return k.Provider + ":" + k.Name
}

// Registry 是能力注册表：把能力按 Key 收集，统一注册到运行时。
// 默认冲突策略：同 Key 重复注册报错（与中间件的 ConflictError 一致）。
type Registry struct {
	byKey map[string]Capability
}

// NewRegistry 创建能力注册表。
func NewRegistry() *Registry {
	return &Registry{byKey: make(map[string]Capability)}
}

// Add 登记一个能力；同 Key 重复登记时返回错误（显式暴露冲突）。
func (r *Registry) Add(c Capability) error {
	key := c.Key()
	if _, exists := r.byKey[key]; exists {
		return fmt.Errorf("duplicate capability %q", key)
	}
	r.byKey[key] = c
	return nil
}

// RegisterAll 把已登记的所有能力注册到运行时。
func (r *Registry) RegisterAll(ctx context.Context, rt *Runtime) error {
	for _, c := range r.byKey {
		if err := c.Register(ctx, rt); err != nil {
			return fmt.Errorf("register capability %q: %w", c.Key(), err)
		}
	}
	return nil
}
