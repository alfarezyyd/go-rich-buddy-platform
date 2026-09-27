CREATE TABLE whatsapp_sessions (
    id BIGSERIAL PRIMARY KEY,
    phone VARCHAR(255) UNIQUE NOT NULL,
    current_state VARCHAR(100) NOT NULL DEFAULT 'MAIN_MENU',
    retry_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ,
    created_by VARCHAR(255),
    updated_at TIMESTAMPTZ,
    updated_by VARCHAR(255),
    deleted_at TIMESTAMPTZ,
    deleted_by VARCHAR(255)
);
-- Buat index untuk pencarian phone
CREATE INDEX idx_whatsapp_sessions_phone ON whatsapp_sessions(phone);
