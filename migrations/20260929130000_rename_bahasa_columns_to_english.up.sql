-- Rename Bahasa columns to English in signal_daily
ALTER TABLE signal_daily
    RENAME COLUMN skor_asing TO foreign_flow_score;

ALTER TABLE signal_daily
    RENAME COLUMN skor_broker_institusi TO institutional_broker_score;

ALTER TABLE signal_daily
    RENAME COLUMN skor_volume TO volume_score;

ALTER TABLE signal_daily
    RENAME COLUMN skor_momentum TO momentum_score;

ALTER TABLE signal_daily
    RENAME COLUMN bonus_laporan_kuartal TO bonus_quarterly_report;

ALTER TABLE signal_daily
    RENAME COLUMN skor_komposit TO composite_score;

-- Recreate composite_score index with new column name
DROP INDEX IF EXISTS idx_signal_daily_sub_sector;
CREATE INDEX IF NOT EXISTS idx_signal_daily_sub_sector ON signal_daily(date, sub_sector, composite_score DESC);

-- Rename Bahasa columns to English in ticker_explanation_daily
ALTER TABLE ticker_explanation_daily
    RENAME COLUMN ringkasan_alasan TO summary_reason;

ALTER TABLE ticker_explanation_daily
    RENAME COLUMN bukti_json TO evidence_json;

ALTER TABLE ticker_explanation_daily
    RENAME COLUMN berita_terkait TO related_news;

-- Rename Bahasa columns to English in user_radar_preference
ALTER TABLE user_radar_preference
    RENAME COLUMN sub_sector_pilihan TO preferred_sub_sector;
