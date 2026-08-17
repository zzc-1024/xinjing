-- +goose Up
-- 用户表增加密码哈希列（bcrypt 加盐哈希，绝不明文存储）。
-- 默认空字符串表示「尚未设置密码」的用户。
ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN password_hash;
