CREATE TABLE IF NOT EXISTS user_radar_preference (
    user_id BIGINT PRIMARY KEY,
    mode VARCHAR(50) NOT NULL DEFAULT 'subsector',
    preferred_sub_sector VARCHAR(100),
    broadcast_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    broadcast_time VARCHAR(5) NOT NULL DEFAULT '08:00',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
