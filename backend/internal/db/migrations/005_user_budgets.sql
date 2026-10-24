CREATE TABLE IF NOT EXISTS user_budgets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    currency VARCHAR(8) NOT NULL,
    monthly_limit TEXT NOT NULL DEFAULT '0',
    yearly_limit TEXT NOT NULL DEFAULT '0',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    UNIQUE(user_id, currency)
);

CREATE INDEX IF NOT EXISTS ix_user_budgets_user_id ON user_budgets (user_id);
