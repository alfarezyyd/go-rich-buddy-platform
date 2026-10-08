CREATE TABLE IF NOT EXISTS user_memory_item (
    id                      BIGSERIAL PRIMARY KEY,
    user_id                 BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind                    VARCHAR(20) NOT NULL CHECK (kind IN ('profile', 'preference', 'interest', 'goal')),
    key                     VARCHAR(100) NOT NULL,
    value                   TEXT NOT NULL,
    source                  VARCHAR(20) NOT NULL DEFAULT 'stated' CHECK (source IN ('stated', 'inferred')),
    status                  VARCHAR(30) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'pending_confirmation', 'deleted')),
    confidence              NUMERIC(4,3) NOT NULL DEFAULT 1.0,
    importance              NUMERIC(4,3) NOT NULL DEFAULT 0.5,
    evidence_message_ids    BIGINT[],
    first_seen_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_confirmed_at       TIMESTAMPTZ,
    last_used_at            TIMESTAMPTZ,
    expires_at              TIMESTAMPTZ,
    CONSTRAINT uq_user_memory_key_active UNIQUE NULLS NOT DISTINCT (user_id, key)
);

CREATE INDEX IF NOT EXISTS idx_user_memory_user_id ON user_memory_item(user_id);
CREATE INDEX IF NOT EXISTS idx_user_memory_status ON user_memory_item(status);
