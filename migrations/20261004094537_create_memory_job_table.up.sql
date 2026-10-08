CREATE TABLE IF NOT EXISTS memory_job (
    message_id  BIGINT PRIMARY KEY,
    kind        VARCHAR(20) NOT NULL CHECK (kind IN ('summarize', 'extract')),
    status      VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    attempts    INT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
