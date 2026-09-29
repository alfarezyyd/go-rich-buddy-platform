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

CREATE TABLE IF NOT EXISTS ticker_explanation_daily (
    id BIGSERIAL PRIMARY KEY,
    date DATE NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    summary_reason TEXT,
    evidence_json JSONB,
    related_news JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_ticker_explanation_date_symbol UNIQUE (date, symbol)
);

CREATE INDEX IF NOT EXISTS idx_ticker_explanation_date_symbol ON ticker_explanation_daily(date, symbol);

CREATE TABLE IF NOT EXISTS user_watchlist (
    user_id BIGINT NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    added_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, symbol)
);

CREATE INDEX IF NOT EXISTS idx_user_watchlist_user ON user_watchlist(user_id);

CREATE TABLE IF NOT EXISTS user_radar_preference (
    user_id BIGINT PRIMARY KEY,
    mode VARCHAR(50) NOT NULL DEFAULT 'subsector',
    preferred_sub_sector VARCHAR(100),
    broadcast_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    broadcast_time VARCHAR(5) NOT NULL DEFAULT '08:00',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS radar_request_log (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    request_type VARCHAR(50) NOT NULL,
    params JSONB,
    response_ticker_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_radar_request_log_user ON radar_request_log(user_id);
