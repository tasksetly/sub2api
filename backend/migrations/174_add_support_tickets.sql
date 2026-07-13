CREATE TABLE IF NOT EXISTS support_tickets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    subject VARCHAR(200) NOT NULL,
    category VARCHAR(32) NOT NULL DEFAULT 'other',
    priority VARCHAR(16) NOT NULL DEFAULT 'normal',
    status VARCHAR(24) NOT NULL DEFAULT 'open',
    admin_unread BOOLEAN NOT NULL DEFAULT TRUE,
    user_unread BOOLEAN NOT NULL DEFAULT FALSE,
    last_message_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT support_tickets_category_check CHECK (category IN ('technical', 'billing', 'account', 'other')),
    CONSTRAINT support_tickets_priority_check CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    CONSTRAINT support_tickets_status_check CHECK (status IN ('open', 'in_progress', 'waiting_user', 'resolved', 'closed'))
);

CREATE TABLE IF NOT EXISTS support_ticket_messages (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    sender_id BIGINT NOT NULL REFERENCES users(id),
    sender_role VARCHAR(16) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT support_ticket_messages_sender_role_check CHECK (sender_role IN ('user', 'admin'))
);

CREATE INDEX IF NOT EXISTS idx_support_tickets_user_last_message
    ON support_tickets (user_id, last_message_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_tickets_status_last_message
    ON support_tickets (status, last_message_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_tickets_category ON support_tickets (category);
CREATE INDEX IF NOT EXISTS idx_support_tickets_priority ON support_tickets (priority);
CREATE INDEX IF NOT EXISTS idx_support_tickets_admin_unread ON support_tickets (admin_unread) WHERE admin_unread = TRUE;
CREATE INDEX IF NOT EXISTS idx_support_tickets_user_unread ON support_tickets (user_unread) WHERE user_unread = TRUE;
CREATE INDEX IF NOT EXISTS idx_support_ticket_messages_ticket_created
    ON support_ticket_messages (ticket_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_support_ticket_messages_sender ON support_ticket_messages (sender_id);
