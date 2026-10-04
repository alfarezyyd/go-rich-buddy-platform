-- Tambah kolom last_radar_symbols untuk menyimpan urutan ticker hasil radar terakhir per sesi.
-- Digunakan untuk memetakan pilihan angka (1, 2, 3...) dari user ke kode saham yang tepat
-- tanpa memerlukan state eksternal (misal Redis/cache).
ALTER TABLE whatsapp_sessions
    ADD COLUMN IF NOT EXISTS last_radar_symbols TEXT NOT NULL DEFAULT '';
