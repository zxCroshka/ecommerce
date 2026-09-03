CREATE TABLE userservice.outbox_events (
    event_id UUID PRIMARY KEY,
    topic TEXT NOT NULL,
    event_type TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version > 0),
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error TEXT,
    locked_by UUID,
    locked_at TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_userservice_outbox_pending
    ON userservice.outbox_events(next_attempt_at, created_at)
    WHERE published_at IS NULL;
