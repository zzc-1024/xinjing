-- +goose Up
-- 插件表：可热插拔插件定义，capabilities/config_schema 存 JSON。
CREATE TABLE plugins (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    version       TEXT NOT NULL,
    digest        TEXT NOT NULL,
    capabilities  TEXT NOT NULL DEFAULT '[]',
    config_schema TEXT NOT NULL DEFAULT '{}',
    status        TEXT NOT NULL DEFAULT 'active',
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    TIMESTAMP NULL
);

CREATE INDEX idx_plugins_deleted_at ON plugins (deleted_at);

-- 插件实例表：插件的启用实例 + 运行时配置。
CREATE TABLE plugin_instances (
    id         TEXT PRIMARY KEY,
    plugin_id  TEXT NOT NULL,
    config     TEXT NOT NULL DEFAULT '{}',
    enabled    BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    status     TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

CREATE INDEX idx_plugin_instances_plugin_id ON plugin_instances (plugin_id);
CREATE INDEX idx_plugin_instances_deleted_at ON plugin_instances (deleted_at);

-- +goose Down
DROP TABLE plugin_instances;
DROP TABLE plugins;
