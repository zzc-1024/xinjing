package wasm

import (
	"context"
	"testing"
	"time"
)

// answerWasm 是一个最小 wasm 模块：导出 answer 函数，返回 i32 常量 42。
// 对应 WAT：(module (func (export "answer") (result i32) i32.const 42))
var answerWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // magic + version
	0x01, 0x05, 0x01, 0x60, 0x00, 0x01, 0x7f, // type: () -> i32
	0x03, 0x02, 0x01, 0x00, // func: 1 个，类型 0
	0x07, 0x0a, 0x01, 0x06, 0x61, 0x6e, 0x73, 0x77, 0x65, 0x72, 0x00, 0x00, // export "answer"
	0x0a, 0x06, 0x01, 0x04, 0x00, 0x41, 0x2a, 0x0b, // code: i32.const 42; end
}

func newTestRuntime(t *testing.T) *Runtime {
	t.Helper()
	rt, err := NewRuntime(context.Background(), Config{
		MaxMemoryPages: 10,
		CallTimeout:    time.Second,
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close(context.Background()) })
	return rt
}

func TestRuntimeCompileAndCall(t *testing.T) {
	rt := newTestRuntime(t)
	ctx := context.Background()

	if err := rt.Compile(ctx, "answer", answerWasm); err != nil {
		t.Fatalf("Compile: %v", err)
	}
	results, err := rt.Call(ctx, "answer", "answer")
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if len(results) != 1 || results[0] != 42 {
		t.Fatalf("results = %v, want [42]", results)
	}
}

func TestRuntimeCallNotCompiled(t *testing.T) {
	rt := newTestRuntime(t)
	if _, err := rt.Call(context.Background(), "missing", "f"); err == nil {
		t.Fatalf("调用未编译模块应报错")
	}
}

func TestRuntimeCallUnknownFunction(t *testing.T) {
	rt := newTestRuntime(t)
	ctx := context.Background()
	if err := rt.Compile(ctx, "answer", answerWasm); err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if _, err := rt.Call(ctx, "answer", "nope"); err == nil {
		t.Fatalf("调用不存在函数应报错")
	}
}

func TestRuntimeCompileIdempotent(t *testing.T) {
	rt := newTestRuntime(t)
	ctx := context.Background()
	if err := rt.Compile(ctx, "answer", answerWasm); err != nil {
		t.Fatalf("Compile: %v", err)
	}
	// 再次编译同名模块应幂等成功
	if err := rt.Compile(ctx, "answer", answerWasm); err != nil {
		t.Fatalf("再次 Compile: %v", err)
	}
}

func TestRuntimeCompileInvalid(t *testing.T) {
	rt := newTestRuntime(t)
	if err := rt.Compile(context.Background(), "bad", []byte{0x00, 0x01, 0x02}); err == nil {
		t.Fatalf("编译非法 wasm 应报错")
	}
}
