CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(64) NOT NULL,
    entity VARCHAR(64) NOT NULL,
    entity_id INTEGER,
    details TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS ix_audit_log_created_at ON audit_log (created_at);
CREATE INDEX IF NOT EXISTS ix_audit_log_action ON audit_log (action);
CREATE INDEX IF NOT EXISTS ix_audit_log_user_id ON audit_log (user_id);
CREATE INDEX IF NOT EXISTS ix_audit_log_action_created_at ON audit_log (action, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS ix_audit_log_created_id ON audit_log (created_at DESC, id DESC);
