ALTER TABLE notifications ADD COLUMN user_id INTEGER REFERENCES users(id);

CREATE INDEX IF NOT EXISTS ix_notifications_user_id ON notifications (user_id);
