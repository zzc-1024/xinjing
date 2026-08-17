-- +goose Up
-- 用户表（认证服务专用）。
--   id 为主键，存 UUIDv7 字符串（多机生成不冲突，利于未来分片）
--   password_hash 存 bcrypt 加盐哈希，绝不明文存储
--   deleted_at 用于软删除（GORM 约定）
CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    TIMESTAMP NULL
);

CREATE INDEX idx_users_deleted_at ON users (deleted_at);

-- +goose Down
DROP TABLE users;
