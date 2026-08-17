package wasm

import (
	"context"

	"github.com/tetratelabs/wazero/api"

	"xinjing/internal/logging"
)

// maxLogLen 限制单次日志消息的最大长度（防 wasm 恶意传超大长度）。
const maxLogLen = 4096

// LogCapability 是官方 "log" 能力（Provider 为空，裸名字 "log"）：让 wasm 模块向宿主输出日志文本。
//
// 这是「最小 ABI」的完整示例：wasm 侧通过线性内存传递数据（指针+长度），
// 宿主函数从沙箱内存读字节——验证了 wasm ↔ 宿主互调的完整链路。
type LogCapability struct {
	// Write 是注入的日志输出回调（便于测试捕获）；
	// nil 时写入统一日志模块（Info 级别）。
	Write func(msg string)
}

// Key 返回能力身份（官方：Provider 为空，裸名字 "log"）。
func (l *LogCapability) Key() string { return "log" }

// Register 注入宿主模块 "log"，导出函数 log_write(ptr, len)：
// wasm 调用它把内存 [ptr, ptr+len) 的字节交给宿主。
func (l *LogCapability) Register(ctx context.Context, rt *Runtime) error {
	_, err := rt.NewHostModuleBuilder("log").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
			ptr := api.DecodeU32(stack[0])
			length := api.DecodeU32(stack[1])
			if length > maxLogLen {
				length = maxLogLen
			}
			if length == 0 {
				return
			}
			// 从调用者（wasm 模块）的沙箱内存读取字节
			data, ok := m.Memory().Read(ptr, length)
			if !ok {
				logging.For("wasm").Warn("log_write read out of range", "ptr", ptr, "len", length)
				return
			}
			msg := string(data)
			if l.Write != nil {
				l.Write(msg)
				return
			}
			logging.For("wasm").Info("wasm log", "message", msg)
		}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{}).
		Export("log_write").
		Instantiate(ctx)
	return err
}
