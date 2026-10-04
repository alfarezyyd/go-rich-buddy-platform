CREATE TABLE IF NOT EXISTS case_file_daily (
    id BIGSERIAL PRIMARY KEY,
    data_date DATE NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    json JSONB NOT NULL,
    evidence_hash VARCHAR(64),
    explanation JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_case_file_daily_date_symbol UNIQUE (data_date, symbol)
);

CREATE INDEX IF NOT EXISTS idx_case_file_daily_date_symbol ON case_file_daily(data_date, symbol);
