ALTER TABLE payment_occurrences ADD COLUMN email_reminder_sent_at TIMESTAMP;
ALTER TABLE payment_occurrences ADD COLUMN webhook_reminder_sent_at TIMESTAMP;
