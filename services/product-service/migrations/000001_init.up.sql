
CREATE SCHEMA IF NOT EXISTS productservice;

CREATE TABLE IF NOT EXISTS productservice.products (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    price BIGINT NOT NULL CHECK (price >= 0),
    stock BIGINT NOT NULL DEFAULT 0 CHECK (stock >= 0),
    category TEXT NOT NULL,
    images TEXT[] DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_products_category ON productservice.products(category);
CREATE INDEX IF NOT EXISTS idx_products_is_active ON productservice.products(is_active);