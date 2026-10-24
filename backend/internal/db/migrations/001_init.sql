CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    login VARCHAR(64) NOT NULL UNIQUE,
    email VARCHAR(255) UNIQUE,
    full_name VARCHAR(255),
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(16) NOT NULL DEFAULT 'user',
    is_active BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS ix_users_login ON users (login);

CREATE TABLE IF NOT EXISTS revoked_tokens (
    jti VARCHAR(64) PRIMARY KEY,
    expires_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS recurring_payments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title VARCHAR(255) NOT NULL,
    amount TEXT NOT NULL,
    currency VARCHAR(8) NOT NULL,
    periodicity VARCHAR(16) NOT NULL DEFAULT 'monthly',
    next_payment_date DATE NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS ix_recurring_payments_currency ON recurring_payments (currency);
CREATE INDEX IF NOT EXISTS ix_recurring_payments_next_payment_date ON recurring_payments (next_payment_date);

CREATE TABLE IF NOT EXISTS payment_occurrences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    recurring_payment_id INTEGER NOT NULL REFERENCES recurring_payments(id),
    due_date DATE NOT NULL,
    is_paid BOOLEAN NOT NULL DEFAULT 0,
    paid_at DATE,
    paid_by INTEGER REFERENCES users(id),
    reminder_sent_at TIMESTAMP,
    email_reminder_sent_at TIMESTAMP,
    webhook_reminder_sent_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    UNIQUE (recurring_payment_id, due_date)
);

CREATE INDEX IF NOT EXISTS ix_payment_occurrences_due_date ON payment_occurrences (due_date);
CREATE INDEX IF NOT EXISTS ix_payment_occurrences_is_paid ON payment_occurrences (is_paid);
CREATE INDEX IF NOT EXISTS ix_payment_occurrences_recurring_payment_id ON payment_occurrences (recurring_payment_id);
