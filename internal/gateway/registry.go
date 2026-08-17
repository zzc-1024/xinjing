// Package gateway 提供网关层的路由注册表与装配逻辑。
// 它把「路径 + 方法 + handler + 所需权限 + 限流策略」集中管理，
// 取代原先散落在 cmd/server/main.go 里的手写路由。
//
// 注意区分两类路由：
//   - 静态内置路由（本包管理）：平台自身提供的端点（如 GET /me），在代码里声明。
//   - 动态函数路由（阶段 5）：用户上传云函数后绑定的 path，存 routes 表，运行时装配。
//
// 本包当前只负责静态路由的集中注册；动态路由留到云函数阶段。
package gateway

import (
	"net/http"

	"xinjing/internal/auth"
	"xinjing/internal/middleware"
	"xinjing/internal/ratelimit"
)

// Route 描述一个静态路由的完整约束。
type Route struct {
	Method  string            // HTTP 方法（GET/POST/...）
	Pattern string            // 路径，如 "/me"；在 SubRouter 下会自动叠加父前缀
	Handler http.HandlerFunc  // 处理器
	Scope   auth.Scope        // 所需权限（空值表示无需额外 scope 校验，仅认证）
	Policy  *ratelimit.Policy // 限流策略；nil 表示不限流
}

// Registry 收集静态路由，并统一装配认证/授权/限流中间件。
type Registry struct {
	authenticator auth.Authenticator
	limiter       ratelimit.RateLimiter
	routes        []Route
}

// NewRegistry 创建路由注册表。
// authenticator 是 JWT 认证器；limiter 是限流器（均为共享实例）。
func NewRegistry(authenticator auth.Authenticator, limiter ratelimit.RateLimiter) *Registry {
	return &Registry{
		authenticator: authenticator,
		limiter:       limiter,
	}
}

// Add 登记一条静态路由。
func (r *Registry) Add(route Route) {
	r.routes = append(r.routes, route)
}

// Sub 创建一个子路由器：它的所有路由共享一个路径前缀与一组默认约束。
// 典型用法：/api/functions 下的多个端点都要求「认证 + functions 权限 + 同一限流策略」，
// 只需在 Sub 上声明一次，子路由无需逐个重复。
//
// prefix 形如 "/api/functions"；subscope 是该组路由默认要求的 scope，
// 传空表示子路由各自决定；subpolicy 是默认限流策略，nil 表示不限流。
func (r *Registry) Sub(prefix string, subscope auth.Scope, subpolicy *ratelimit.Policy) *SubRouter {
	return &SubRouter{
		registry:      r,
		prefix:        prefix,
		defaultScope:  subscope,
		defaultPolicy: subpolicy,
	}
}

// SubRouter 是一组共享前缀与默认约束的子路由。
type SubRouter struct {
	registry      *Registry
	prefix        string
	defaultScope  auth.Scope
	defaultPolicy *ratelimit.Policy
}

// Add 在子路由下登记一条路由：
//   - 路径自动叠加前缀（如子路由 prefix="/api"，Add Pattern="/list" → 实际 "/api/list"）
//   - 若子路由声明了默认 scope，且本条路由未显式设置 scope，则继承默认 scope
//   - 若子路由声明了默认 policy，且本条路由未显式设置 policy，则继承默认 policy
func (s *SubRouter) Add(route Route) {
	route.Pattern = s.prefix + route.Pattern

	if route.Scope == "" {
		route.Scope = s.defaultScope
	}
	if route.Policy == nil {
		route.Policy = s.defaultPolicy
	}
	s.registry.routes = append(s.registry.routes, route)
}

// Include 把另一个 Registry 中「已收集的所有路由」整体并入当前 Registry，
// 并给它们统一叠加 prefix 前缀。相当于「路由组之间的嵌套/组合」：
// 可以把一个独立构建、独立测试的 Registry（如 /functions 相关的一组路由），
// 作为子模块挂到更大的 Registry 的某个前缀下。
//
// 例：functions 是一个已 Add 了 /list、/create 的 Registry，那么
//
//	main.Include("/api", functions)
//
// 会得到 /api/list、/api/create 两条路由并入 main。
//
// 说明：
//   - 只复制路由条目（叠加前缀），不在此处包裹中间件——中间件统一在最终 Mount 时包裹，
//     因此嵌套不会产生重复认证/限流。
//   - 被 Include 的 Registry 自身的 authenticator/limiter 不参与，仍以「宿主 Registry」的为准，
//     保证整棵路由树最终用同一套认证器与限流器装配。
func (r *Registry) Include(prefix string, other *Registry) {
	if other == nil {
		return
	}
	for _, route := range other.routes {
		route.Pattern = prefix + route.Pattern
		r.routes = append(r.routes, route)
	}
}

// Mount 把所有已登记路由装配到 mux（支持 http.Handler 接口）。
// 每个路由依次包裹：限流（若有）→ 授权（若有）→ 认证（必有）。
// Chain 从后往前包裹，故书写顺序为 Handler 最内、Authenticate 最外。
func (r *Registry) Mount(mux *http.ServeMux) {
	for _, route := range r.routes {
		h := http.Handler(route.Handler)

		// 从内到外：限流最内（先消耗令牌），再授权，再认证
		if route.Policy != nil {
			h = middleware.RateLimit(r.limiter, *route.Policy)(h)
		}
		if route.Scope != "" {
			h = middleware.RequireScope(route.Scope)(h)
		}
		h = middleware.Authenticate(r.authenticator)(h)

		mux.Handle(route.Method+" "+route.Pattern, h)
	}
}
