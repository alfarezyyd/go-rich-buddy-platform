
-- Orders
CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    order_id VARCHAR(100) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL,
    package_id BIGINT NOT NULL,
    gross_amount BIGINT NOT NULL,
    payment_type VARCHAR(50) NOT NULL DEFAULT 'qris',
    transaction_status VARCHAR(50) NOT NULL DEFAULT 'pending',
    qr_string TEXT,
    qr_url TEXT,
    midtrans_response JSONB,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_orders_package FOREIGN KEY (package_id) REFERENCES packages(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_orders_order_id ON orders(order_id);
CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(transaction_status);
