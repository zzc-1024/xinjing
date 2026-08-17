package wasm

import (
	"context"
	"testing"
)

// logWasm 是一个调用宿主 log.log_write 的 wasm 模块。
// 对应 WAT：
//
//	(module
//	  (import "log" "log_write" (func $log_write (param i32 i32)))
//	  (memory (export "memory") 1)
//	  (data (i32.const 0) "hello from wasm")
//	  (func (export "run")
//	    i32.const 0
//	    i32.const 15
//	    call $log_write))
//
// run 把内存 0..15 的 15 字节（"hello from wasm"）交给宿主日志。
var logWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // magic + version
	0x01, 0x09, 0x02, 0x60, 0x02, 0x7f, 0x7f, 0x00, 0x60, 0x00, 0x00, // type: (i32,i32)->(), ()->()
	0x02, 0x11, 0x01, 0x03, 0x6c, 0x6f, 0x67, 0x09, 0x6c, 0x6f, 0x67, 0x5f, 0x77, 0x72, 0x69, 0x74, 0x65, 0x00, 0x00, // import log.log_write
	0x03, 0x02, 0x01, 0x01, // func: 1 个，类型 1
	0x05, 0x03, 0x01, 0x00, 0x01, // memory: min 1
	0x07, 0x10, 0x02, 0x06, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x02, 0x00, 0x03, 0x72, 0x75, 0x6e, 0x00, 0x01, // export memory, run
	0x0a, 0x0a, 0x01, 0x08, 0x00, 0x41, 0x00, 0x41, 0x0f, 0x10, 0x00, 0x0b, // code: run（section 10 在 data 之前）
	0x0b, 0x15, 0x01, 0x00, 0x41, 0x00, 0x0b, 0x0f, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x66, 0x72, 0x6f, 0x6d, 0x20, 0x77, 0x61, 0x73, 0x6d, // data "hello from wasm"
}

func TestLogCapabilityEndToEnd(t *testing.T) {
	rt, err := NewRuntime(context.Background(), Config{MaxMemoryPages: 10})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	defer rt.Close(context.Background())

	// 注入日志能力（捕获输出）
	var got string
	logCap := &LogCapability{Write: func(msg string) { got = msg }}
	reg := NewRegistry()
	if err := reg.Add(logCap); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := reg.RegisterAll(context.Background(), rt); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	// 编译并调用 wasm
	if err := rt.Compile(context.Background(), "guest", logWasm); err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if _, err := rt.Call(context.Background(), "guest", "run"); err != nil {
		t.Fatalf("Call: %v", err)
	}

	// 验证宿主收到了 wasm 内存里的字符串 —— 最小 ABI 全链路打通
	if got != "hello from wasm" {
		t.Fatalf("host got %q, want %q", got, "hello from wasm")
	}
}
