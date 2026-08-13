package middleware

import "net/http"

// Middleware 定义中间件签名：接收一个 Handler，返回一个增强后的 Handler
type Middleware func(http.Handler) http.Handler

// Chain 将多个中间件按顺序包裹到 handler 上
// 调用顺序：第一个中间件最先执行请求逻辑，最后执行响应逻辑（洋葱模型）
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	// 从后往前包裹，确保第一个中间件在最外层
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] != nil {
			h = middlewares[i](h)
		}
	}
	return h
}