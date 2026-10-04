CREATE TABLE IF NOT EXISTS chat_session (
    id                      BIGSERIAL PRIMARY KEY,
    user_id                 BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    started_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status                  VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'closed')),
    summary_text            TEXT,
    summary_upto_message_id BIGINT,
    summary_tokens          INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_chat_session_user_id ON chat_session(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_session_status ON chat_session(status);
