CREATE TABLE IF NOT EXISTS radar_pick_outcome (
    id BIGSERIAL PRIMARY KEY,
    pick_id BIGINT NOT NULL REFERENCES radar_pick_log(id) ON DELETE CASCADE,
    horizon_days INT NOT NULL,
    close_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    ret DOUBLE PRECISION NOT NULL DEFAULT 0,
    ihsg_ret DOUBLE PRECISION NOT NULL DEFAULT 0,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_radar_pick_outcome_pick_horizon UNIQUE (pick_id, horizon_days)
);

CREATE INDEX IF NOT EXISTS idx_radar_pick_outcome_pick_id ON radar_pick_outcome(pick_id);
