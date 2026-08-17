// Package auth 提供平台认证与授权基础能力：
// API Key 的生成与校验、JWT 的签发与校验、以及 scope（权限范围）的解析与判断。
// 本包只依赖 models 包的数据结构，通过接口访问密钥存储，不直接接触 GORM 或 HTTP 路由。
package auth

import (
	"sort"
	"strings"
)

// Scope 表示一个权限范围。底层用 string，方便在 JSON、日志、数据库中直接序列化。
// 采用「资源:动作」的层级命名（如 "functions:read"），便于后续按资源细分权限。
type Scope string

// 平台预定义的最小 scope 集合。
// 随着阶段推进（云函数、插件管理），会按需在此扩展更多细分 scope。
const (
	// ScopeRead 通用只读权限。
	ScopeRead Scope = "read"
	// ScopeWrite 通用写权限。
	ScopeWrite Scope = "write"
	// ScopeAdmin 管理员权限：通配全部 scope（见 Scopes.Has）。
	ScopeAdmin Scope = "admin"
	// ScopeFunctions 云函数管理权限。
	ScopeFunctions Scope = "functions"
	// ScopePlugins 插件管理权限。
	ScopePlugins Scope = "plugins"
)

// Scopes 表示一组去重后的权限范围，内部用 map 实现 O(1) 的判断。
// map 的值类型 struct{} 是 Go 里「零内存占位」的惯用法，表示「只看键是否存在」。
type Scopes map[Scope]struct{}

// NewScopes 从字符串切片构造 Scopes，自动去重、忽略空白字符串。
func NewScopes(raw []string) Scopes {
	s := make(Scopes, len(raw))
	for _, r := range raw {
		// TrimSpace 去掉首尾空白，避免 " read " 这类脏数据被当成独立 scope。
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		s[Scope(r)] = struct{}{}
	}
	return s
}

// Has 判断是否拥有指定 scope。规则：
//   - 若集合包含 admin，则视为拥有全部权限（admin 通配）。
//   - 否则只看是否直接包含该 scope。
func (s Scopes) Has(scope Scope) bool {
	if s == nil {
		return false
	}
	if _, ok := s[ScopeAdmin]; ok {
		return true
	}
	_, ok := s[scope]
	return ok
}

// HasAny 判断是否拥有任意一个 scope，常用于「满足其一即可放行」的场景。
func (s Scopes) HasAny(scopes ...Scope) bool {
	for _, sc := range scopes {
		if s.Has(sc) {
			return true
		}
	}
	return false
}

// Strings 返回去重后的 scope 字符串切片，先排序再输出，保证结果稳定（便于日志与测试断言）。
func (s Scopes) Strings() []string {
	out := make([]string, 0, len(s))
	for sc := range s {
		out = append(out, string(sc))
	}
	sort.Strings(out)
	return out
}
