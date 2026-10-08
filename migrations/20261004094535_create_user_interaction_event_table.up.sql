CREATE TABLE IF NOT EXISTS user_interaction_event (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        VARCHAR(50) NOT NULL CHECK (type IN ('radar_view', 'drilldown', 'glossary', 'setup_change')),
    symbol      VARCHAR(10),
    sub_sector  VARCHAR(100),
    radar_date  DATE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_interaction_event_user_id ON user_interaction_event(user_id);
CREATE INDEX IF NOT EXISTS idx_user_interaction_event_created_at ON user_interaction_event(created_at);
