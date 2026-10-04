CREATE TABLE IF NOT EXISTS chat_message (
    id              BIGSERIAL PRIMARY KEY,
    session_id      BIGINT NOT NULL REFERENCES chat_session(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wa_message_id   VARCHAR(255) UNIQUE,
    role            VARCHAR(20) NOT NULL CHECK (role IN ('user', 'assistant', 'tool')),
    content         TEXT NOT NULL DEFAULT '',
    content_type    VARCHAR(50) NOT NULL DEFAULT 'text',
    tokens          INT NOT NULL DEFAULT 0,
    refs_json       JSONB,
    redacted        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_message_session_id ON chat_message(session_id);
CREATE INDEX IF NOT EXISTS idx_chat_message_user_id ON chat_message(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_message_created_at ON chat_message(created_at);
