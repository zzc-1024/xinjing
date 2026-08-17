-- +goose Up
-- refresh token 表：长期「换发短期 JWT」的凭证（替代已废弃的 API Key）。
-- 只存 token 哈希（token_hash），绝不明文存储；granted_to 记录授权给谁（self / third_party）。
CREATE TABLE refresh_tokens (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    granted_to   TEXT NOT NULL DEFAULT 'self',
    audience     TEXT NOT NULL DEFAULT '',
    scopes       TEXT NOT NULL DEFAULT '[]',
    expires_at   TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP NULL,
    revoked_at   TIMESTAMP NULL,
    rotated_from TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   TIMESTAMP NULL
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);
CREATE INDEX idx_refresh_tokens_deleted_at ON refresh_tokens (deleted_at);

-- +goose Down
DROP TABLE refresh_tokens;
