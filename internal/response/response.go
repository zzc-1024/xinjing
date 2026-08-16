// Package response 提供统一的 HTTP JSON 响应辅助函数。
// 所有 handler 的响应都应通过本包输出，保证格式一致。
package response

import (
	"encoding/json"
	"net/http"
)

// ErrorBody 是标准错误响应的 JSON 结构。
// 序列化后形如 {"error":{"code":404,"message":"not found"}}。
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail 包含错误的具体信息。
type ErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON 将 data 以 JSON 格式写入响应，状态码为 status。
// data 可以是任意类型（map、struct、slice 等），会被自动序列化。
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// Error 以统一 JSON 格式返回错误响应。
// 调用方只需传入状态码和消息，本函数负责组装成标准 ErrorBody 结构。
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, ErrorBody{
		Error: ErrorDetail{
			Code:    status,
			Message: message,
		},
	})
}
