package logging

import (
	"context"
	"log/slog"
)

// requestCtxKey 是存放请求级日志上下文的 context key。
type requestCtxKey struct{}

// requestLogCtx 保存一次请求的 traceID 以及派生出的 logger。
type requestLogCtx struct {
	traceID string
	logger  *slog.Logger
}

// WithTraceID 将 traceID 提交到日志模块：注入请求上下文，并派生一个
// 自动携带 trace_id 字段的 logger。此后该请求内通过 FromContext 输出的
// 所有日志都会带上同一个 trace_id，保证同一请求的日志可关联、不会散开。
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, requestCtxKey{}, &requestLogCtx{
		traceID: traceID,
		logger:  Default().With("trace_id", traceID),
	})
}

// FromContext 返回当前请求的 logger（自动携带 trace_id）。
// 若不在请求上下文中，则回退到全局默认 logger。
func FromContext(ctx context.Context) *slog.Logger {
	if rc, ok := ctx.Value(requestCtxKey{}).(*requestLogCtx); ok && rc != nil {
		return rc.logger
	}
	return Default()
}

// TraceID 返回当前请求的 traceID，供业务逻辑按需读取。
func TraceID(ctx context.Context) string {
	if rc, ok := ctx.Value(requestCtxKey{}).(*requestLogCtx); ok && rc != nil {
		return rc.traceID
	}
	return ""
}
