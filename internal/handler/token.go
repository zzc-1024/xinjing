package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"xinjing/internal/auth"
	"xinjing/internal/logging"
	"xinjing/internal/persistence/models"
	"xinjing/internal/persistence/repo"
	"xinjing/internal/response"
)

// TokenHandler 处理 OAuth2 风格的两类令牌请求：
//   - POST /token   用密码登录，签发「短期 access + 长期 refresh」
//   - POST /refresh 用 refresh token 兑换新的短期 access token（并旋转 refresh token）
//
// 授权对象：本期仅支持用户本人（granted_to=self）；第三方授权留待后续阶段。
type TokenHandler struct {
	users       repo.UserRepository
	refreshRepo repo.RefreshTokenRepository
	jwt         *auth.JWTManager
	accessTTL   time.Duration // access token（JWT）有效期，如 15 分钟
	refreshTTL  time.Duration // refresh token 有效期，如 30 天
	now         func() time.Time
}

// NewTokenHandler 创建令牌处理器。
func NewTokenHandler(
	users repo.UserRepository,
	refreshRepo repo.RefreshTokenRepository,
	jwt *auth.JWTManager,
	accessTTL, refreshTTL time.Duration,
) *TokenHandler {
	return &TokenHandler{
		users:       users,
		refreshRepo: refreshRepo,
		jwt:         jwt,
		accessTTL:   accessTTL,
		refreshTTL:  refreshTTL,
		now:         time.Now,
	}
}

// tokenRequest 是 /token 的请求体。
type tokenRequest struct {
	GrantType string `json:"grant_type"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Scope     string `json:"scope"`
}

// refreshRequest 是 /refresh 的请求体。
type refreshRequest struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
}

// tokenResponse 是成功签发后返回的令牌对。
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

// HandleToken 处理 POST /token：密码登录换取令牌。
func (h *TokenHandler) HandleToken(w http.ResponseWriter, r *http.Request) {
	var req tokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.GrantType != "password" {
		response.Error(w, http.StatusBadRequest, "unsupported grant_type")
		return
	}
	if req.Email == "" || req.Password == "" {
		response.Error(w, http.StatusBadRequest, "email and password are required")
		return
	}

	// 按邮箱查用户
	user, err := h.users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		// 用户不存在也返回统一错误，避免泄露「邮箱是否注册」
		response.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	// 校验密码（bcrypt 加盐比对）
	if !auth.VerifyPassword(req.Password, user.PasswordHash) {
		response.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	h.issueTokens(w, r, user.ID, h.parseScopes(req.Scope))
}

// HandleRefresh 处理 POST /refresh：用 refresh token 换新的 access token。
func (h *TokenHandler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.GrantType != "refresh_token" {
		response.Error(w, http.StatusBadRequest, "unsupported grant_type")
		return
	}
	if req.RefreshToken == "" {
		response.Error(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	// 校验 refresh token 是否合法、未吊销、未过期
	token, err := auth.ValidateRefreshToken(r.Context(), h.refreshRepo, req.RefreshToken)
	if err != nil {
		logging.FromContext(r.Context()).Warn("refresh token invalid", "error", err)
		// 对外统一 invalid_grant，不泄露具体原因
		response.Error(w, http.StatusUnauthorized, "invalid_grant")
		return
	}

	// 旋转刷新：旧的作废，签发一个新的 refresh token（滑动过期 30 天）。
	// oldID 记为 rotated_from，用于审计与重放检测。
	oldID := token.ID
	now := h.now()

	// 旧 token 作废（置 revoked_at）
	token.RevokedAt = &now
	if err := h.refreshRepo.Update(r.Context(), token); err != nil {
		logging.FromContext(r.Context()).Error("revoke old refresh token failed", "error", err)
		response.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	// 签发新 refresh token（继承授权对象与权限）
	newPlain, err := auth.GenerateRefreshToken()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	newToken := &models.RefreshToken{
		UserID:      token.UserID,
		TokenHash:   auth.HashRefreshToken(newPlain),
		GrantedTo:   token.GrantedTo,
		Audience:    token.Audience,
		Scopes:      token.Scopes,
		ExpiresAt:   now.Add(h.refreshTTL),
		RotatedFrom: oldID,
	}
	if err := h.refreshRepo.Create(r.Context(), newToken); err != nil {
		logging.FromContext(r.Context()).Error("create new refresh token failed", "error", err)
		response.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	// 签发新的短期 access token
	access, _ := h.jwt.Issue(r.Context(), token.UserID, token.Scopes, h.accessTTL)

	response.JSON(w, http.StatusOK, tokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(h.accessTTL.Seconds()),
		RefreshToken: newPlain,
	})
}

// issueTokens 签发 access + refresh 令牌对（/token 成功时调用）。
func (h *TokenHandler) issueTokens(w http.ResponseWriter, r *http.Request, userID string, scopes []string) {
	now := h.now()

	// 签发新的 refresh token（granted_to=self，本轮仅支持用户本人）
	newPlain, err := auth.GenerateRefreshToken()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	newToken := &models.RefreshToken{
		UserID:    userID,
		TokenHash: auth.HashRefreshToken(newPlain),
		GrantedTo: models.GrantedToSelf,
		Scopes:    scopes,
		ExpiresAt: now.Add(h.refreshTTL),
	}
	if err := h.refreshRepo.Create(r.Context(), newToken); err != nil {
		logging.FromContext(r.Context()).Error("create refresh token failed", "error", err)
		response.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	// 签发短期 access token
	access, err := h.jwt.Issue(r.Context(), userID, scopes, h.accessTTL)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	response.JSON(w, http.StatusOK, tokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(h.accessTTL.Seconds()),
		RefreshToken: newPlain,
	})
}

// parseScopes 解析请求中的 scope 字符串（空格分隔，如 "read write"）。
// 空字符串 → 返回空 slice（无权限，仅身份）。
func (h *TokenHandler) parseScopes(scope string) []string {
	if scope == "" {
		return nil
	}
	return strings.Fields(scope)
}
