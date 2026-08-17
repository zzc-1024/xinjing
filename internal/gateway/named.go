package gateway

import (
	"fmt"
	"strings"

	"xinjing/internal/middleware"
)

// Provider 是提供者标识（官方为 "xinjing"，第三方用自己的组织名）。
// 它与 Name 共同构成中间件/插件的唯一身份，用于命名空间隔离与冲突判定。
type Provider string

// 官方内置提供者标识。
const ProviderXinjing Provider = "xinjing"

// validProviderChar 判断字符是否合法：英文大小写字母、数字、减号、下划线。
// 不含点号（点保留给未来可能的层级扩展），也不含其他符号。
func validProviderChar(c rune) bool {
	switch {
	case c >= 'a' && c <= 'z':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '-' || c == '_':
		return true
	default:
		return false
	}
}

// Valid 校验 Provider 是否合法（非空，且每个字符都满足 validProviderChar）。
func (p Provider) Valid() bool {
	if p == "" {
		return false
	}
	for _, c := range p {
		if !validProviderChar(c) {
			return false
		}
	}
	return true
}

// ConflictPolicy 定义「同 Provider+Name 重复出现」时的处理方式。
type ConflictPolicy int

const (
	// ConflictError 默认：重复即报错（显式暴露冲突）。
	ConflictError ConflictPolicy = iota
	// ConflictKeepFirst 优先规则：保留先加入者，忽略后加入者。
	ConflictKeepFirst
	// ConflictKeepLast 优先规则：后加入者覆盖先加入者。
	ConflictKeepLast
)

// NamedMiddleware 是一个带「身份 + 冲突策略」的中间件。
// 身份 = Provider + Name；冲突策略决定重复时如何处理。
//
// 该结构与未来的插件身份体系保持一致：每个来源的插件都有 Provider + Name，
// 便于不同提供者提供同名部件，且防止第三方抢占官方命名。
type NamedMiddleware struct {
	Provider   Provider              // 提供者（如 xinjing / acme）
	Name       string                // 部件名（如 auth / ratelimit）
	Apply      middleware.Middleware // 实际的中间件函数
	OnConflict ConflictPolicy        // 重复冲突时的处理（默认 ConflictError）
}

// Key 返回唯一身份键，形如 "xinjing:auth"。
func (n NamedMiddleware) Key() string {
	return string(n.Provider) + ":" + n.Name
}

// validate 校验该中间件是否合法（Provider 合法、Name 非空）。
func (n NamedMiddleware) validate() error {
	if !n.Provider.Valid() {
		return fmt.Errorf("invalid provider %q (charset: [a-zA-Z0-9_-])", n.Provider)
	}
	if strings.TrimSpace(n.Name) == "" {
		return fmt.Errorf("middleware name must not be empty (provider %q)", n.Provider)
	}
	return nil
}
