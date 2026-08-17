-- +goose Up
-- 调用日志表：记录每次函数调用的结果，只追加、不可变、不软删除。
-- duration_ms 用 BIGINT 存毫秒，避免浮点精度问题。
CREATE TABLE invocation_logs (
    id          TEXT PRIMARY KEY,
    function_id TEXT NOT NULL,
    route_id    TEXT NULL,
    trace_id    TEXT NOT NULL DEFAULT '',
    status_code INTEGER NOT NULL DEFAULT 0,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    error       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_invocation_logs_function_id ON invocation_logs (function_id);
CREATE INDEX idx_invocation_logs_trace_id ON invocation_logs (trace_id);
CREATE INDEX idx_invocation_logs_created_at ON invocation_logs (created_at);

-- +goose Down
DROP TABLE invocation_logs;
