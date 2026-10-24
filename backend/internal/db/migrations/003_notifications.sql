CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type VARCHAR(16) NOT NULL DEFAULT 'reminder',
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    occurrence_id INTEGER REFERENCES payment_occurrences(id),
    channel VARCHAR(16) NOT NULL,
    is_read BOOLEAN NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS ix_notifications_created_at ON notifications (created_at);
CREATE INDEX IF NOT EXISTS ix_notifications_is_read ON notifications (is_read);
CREATE INDEX IF NOT EXISTS ix_notifications_occurrence_id ON notifications (occurrence_id);
