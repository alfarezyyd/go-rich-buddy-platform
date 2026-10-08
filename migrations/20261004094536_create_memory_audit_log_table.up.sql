CREATE TABLE IF NOT EXISTS memory_audit_log (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL,
    action      VARCHAR(20) NOT NULL CHECK (action IN ('add', 'update', 'delete', 'export', 'purge')),
    item_id     BIGINT,
    actor       VARCHAR(20) NOT NULL CHECK (actor IN ('system', 'user')),
    before_json JSONB,
    after_json  JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_memory_audit_log_user_id ON memory_audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_memory_audit_log_created_at ON memory_audit_log(created_at);
