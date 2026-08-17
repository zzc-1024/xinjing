package handler

import (
	"net/http"

	"xinjing/internal/auth"
	"xinjing/internal/response"
)

// meResponse 是 /me 的响应：返回当前请求的主体信息。
type meResponse struct {
	UserID     string   `json:"user_id"`
	AuthMethod string   `json:"auth_method"`
	Scopes     []string `json:"scopes"`
}

// Me 处理 GET /me：返回当前已认证用户的信息（由 auth.FromContext 读取）。
// 该路由应被 middleware.Authenticate 包裹，否则上下文里没有 Principal。
func Me(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.FromContext(r.Context())
	if !ok {
		// 理论上不会走到这里（中间件已拦截），防御性兜底
		response.Error(w, http.StatusUnauthorized, "authentication required")
		return
	}
	response.JSON(w, http.StatusOK, meResponse{
		UserID:     p.UserID,
		AuthMethod: string(p.AuthMethod),
		Scopes:     p.Scopes,
	})
}
