CREATE TABLE IF NOT EXISTS support_ticket_attachments (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    message_id BIGINT NOT NULL REFERENCES support_ticket_messages(id) ON DELETE CASCADE,
    uploader_id BIGINT NOT NULL REFERENCES users(id),
    object_key VARCHAR(1024) NOT NULL UNIQUE,
    file_name VARCHAR(255) NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT support_ticket_attachments_content_type_check
        CHECK (content_type IN ('image/jpeg', 'image/png', 'image/gif', 'image/webp'))
);

CREATE INDEX IF NOT EXISTS idx_support_ticket_attachments_ticket
    ON support_ticket_attachments (ticket_id);
CREATE INDEX IF NOT EXISTS idx_support_ticket_attachments_message_created
    ON support_ticket_attachments (message_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_support_ticket_attachments_uploader
    ON support_ticket_attachments (uploader_id);
