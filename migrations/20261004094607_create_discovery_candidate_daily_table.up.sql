CREATE TABLE IF NOT EXISTS discovery_candidate_daily (
    id BIGSERIAL PRIMARY KEY,
    data_date DATE NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    lens_hits TEXT,
    stage1_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    in_pool_b BOOLEAN NOT NULL DEFAULT FALSE,
    in_finalist BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_discovery_candidate_daily_date_symbol UNIQUE (data_date, symbol)
);

CREATE INDEX IF NOT EXISTS idx_discovery_candidate_daily_date ON discovery_candidate_daily(data_date);
CREATE INDEX IF NOT EXISTS idx_discovery_candidate_daily_symbol ON discovery_candidate_daily(symbol);
