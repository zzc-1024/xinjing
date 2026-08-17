package wasm

import (
	"context"
	"testing"
)

// logCapability 是一个最小能力示例：向宿主输出日志。
// 它演示「不共用部分」如何通过 Capability 接口接入共用内核。
type logCapability struct{}

func (logCapability) Key() string { return "log" } // 官方：空 Provider

func (logCapability) Register(ctx context.Context, rt *Runtime) error {
	// 用宿主模块构建器注入一个宿主函数（此处仅演示骨架，不真正导出函数）。
	_ = rt.NewHostModuleBuilder("log")
	return nil
}

func TestRegistryAddAndDuplicate(t *testing.T) {
	r := NewRegistry()

	if err := r.Add(logCapability{}); err != nil {
		t.Fatalf("首次 Add: %v", err)
	}
	// 同 Key 重复登记应报错
	if err := r.Add(logCapability{}); err == nil {
		t.Fatalf("重复 Add 应报错")
	}
}

func TestCapabilityKeyString(t *testing.T) {
	// 官方：空 Provider → 裸名字
	k := CapabilityKey{Provider: "", Name: "log"}
	if k.String() != "log" {
		t.Fatalf("String() = %q, want log", k.String())
	}
	// 第三方：非空 → provider:name
	k2 := CapabilityKey{Provider: "acme", Name: "log"}
	if k2.String() != "acme:log" {
		t.Fatalf("String() = %q, want acme:log", k2.String())
	}
}

func TestRegistryRegisterAll(t *testing.T) {
	rt, err := NewRuntime(context.Background(), Config{})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	defer rt.Close(context.Background())

	r := NewRegistry()
	if err := r.Add(logCapability{}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := r.RegisterAll(context.Background(), rt); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
}
