CREATE TABLE IF NOT EXISTS user_learning_progress (
    user_id             BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    term                VARCHAR(200) NOT NULL,
    explained_count     INT NOT NULL DEFAULT 0,
    last_explained_at   TIMESTAMPTZ,
    understood          BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (user_id, term)
);
