-- Revert English columns back to original Bahasa names in signal_daily
ALTER TABLE signal_daily
    RENAME COLUMN foreign_flow_score TO skor_asing;

ALTER TABLE signal_daily
    RENAME COLUMN institutional_broker_score TO skor_broker_institusi;

ALTER TABLE signal_daily
    RENAME COLUMN volume_score TO skor_volume;

ALTER TABLE signal_daily
    RENAME COLUMN momentum_score TO skor_momentum;

ALTER TABLE signal_daily
    RENAME COLUMN bonus_quarterly_report TO bonus_laporan_kuartal;

ALTER TABLE signal_daily
    RENAME COLUMN composite_score TO skor_komposit;

DROP INDEX IF EXISTS idx_signal_daily_sub_sector;
CREATE INDEX IF NOT EXISTS idx_signal_daily_sub_sector ON signal_daily(date, sub_sector, skor_komposit DESC);

-- Revert English columns back to original Bahasa names in ticker_explanation_daily
ALTER TABLE ticker_explanation_daily
    RENAME COLUMN summary_reason TO ringkasan_alasan;

ALTER TABLE ticker_explanation_daily
    RENAME COLUMN evidence_json TO bukti_json;

ALTER TABLE ticker_explanation_daily
    RENAME COLUMN related_news TO berita_terkait;

-- Revert English columns back to original Bahasa names in user_radar_preference
ALTER TABLE user_radar_preference
    RENAME COLUMN preferred_sub_sector TO sub_sector_pilihan;
