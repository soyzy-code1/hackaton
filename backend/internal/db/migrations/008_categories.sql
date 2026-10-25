ALTER TABLE recurring_payments ADD COLUMN category TEXT NOT NULL DEFAULT 'other';
CREATE INDEX IF NOT EXISTS ix_recurring_payments_category ON recurring_payments (category);

CREATE TABLE IF NOT EXISTS payment_categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO payment_categories (name) VALUES
 ('housing'), ('subscriptions'), ('services'), ('infrastructure'), ('transport'), ('other');
