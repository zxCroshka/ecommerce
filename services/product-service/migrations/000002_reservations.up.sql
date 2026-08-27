CREATE TABLE productservice.stock_reservations (
    reservation_id TEXT NOT NULL,
    product_id BIGINT NOT NULL,
    quantity BIGINT NOT NULL CHECK (quantity > 0),
    status TEXT NOT NULL CHECK (status IN ('reserved', 'released')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    released_at TIMESTAMPTZ,
    PRIMARY KEY (reservation_id, product_id),
    FOREIGN KEY (product_id)
        REFERENCES productservice.products(id)
);