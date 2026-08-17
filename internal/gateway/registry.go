// Package gateway 提供网关层的路由注册表与装配逻辑。
// 它把「路径 + 方法 + handler + 所需权限 + 限流策略 + 自定义中间件」集中管理，
// 取代原先散落在 cmd/server/main.go 里的手写路由。
//
// 注意区分两类路由：
//   - 静态内置路由（本包管理）：平台自身提供的端点（如 GET /me），在代码里声明。
//   - 动态函数路由（阶段 5）：用户上传云函数后绑定的 path，存 routes 表，运行时装配。
//
// 本包当前只负责静态路由的集中注册；动态路由留到云函数阶段。
package gateway

import (
	"fmt"
	"net/http"

	"xinjing/internal/auth"
	"xinjing/internal/logging"
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
	// Middlewares 是本路由额外的自定义中间件（带身份，按 Provider+Name 去重）。
	// 默认会叠加到内置的认证/授权/限流之上，顺序见 Mount。
	Middlewares []NamedMiddleware
}

// Registry 收集静态路由，并统一装配认证/授权/限流/自定义中间件。
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
// subMiddlewares 是该组路由共享的中间件集合（带身份）。
func (r *Registry) Sub(prefix string, subscope auth.Scope, subpolicy *ratelimit.Policy, subMiddlewares ...NamedMiddleware) *SubRouter {
	return &SubRouter{
		registry:      r,
		prefix:        prefix,
		defaultScope:  subscope,
		defaultPolicy: subpolicy,
		middlewares:   subMiddlewares,
	}
}

// SubRouter 是一组共享前缀、默认约束与中间件的子路由。
type SubRouter struct {
	registry      *Registry
	prefix        string
	defaultScope  auth.Scope
	defaultPolicy *ratelimit.Policy
	middlewares   []NamedMiddleware
}

// Add 在子路由下登记一条路由：
//   - 路径自动叠加前缀（如子路由 prefix="/api"，Add Pattern="/list" → 实际 "/api/list"）
//   - 若子路由声明了默认 scope，且本条路由未显式设置 scope，则继承默认 scope
//   - 若子路由声明了默认 policy，且本条路由未显式设置 policy，则继承默认 policy
//   - 子路由的中间件集合会合并到本条路由（与路由自身中间件按身份去重）
func (s *SubRouter) Add(route Route) {
	route.Pattern = s.prefix + route.Pattern

	if route.Scope == "" {
		route.Scope = s.defaultScope
	}
	if route.Policy == nil {
		route.Policy = s.defaultPolicy
	}
	// 合并中间件：子路由共享的在前，路由自身的在后（后加者靠内层）。
	route.Middlewares = append(s.middlewares, route.Middlewares...)
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
// extra 可选：为本组路由额外附加的中间件（带身份，与路由自身中间件去重）。
//
// 说明：
//   - 只复制路由条目（叠加前缀、合并中间件），不在此处包裹中间件——中间件统一在最终 Mount 时包裹，
//     因此嵌套不会产生重复认证/限流。
//   - 被 Include 的 Registry 自身的 authenticator/limiter 不参与，仍以「宿主 Registry」的为准。
func (r *Registry) Include(prefix string, other *Registry, extra ...NamedMiddleware) {
	if other == nil {
		return
	}
	for _, route := range other.routes {
		route.Pattern = prefix + route.Pattern
		route.Middlewares = append(extra, route.Middlewares...)
		r.routes = append(r.routes, route)
	}
}

// Mount 把所有已登记路由装配到 mux（支持 http.Handler 接口）。
// 每个路由依次包裹：自定义中间件 → 限流（若有）→ 授权（若有）→ 认证（必有）。
// 自定义中间件在最终按 Provider+Name 去重（并校验冲突策略），保证不重复添加。
func (r *Registry) Mount(mux *http.ServeMux) error {
	for _, route := range r.routes {
		h := http.Handler(route.Handler)

		// ① 自定义中间件（去重后），最先包裹在最内层
		deduped, err := dedupeMiddlewares(route.Middlewares)
		if err != nil {
			return fmt.Errorf("route %s %s: %w", route.Method, route.Pattern, err)
		}
		for i := len(deduped) - 1; i >= 0; i-- {
			h = deduped[i].Apply(h)
		}

		// ② 限流（若有）
		if route.Policy != nil {
			h = middleware.RateLimit(r.limiter, *route.Policy)(h)
		}
		// ③ 授权（若有）
		if route.Scope != "" {
			h = middleware.RequireScope(route.Scope)(h)
		}
		// ④ 认证（必有，最外层）
		h = middleware.Authenticate(r.authenticator)(h)

		mux.Handle(route.Method+" "+route.Pattern, h)
	}
	return nil
}

// dedupeMiddlewares 按 Provider+Name 去重并校验冲突策略。
// 遇到重复（同 Provider+Name）时按每个中间件自身的 OnConflict 策略处理：
//   - ConflictError（默认）：返回错误，显式暴露冲突
//   - ConflictKeepFirst：保留先加入者
//   - ConflictKeepLast：后加入者覆盖先加入者
func dedupeMiddlewares(ms []NamedMiddleware) ([]NamedMiddleware, error) {
	seen := make(map[string]int) // key → 在结果切片中的下标
	out := make([]NamedMiddleware, 0, len(ms))

	for _, m := range ms {
		if err := m.validate(); err != nil {
			return nil, err
		}
		key := m.Key()
		if idx, exists := seen[key]; exists {
			// 重复：按冲突策略处理
			switch m.OnConflict {
			case ConflictKeepFirst:
				// DEBUG 记录「重复被去重，保留先加入者」
				logging.For("gateway").Debug("duplicate middleware, keep first",
					"key", key, "policy", "keep_first")
				continue // 保留先加入者，丢弃当前
			case ConflictKeepLast:
				// DEBUG 记录「重复被去重，后加入者覆盖」
				logging.For("gateway").Debug("duplicate middleware, keep last",
					"key", key, "policy", "keep_last")
				out[idx] = m // 后加入者覆盖
				continue
			default: // ConflictError
				// DEBUG 先记录冲突详情，再返回错误
				logging.For("gateway").Debug("duplicate middleware, conflict error",
					"key", key, "policy", "error")
				return nil, fmt.Errorf("duplicate middleware %q", key)
			}
		}
		if m.Apply == nil {
			return nil, fmt.Errorf("middleware %q has nil Apply", key)
		}
		seen[key] = len(out)
		out = append(out, m)
	}
	return out, nil
}
