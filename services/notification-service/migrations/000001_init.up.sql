CREATE SCHEMA IF NOT EXISTS notificationservice;

CREATE TABLE notificationservice.notifications (
    id BIGSERIAL PRIMARY KEY,
    event_id UUID NOT NULL UNIQUE,
    user_id BIGINT NOT NULL CHECK (user_id > 0),
    type TEXT NOT NULL CHECK (char_length(type) BETWEEN 1 AND 64),
    title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 255),
    body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    read_at TIMESTAMPTZ
);

CREATE INDEX idx_notifications_user_created
    ON notificationservice.notifications(user_id, created_at DESC, id DESC);
