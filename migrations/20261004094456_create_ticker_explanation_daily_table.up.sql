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
