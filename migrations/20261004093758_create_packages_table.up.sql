-- Packages (Subscription / Credit price list)
CREATE TABLE IF NOT EXISTS packages (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    tier VARCHAR(50) NOT NULL,
    credits INT NOT NULL DEFAULT 0,
    price BIGINT NOT NULL DEFAULT 0,
    description TEXT,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_packages_active ON packages(active);

-- Seed initial packages
INSERT INTO packages (code, name, tier, credits, price, description, active)
VALUES
    ('BUDDY_PLUS', 'Buddy+', 'Buddy+', 300, 5900, '300 kredit per bulan', TRUE),
    ('PRO_ANALYST', 'Pro/Analyst', 'Pro/Analyst', 9999, 14900, 'Unlimited fair-use kredit', TRUE),
    ('TOPUP_50', 'Top-up 50 Kredit', 'Top-up', 50, 1500, 'Top-up instan 50 kredit', TRUE)
ON CONFLICT (code) DO NOTHING;
