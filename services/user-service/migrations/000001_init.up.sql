CREATE SCHEMA IF NOT EXISTS userservice;

CREATE TABLE IF NOT EXISTS userservice.users (
	id SERIAL PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	password_hash BYTEA NOT NULL,
	name TEXT,
	role TEXT NOT NULL DEFAULT 'customer' CHECK (role IN ('customer', 'admin')),
	created_at TIMESTAMP DEFAULT NOW()

);
