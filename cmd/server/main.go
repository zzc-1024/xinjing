// cmd/server 是心境平台的网关服务。
// 职责：连接业务数据库（functions、routes、plugins、invocation_logs），
// 只持 JWT 公钥验签，拒绝一切未认证请求；不接触 auth 数据库。
// 认证（/token /refresh /revoke）由独立的认证服务（cmd/auth）负责。
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"xinjing/internal/auth"
	"xinjing/internal/config"
	"xinjing/internal/gateway"
	"xinjing/internal/handler"
	"xinjing/internal/logging"
	"xinjing/internal/middleware"
	"xinjing/internal/persistence"
	"xinjing/internal/persistence/cache"
	"xinjing/internal/persistence/migrate"
	"xinjing/internal/persistence/migrate/gatewaymigrations"
	"xinjing/internal/persistence/repo"
	"xinjing/internal/ratelimit"
)

func main() {
	cfg := config.Load()

	logging.Init(logging.Config{
		Level:  logging.ParseLevel(cfg.LogLevel),
		Format: logging.ParseFormat(cfg.LogFormat),
	})
	log := logging.For("gateway")

	// 1. 打开业务数据库（与 auth 数据库分离）
	db, err := persistence.Open(cfg.DBDriver, cfg.DBDSN, cfg.DBMaxOpen, cfg.DBMaxIdle)
	if err != nil {
		log.Error("open database failed", "error", err)
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Error("get sql.DB failed", "error", err)
		os.Exit(1)
	}

	// 2. 运行业务迁移（functions、routes、plugins、invocation_logs）
	if cfg.DBAutoMigrate {
		if err := migrate.Run(context.Background(), sqlDB, cfg.DBDriver, gatewaymigrations.FS); err != nil {
			log.Error("migrate database failed", "error", err)
			os.Exit(1)
		}
	}

	// 3. 网关只负责验签，公钥必须配置（不持有私钥，无法签发）
	if cfg.JWTPublicKeyPath == "" {
		log.Error("JWT public key not configured", "hint", "set XINJING_JWT_PUBLIC_KEY to the public key PEM path")
		os.Exit(1)
	}
	jwtManager, err := auth.LoadJWTManager("", cfg.JWTPublicKeyPath)
	if err != nil {
		log.Error("load JWT public key failed", "error", err)
		os.Exit(1)
	}

	// 4. JWT 认证器：所有业务路由统一用它校验 Bearer JWT
	jwtAuthenticator := &auth.JWTAuthenticator{Manager: jwtManager}

	// 5. 初始化限流器（开发期 memory 缓存，单机可用；valkey 留待引入后替换）
	limiter := ratelimit.NewTokenBucket(cache.NewMemory())

	// 6. 从数据库加载限流策略（按 name 查询）；未配置时回退内置默认策略。
	rateLimitPolicyRepo := repo.NewRateLimitPolicyRepository(db)
	mePolicy := lookupRateLimitPolicy(rateLimitPolicyRepo, "me-default")

	// 7. 路由注册表：集中声明静态路由，统一装配认证/授权/限流。
	// /ping 保留公开（健康检查探活），不走注册表；业务路由一律进注册表。
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", handler.HealthCheck(sqlDB))

	registry := gateway.NewRegistry(jwtAuthenticator, limiter)
	registry.Add(gateway.Route{
		Method:  "GET",
		Pattern: "/me",
		Handler: handler.Me,
		Scope:   auth.ScopeRead,
		Policy:  mePolicy,
	})
	registry.Mount(mux)

	finalHandler := middleware.Chain(
		mux,
		middleware.Recovery(),
		middleware.Trace(),
		middleware.AccessLog(),
		middleware.CORS(),
	)

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      finalHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		log.Info("gateway server starting", "port", cfg.ServerPort, "env", cfg.AppEnv)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("gateway server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "error", err)
	}
	log.Info("gateway server stopped")
}

// lookupRateLimitPolicy 从数据库按名称加载限流策略，转换为 ratelimit.Policy。
// 库中无该策略（或 DB 未初始化策略数据）时，回退到内置默认策略，保证网关可启动。
func lookupRateLimitPolicy(repo repo.RateLimitPolicyRepository, name string) *ratelimit.Policy {
	p, err := repo.GetByName(context.Background(), name)
	if err != nil {
		// 查不到（开发初期策略表为空）或查询失败：回退默认策略
		logging.For("gateway").Debug("rate limit policy not found, use default", "name", name, "error", err)
		return defaultPolicy(name)
	}
	return &ratelimit.Policy{
		Name:       p.Name,
		LimitCount: p.LimitCount,
		WindowSec:  p.WindowSec,
		Burst:      p.Burst,
		Scope:      ratelimit.Scope(p.Scope),
	}
}

// defaultPolicy 返回内置默认限流策略（per-key，60 秒 100 次，突发 10）。
func defaultPolicy(name string) *ratelimit.Policy {
	return &ratelimit.Policy{
		Name:       name,
		LimitCount: 100,
		WindowSec:  60,
		Burst:      10,
		Scope:      ratelimit.ScopePerKey,
	}
}
