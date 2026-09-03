CREATE SCHEMA IF NOT EXISTS orderservice;

CREATE TABLE orderservice.orders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL CHECK (user_id > 0),
    status TEXT NOT NULL CHECK (status IN ('pending', 'confirmed', 'failed')),
    total_amount BIGINT NOT NULL CHECK (total_amount >= 0),
    currency TEXT NOT NULL CHECK (char_length(currency) = 3),
    idempotency_key TEXT NOT NULL CHECK (
        char_length(idempotency_key) BETWEEN 1 AND 128
    ),
    cart_revision BIGINT NOT NULL CHECK (cart_revision > 0),
    failure_reason TEXT NOT NULL DEFAULT '',
    processing_by UUID,
    processing_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, idempotency_key)
);

CREATE TABLE orderservice.order_items (
    order_id BIGINT NOT NULL REFERENCES orderservice.orders(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL CHECK (product_id > 0),
    product_name TEXT NOT NULL,
    unit_price BIGINT NOT NULL CHECK (unit_price >= 0),
    quantity BIGINT NOT NULL CHECK (quantity > 0),
    line_total BIGINT NOT NULL CHECK (line_total >= 0),
    PRIMARY KEY (order_id, product_id)
);

CREATE INDEX idx_orders_user_created
    ON orderservice.orders(user_id, created_at DESC, id DESC);

CREATE INDEX idx_orders_pending_recovery
    ON orderservice.orders(updated_at, processing_at)
    WHERE status = 'pending';
