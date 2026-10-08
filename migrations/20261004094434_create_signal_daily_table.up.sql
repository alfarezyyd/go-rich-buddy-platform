CREATE TABLE IF NOT EXISTS signal_daily (
    id BIGSERIAL PRIMARY KEY,
    date DATE NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    sub_sector VARCHAR(100) NOT NULL,
    foreign_flow_score INT NOT NULL DEFAULT 0,
    institutional_broker_score INT NOT NULL DEFAULT 0,
    volume_score INT NOT NULL DEFAULT 0,
    momentum_score INT NOT NULL DEFAULT 0,
    bonus_corporate_action INT NOT NULL DEFAULT 0,
    bonus_quarterly_report INT NOT NULL DEFAULT 0,
    bonus_insider_buy INT NOT NULL DEFAULT 0,
    composite_score INT NOT NULL DEFAULT 0,
    is_shortlisted BOOLEAN NOT NULL DEFAULT FALSE,
    is_enriched BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_signal_daily_date_symbol UNIQUE (date, symbol)
);

CREATE INDEX IF NOT EXISTS idx_signal_daily_date ON signal_daily(date);
CREATE INDEX IF NOT EXISTS idx_signal_daily_symbol ON signal_daily(symbol);
CREATE INDEX IF NOT EXISTS idx_signal_daily_sub_sector ON signal_daily(date, sub_sector, composite_score DESC);
