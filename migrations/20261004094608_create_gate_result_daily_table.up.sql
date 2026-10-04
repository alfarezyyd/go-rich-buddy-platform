CREATE TABLE IF NOT EXISTS gate_result_daily (
    id BIGSERIAL PRIMARY KEY,
    data_date DATE NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    gate_code VARCHAR(20) NOT NULL,
    result VARCHAR(10) NOT NULL,
    multiplier DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    detail JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_gate_result_daily_date ON gate_result_daily(data_date);
CREATE INDEX IF NOT EXISTS idx_gate_result_daily_symbol ON gate_result_daily(symbol);
