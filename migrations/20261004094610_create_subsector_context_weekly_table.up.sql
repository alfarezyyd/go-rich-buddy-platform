CREATE TABLE IF NOT EXISTS subsector_context_weekly (
    id BIGSERIAL PRIMARY KEY,
    iso_week VARCHAR(10) NOT NULL,
    sub_sector VARCHAR(100) NOT NULL,
    median_pe DOUBLE PRECISION NOT NULL DEFAULT 0,
    median_pb DOUBLE PRECISION NOT NULL DEFAULT 0,
    growth JSONB,
    valuation JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_subsector_context_weekly_week_subsector UNIQUE (iso_week, sub_sector)
);

CREATE INDEX IF NOT EXISTS idx_subsector_context_weekly_week ON subsector_context_weekly(iso_week);
