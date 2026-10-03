-- Rename Bahasa columns to English in signal_daily (no-op if already renamed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'signal_daily' AND column_name = 'skor_asing') THEN
        ALTER TABLE signal_daily RENAME COLUMN skor_asing TO foreign_flow_score;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'signal_daily' AND column_name = 'skor_broker_institusi') THEN
        ALTER TABLE signal_daily RENAME COLUMN skor_broker_institusi TO institutional_broker_score;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'signal_daily' AND column_name = 'skor_volume') THEN
        ALTER TABLE signal_daily RENAME COLUMN skor_volume TO volume_score;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'signal_daily' AND column_name = 'skor_momentum') THEN
        ALTER TABLE signal_daily RENAME COLUMN skor_momentum TO momentum_score;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'signal_daily' AND column_name = 'bonus_laporan_kuartal') THEN
        ALTER TABLE signal_daily RENAME COLUMN bonus_laporan_kuartal TO bonus_quarterly_report;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'signal_daily' AND column_name = 'skor_komposit') THEN
        ALTER TABLE signal_daily RENAME COLUMN skor_komposit TO composite_score;
    END IF;
END $$;

-- Recreate composite_score index with new column name (idempotent)
DROP INDEX IF EXISTS idx_signal_daily_sub_sector;
CREATE INDEX IF NOT EXISTS idx_signal_daily_sub_sector ON signal_daily(date, sub_sector, composite_score DESC);

-- Rename Bahasa columns to English in ticker_explanation_daily (no-op if already renamed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'ticker_explanation_daily' AND column_name = 'ringkasan_alasan') THEN
        ALTER TABLE ticker_explanation_daily RENAME COLUMN ringkasan_alasan TO summary_reason;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'ticker_explanation_daily' AND column_name = 'bukti_json') THEN
        ALTER TABLE ticker_explanation_daily RENAME COLUMN bukti_json TO evidence_json;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'ticker_explanation_daily' AND column_name = 'berita_terkait') THEN
        ALTER TABLE ticker_explanation_daily RENAME COLUMN berita_terkait TO related_news;
    END IF;
END $$;

-- Rename Bahasa column to English in user_radar_preference (no-op if already renamed)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'user_radar_preference' AND column_name = 'sub_sector_pilihan') THEN
        ALTER TABLE user_radar_preference RENAME COLUMN sub_sector_pilihan TO preferred_sub_sector;
    END IF;
END $$;
