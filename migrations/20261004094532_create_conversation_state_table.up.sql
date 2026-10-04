CREATE TABLE IF NOT EXISTS conversation_state (
    user_id             BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    last_radar_date     DATE,
    last_radar_symbols  JSONB,
    last_focus_symbol   VARCHAR(10),
    pending_action      VARCHAR(100),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ
);
