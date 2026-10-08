-- Tambah kolom active_symbol untuk melacak saham yang sedang dilihat/di-drilldown per sesi.
-- Digunakan untuk memberikan konteks ke LLM Analyst saat user menanyakan 'saham ini'.
ALTER TABLE whatsapp_sessions
    ADD COLUMN IF NOT EXISTS active_symbol VARCHAR(20) NOT NULL DEFAULT '';
