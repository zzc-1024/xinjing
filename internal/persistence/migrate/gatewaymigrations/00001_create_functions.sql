-- +goose Up
-- 云函数表：函数定义（运行时、入口、环境变量等）。
-- env_vars 存 JSON 对象字符串（如 {"KEY":"val"}）。
CREATE TABLE functions (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    name        TEXT NOT NULL,
    runtime     TEXT NOT NULL DEFAULT 'wasm',
    handler     TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    env_vars    TEXT NOT NULL DEFAULT '{}',
    timeout_sec INTEGER NOT NULL DEFAULT 30,
    memory_mb   INTEGER NOT NULL DEFAULT 128,
    status      TEXT NOT NULL DEFAULT 'draft',
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMP NULL
);

CREATE INDEX idx_functions_user_id ON functions (user_id);
CREATE INDEX idx_functions_deleted_at ON functions (deleted_at);

-- 函数版本表：函数的不可变发布版本，artifact_ref 指向对象存储中的产物。
CREATE TABLE function_versions (
    id           TEXT PRIMARY KEY,
    function_id  TEXT NOT NULL,
    version      TEXT NOT NULL,
    artifact_ref TEXT NOT NULL,
    digest       TEXT NOT NULL,
    build_log    TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'building',
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   TIMESTAMP NULL
);

CREATE INDEX idx_function_versions_function_id ON function_versions (function_id);
CREATE INDEX idx_function_versions_deleted_at ON function_versions (deleted_at);

-- +goose Down
DROP TABLE function_versions;
DROP TABLE functions;
