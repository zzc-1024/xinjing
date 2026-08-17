// Package wasm 提供 Wasm 运行时内核：模块编译缓存、函数调用、标准 WASI 基础能力。
// 插件系统与云函数系统共享本内核：二者最终都以 wasm 模块形式运行。
// 不共用部分（插件的能力声明、云函数的执行器）通过 Capability 等接口在本包之上构建。
package wasm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Config 是 Wasm 运行时的配置。
type Config struct {
	// MaxMemoryPages 限制单个模块实例可申请的内存上限（1 页 = 64KB）。
	// 0 表示不限制（使用 wazero 默认值，通常 512 页 = 32MB）。
	MaxMemoryPages uint32
	// CallTimeout 是单次函数调用的超时；<=0 表示不设超时。
	CallTimeout time.Duration
}

// Runtime 封装 wazero 运行时：负责模块编译缓存、实例化与函数调用。
// 它是「共用部分」的核心——插件与云函数都经由它执行 wasm。
type Runtime struct {
	rt    wazero.Runtime
	cfg   Config
	mu    sync.RWMutex
	cache map[string]wazero.CompiledModule
}

// NewRuntime 创建运行时并实例化标准 WASI（wasi_snapshot_preview1）。
// 标准 WASI 提供最基础的宿主能力：时钟（clock_time_get）、随机数（random_get）、
// 标准输出（fd_write）等——这是「基础 WASI」的第一层。
func NewRuntime(ctx context.Context, cfg Config) (*Runtime, error) {
	rc := wazero.NewRuntimeConfig()
	if cfg.MaxMemoryPages > 0 {
		rc = rc.WithMemoryLimitPages(cfg.MaxMemoryPages)
	}
	rt := wazero.NewRuntimeWithConfig(ctx, rc)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("instantiate wasi: %w", err)
	}

	return &Runtime{rt: rt, cfg: cfg, cache: make(map[string]wazero.CompiledModule)}, nil
}

// Close 释放运行时及所有资源。
func (r *Runtime) Close(ctx context.Context) error {
	return r.rt.Close(ctx)
}

// Compile 编译 wasm 模块并缓存（同一 name 只编译一次，后续调用复用编译产物）。
func (r *Runtime) Compile(ctx context.Context, name string, wasmBytes []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.cache[name]; ok {
		return nil
	}
	compiled, err := r.rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("compile module %q: %w", name, err)
	}
	r.cache[name] = compiled
	return nil
}

// Call 实例化模块并调用其导出的函数。
// name 是 Compile 时的模块名；funcName 是 wasm 导出的函数名。
// args/results 使用 uint64 承载 wasm 的 i32/i64（按 wazero 约定）。
func (r *Runtime) Call(ctx context.Context, name, funcName string, args ...uint64) ([]uint64, error) {
	r.mu.RLock()
	compiled, ok := r.cache[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("module %q not compiled", name)
	}

	mod, err := r.rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("instantiate module %q: %w", name, err)
	}
	defer mod.Close(ctx)

	fn := mod.ExportedFunction(funcName)
	if fn == nil {
		return nil, fmt.Errorf("function %q not found in module %q", funcName, name)
	}

	if r.cfg.CallTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.cfg.CallTimeout)
		defer cancel()
	}
	return fn.Call(ctx, args...)
}

// NewHostModuleBuilder 暴露 wazero 的宿主模块构建器，供能力（Capability）注册宿主函数。
// 这是「共用部分」给「不共用部分」开的唯一扩展点：能力实现通过它把自己注入运行时。
func (r *Runtime) NewHostModuleBuilder(name string) wazero.HostModuleBuilder {
	return r.rt.NewHostModuleBuilder(name)
}
