-- Migration: create_memory_tables
-- Implements the memory & caching data model from PRD MEMORY v0.3

-- ============================================================
-- chat_session
-- Session is split by inactivity gap (e.g. 6 hours) or new day
-- ============================================================
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

-- ============================================================
-- chat_message
-- Raw conversation turns
-- ============================================================
CREATE TABLE IF NOT EXISTS chat_message (
    id              BIGSERIAL PRIMARY KEY,
    session_id      BIGINT NOT NULL REFERENCES chat_session(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wa_message_id   VARCHAR(255) UNIQUE,                 -- dedup webhook retries
    role            VARCHAR(20) NOT NULL CHECK (role IN ('user', 'assistant', 'tool')),
    content         TEXT NOT NULL DEFAULT '',
    content_type    VARCHAR(50) NOT NULL DEFAULT 'text',
    tokens          INT NOT NULL DEFAULT 0,
    refs_json       JSONB,                               -- {symbols:[], radar_date, sub_sector}
    redacted        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_message_session_id ON chat_message(session_id);
CREATE INDEX IF NOT EXISTS idx_chat_message_user_id ON chat_message(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_message_created_at ON chat_message(created_at);

-- ============================================================
-- conversation_state
-- Working state: one row per user
-- ============================================================
CREATE TABLE IF NOT EXISTS conversation_state (
    user_id             BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    last_radar_date     DATE,
    last_radar_symbols  JSONB,     -- ordered list of symbols for resolving "yang kedua tadi"
    last_focus_symbol   VARCHAR(10),
    pending_action      VARCHAR(100),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ
);

-- ============================================================
-- user_memory_item
-- Long-term memory: preferences, experience level, interests, goals
-- ============================================================
CREATE TABLE IF NOT EXISTS user_memory_item (
    id                      BIGSERIAL PRIMARY KEY,
    user_id                 BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind                    VARCHAR(20) NOT NULL CHECK (kind IN ('profile', 'preference', 'interest', 'goal')),
    key                     VARCHAR(100) NOT NULL,
    value                   TEXT NOT NULL,
    source                  VARCHAR(20) NOT NULL DEFAULT 'stated' CHECK (source IN ('stated', 'inferred')),
    status                  VARCHAR(30) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'pending_confirmation', 'deleted')),
    confidence              NUMERIC(4,3) NOT NULL DEFAULT 1.0,
    importance              NUMERIC(4,3) NOT NULL DEFAULT 0.5,
    evidence_message_ids    BIGINT[],
    first_seen_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_confirmed_at       TIMESTAMPTZ,
    last_used_at            TIMESTAMPTZ,
    expires_at              TIMESTAMPTZ,
    CONSTRAINT uq_user_memory_key_active
        UNIQUE NULLS NOT DISTINCT (user_id, key)
        -- enforced at application level for active/pending_confirmation
);

CREATE INDEX IF NOT EXISTS idx_user_memory_user_id ON user_memory_item(user_id);
CREATE INDEX IF NOT EXISTS idx_user_memory_status ON user_memory_item(status);

-- ============================================================
-- user_learning_progress
-- Tracks which glossary terms have been explained
-- ============================================================
CREATE TABLE IF NOT EXISTS user_learning_progress (
    user_id             BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    term                VARCHAR(200) NOT NULL,
    explained_count     INT NOT NULL DEFAULT 0,
    last_explained_at   TIMESTAMPTZ,
    understood          BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (user_id, term)
);

-- ============================================================
-- user_interaction_event
-- Tracks radar views, drill-downs, glossary opens, etc. (90d retention)
-- ============================================================
CREATE TABLE IF NOT EXISTS user_interaction_event (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        VARCHAR(50) NOT NULL CHECK (type IN ('radar_view', 'drilldown', 'glossary', 'setup_change')),
    symbol      VARCHAR(10),
    sub_sector  VARCHAR(100),
    radar_date  DATE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_interaction_event_user_id ON user_interaction_event(user_id);
CREATE INDEX IF NOT EXISTS idx_user_interaction_event_created_at ON user_interaction_event(created_at);

-- ============================================================
-- memory_audit_log
-- Immutable audit trail for memory operations (no sensitive content)
-- ============================================================
CREATE TABLE IF NOT EXISTS memory_audit_log (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL,
    action      VARCHAR(20) NOT NULL CHECK (action IN ('add', 'update', 'delete', 'export', 'purge')),
    item_id     BIGINT,
    actor       VARCHAR(20) NOT NULL CHECK (actor IN ('system', 'user')),
    before_json JSONB,
    after_json  JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_memory_audit_log_user_id ON memory_audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_memory_audit_log_created_at ON memory_audit_log(created_at);

-- ============================================================
-- memory_job
-- Idempotency marker for async memory/summarize jobs
-- ============================================================
CREATE TABLE IF NOT EXISTS memory_job (
    message_id  BIGINT PRIMARY KEY,
    kind        VARCHAR(20) NOT NULL CHECK (kind IN ('summarize', 'extract')),
    status      VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    attempts    INT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
