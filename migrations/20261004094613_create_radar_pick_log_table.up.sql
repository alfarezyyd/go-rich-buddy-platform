CREATE TABLE IF NOT EXISTS radar_pick_log (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    shown_date DATE NOT NULL,
    ref_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    hgs DOUBLE PRECISION NOT NULL DEFAULT 0,
    archetype VARCHAR(50),
    mode VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_radar_pick_log_user ON radar_pick_log(user_id);
CREATE INDEX IF NOT EXISTS idx_radar_pick_log_date ON radar_pick_log(shown_date);
