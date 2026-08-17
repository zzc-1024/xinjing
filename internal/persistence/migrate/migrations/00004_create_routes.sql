-- +goose Up
-- 限流策略表：token bucket 参数。
-- 列名用 limit_count 而非 limit（后者是 SQL 保留字）。
CREATE TABLE rate_limit_policies (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    limit_count INTEGER NOT NULL DEFAULT 100,
    window_sec  INTEGER NOT NULL DEFAULT 60,
    burst       INTEGER NOT NULL DEFAULT 10,
    scope       TEXT NOT NULL DEFAULT 'per-key',
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMP NULL
);

CREATE INDEX idx_rate_limit_policies_deleted_at ON rate_limit_policies (deleted_at);

-- 路由表：把 API 路径 + 方法绑定到函数。
-- 不建外键约束：分布式数据库（YugabyteDB）建议由应用层保证引用完整性。
CREATE TABLE routes (
    id                   TEXT PRIMARY KEY,
    function_id          TEXT NOT NULL,
    path                 TEXT NOT NULL,
    method               TEXT NOT NULL,
    auth_required        BOOLEAN NOT NULL DEFAULT TRUE,
    rate_limit_policy_id TEXT NULL,
    status               TEXT NOT NULL DEFAULT 'active',
    created_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at           TIMESTAMP NULL
);

CREATE INDEX idx_routes_function_id ON routes (function_id);
CREATE INDEX idx_routes_path_method ON routes (path, method);
CREATE INDEX idx_routes_deleted_at ON routes (deleted_at);

-- +goose Down
DROP TABLE routes;
DROP TABLE rate_limit_policies;
