CREATE TABLE IF NOT EXISTS commodity_trend_weekly (
    id BIGSERIAL PRIMARY KEY,
    iso_week VARCHAR(10) NOT NULL,
    commodity VARCHAR(50) NOT NULL,
    trend_3m DOUBLE PRECISION NOT NULL DEFAULT 0,
    trend_12m DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_commodity_trend_weekly_week_commodity UNIQUE (iso_week, commodity)
);
