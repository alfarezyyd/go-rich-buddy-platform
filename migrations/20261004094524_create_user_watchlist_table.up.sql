CREATE TABLE IF NOT EXISTS user_watchlist (
    user_id BIGINT NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    added_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, symbol)
);

CREATE INDEX IF NOT EXISTS idx_user_watchlist_user ON user_watchlist(user_id);
