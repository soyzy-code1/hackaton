CREATE TABLE IF NOT EXISTS recurring_payment_users (
    recurring_payment_id INTEGER NOT NULL REFERENCES recurring_payments(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (recurring_payment_id, user_id)
);

CREATE INDEX IF NOT EXISTS ix_recurring_payment_users_user_id
    ON recurring_payment_users (user_id);

INSERT OR IGNORE INTO recurring_payment_users (recurring_payment_id, user_id)
SELECT p.id, u.id
FROM recurring_payments p
CROSS JOIN users u
WHERE u.role = 'user' AND u.is_active = 1;

UPDATE notifications
SET user_id = (
    SELECT rpu.user_id
    FROM recurring_payment_users rpu
    JOIN payment_occurrences po ON po.recurring_payment_id = rpu.recurring_payment_id
    WHERE po.id = notifications.occurrence_id
    ORDER BY rpu.user_id
    LIMIT 1
)
WHERE occurrence_id IS NOT NULL AND user_id IS NULL;
