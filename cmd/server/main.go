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
	"xinjing/internal/handler"
	"xinjing/internal/logging"
	"xinjing/internal/middleware"
	"xinjing/internal/persistence"
	"xinjing/internal/persistence/cache"
	"xinjing/internal/persistence/migrate"
	"xinjing/internal/persistence/migrate/gatewaymigrations"
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

	// 6. 路由：/ping 保留公开（健康检查探活）；业务路由一律要求认证 + 限流。
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", handler.HealthCheck(sqlDB))

	// 受保护路由示例：GET /me 需要「已认证 + 拥有 read 权限 + 不超限」。
	// Chain 从后往前包裹：先 Authenticate（认证）→ 再 RequireScope（授权）→ 最后 RateLimit（限流）。
	// 限流策略：per-key（按用户），每 60 秒最多 100 次，突发 10。
	mux.Handle("GET /me", middleware.Chain(
		http.HandlerFunc(handler.Me),
		middleware.Authenticate(jwtAuthenticator),
		middleware.RequireScope(auth.ScopeRead),
		middleware.RateLimit(limiter, ratelimit.Policy{
			Name:       "me-default",
			LimitCount: 100,
			WindowSec:  60,
			Burst:      10,
			Scope:      ratelimit.ScopePerKey,
		}),
	))

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
