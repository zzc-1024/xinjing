package logging

import (
	"fmt"
	"log/slog"
)

// PrintfLogger 把 slog 适配成「Printf 风格」的日志接口，
// 供 GORM、goose 等期望 Printf 方法的第三方库接入统一日志模块。
type PrintfLogger struct {
	Logger *slog.Logger
}

// Printf 以 Info 级别输出第三方库的日志。
func (a PrintfLogger) Printf(format string, args ...any) {
	a.Logger.Info(fmt.Sprintf(format, args...))
}

// Fatalf 记录 Error 后 panic（与标准库 log.Fatalf 的「终止进程」语义对齐）。
func (a PrintfLogger) Fatalf(format string, args ...any) {
	a.Logger.Error(fmt.Sprintf(format, args...))
	panic(fmt.Sprintf(format, args...))
}
