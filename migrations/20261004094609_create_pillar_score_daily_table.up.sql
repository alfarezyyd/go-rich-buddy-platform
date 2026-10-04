CREATE TABLE IF NOT EXISTS pillar_score_daily (
    id BIGSERIAL PRIMARY KEY,
    data_date DATE NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    sub_sector VARCHAR(100),
    v DOUBLE PRECISION NOT NULL DEFAULT 0,
    q DOUBLE PRECISION NOT NULL DEFAULT 0,
    i DOUBLE PRECISION NOT NULL DEFAULT 0,
    h DOUBLE PRECISION NOT NULL DEFAULT 0,
    s DOUBLE PRECISION NOT NULL DEFAULT 0,
    t DOUBLE PRECISION NOT NULL DEFAULT 0,
    hgs_raw DOUBLE PRECISION NOT NULL DEFAULT 0,
    penalty_total DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    hgs DOUBLE PRECISION NOT NULL DEFAULT 0,
    data_completeness DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    archetype VARCHAR(50),
    secondary_archetype VARCHAR(50),
    confidence_label VARCHAR(20),
    is_finalist BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_pillar_score_daily_date_symbol UNIQUE (data_date, symbol)
);

CREATE INDEX IF NOT EXISTS idx_pillar_score_daily_date ON pillar_score_daily(data_date);
CREATE INDEX IF NOT EXISTS idx_pillar_score_daily_symbol ON pillar_score_daily(symbol);
CREATE INDEX IF NOT EXISTS idx_pillar_score_daily_subsector ON pillar_score_daily(data_date, sub_sector, hgs DESC);
