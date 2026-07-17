-- User support tickets and their conversation messages.
CREATE TABLE IF NOT EXISTS tickets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject VARCHAR(200) NOT NULL,
    category VARCHAR(32) NOT NULL DEFAULT 'other',
    status VARCHAR(24) NOT NULL DEFAULT 'open',
    priority VARCHAR(16) NOT NULL DEFAULT 'normal',
    last_message_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ticket_messages (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    author_role VARCHAR(16) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    attachments JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tickets_user_last_message
    ON tickets (user_id, last_message_at DESC);
CREATE INDEX IF NOT EXISTS idx_tickets_status_last_message
    ON tickets (status, last_message_at DESC);
CREATE INDEX IF NOT EXISTS idx_tickets_priority_last_message
    ON tickets (priority, last_message_at DESC);
CREATE INDEX IF NOT EXISTS idx_ticket_messages_ticket_created
    ON ticket_messages (ticket_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_ticket_messages_author
    ON ticket_messages (author_id);
