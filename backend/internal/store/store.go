package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"invoicekeeper/internal/models"
)

type Store struct {
	DB *sql.DB
}

func (s *Store) GetUserByLogin(login string) (*models.User, error) {
	row := s.DB.QueryRow(`SELECT id, login, email, full_name, password_hash, role, is_active, created_at, updated_at
		FROM users WHERE login = ?`, login)
	return scanUser(row)
}

func (s *Store) GetUserByID(id int) (*models.User, error) {
	row := s.DB.QueryRow(`SELECT id, login, email, full_name, password_hash, role, is_active, created_at, updated_at
		FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (s *Store) ListUsers() ([]models.User, error) {
	rows, err := s.DB.Query(`SELECT id, login, email, full_name, password_hash, role, is_active, created_at, updated_at
		FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		u, err := scanUserRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

func (s *Store) CreateUser(u *models.User) error {
	res, err := s.DB.Exec(`INSERT INTO users (login, email, full_name, password_hash, role, is_active) VALUES (?, ?, ?, ?, ?, ?)`,
		u.Login, u.Email, u.FullName, u.PasswordHash, u.Role, u.IsActive)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	u.ID = int(id)
	return s.refreshUser(u)
}

func (s *Store) UpdateUser(u *models.User) error {
	_, err := s.DB.Exec(`UPDATE users SET email=?, full_name=?, password_hash=?, is_active=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		u.Email, u.FullName, u.PasswordHash, u.IsActive, u.ID)
	if err != nil {
		return err
	}
	return s.refreshUser(u)
}

func (s *Store) DeleteUser(id int) error {
	_, err := s.DB.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (s *Store) RevokeToken(jti string, expiresAt time.Time) error {
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO revoked_tokens (jti, expires_at) VALUES (?, ?)`, jti, expiresAt.UTC())
	return err
}

func (s *Store) IsTokenRevoked(jti string) bool {
	var n int
	err := s.DB.QueryRow(`SELECT 1 FROM revoked_tokens WHERE jti = ?`, jti).Scan(&n)
	return err == nil
}

func (s *Store) ListRecurringPayments(currency *string, isActive *bool) ([]models.RecurringPayment, error) {
	q := `SELECT id, title, amount, currency, periodicity, next_payment_date, is_active, created_by, created_at, updated_at, deleted_at
		FROM recurring_payments WHERE 1=1`
	var args []interface{}
	if currency != nil {
		q += ` AND currency = ?`
		args = append(args, *currency)
	}
	if isActive != nil {
		q += ` AND is_active = ?`
		args = append(args, *isActive)
	}
	q += ` ORDER BY id`
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPayments(rows)
}

func (s *Store) GetRecurringPayment(id int) (*models.RecurringPayment, error) {
	row := s.DB.QueryRow(`SELECT id, title, amount, currency, periodicity, next_payment_date, is_active, created_by, created_at, updated_at, deleted_at
		FROM recurring_payments WHERE id = ?`, id)
	return scanPayment(row)
}

func (s *Store) CreateRecurringPayment(p *models.RecurringPayment) error {
	res, err := s.DB.Exec(`INSERT INTO recurring_payments (title, amount, currency, periodicity, next_payment_date, is_active, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.Title, p.Amount.StringFixed(2), p.Currency, p.Periodicity, formatDate(p.NextPaymentDate), p.IsActive, p.CreatedBy)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	p.ID = int(id)
	return s.refreshPayment(p)
}

func (s *Store) UpdateRecurringPayment(p *models.RecurringPayment) error {
	_, err := s.DB.Exec(`UPDATE recurring_payments SET title=?, amount=?, currency=?, periodicity=?, next_payment_date=?, is_active=?, updated_at=CURRENT_TIMESTAMP, deleted_at=?
		WHERE id=?`,
		p.Title, p.Amount.StringFixed(2), p.Currency, p.Periodicity, formatDate(p.NextPaymentDate), p.IsActive, p.DeletedAt, p.ID)
	if err != nil {
		return err
	}
	return s.refreshPayment(p)
}

func (s *Store) SoftDeletePayment(id int) error {
	now := time.Now().UTC()
	_, err := s.DB.Exec(`UPDATE recurring_payments SET is_active=0, deleted_at=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, now, id)
	return err
}

func (s *Store) ListActivePaymentsForLazy() ([]models.RecurringPayment, error) {
	rows, err := s.DB.Query(`SELECT id, title, amount, currency, periodicity, next_payment_date, is_active, created_by, created_at, updated_at, deleted_at
		FROM recurring_payments WHERE is_active = 1 AND deleted_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPayments(rows)
}

func (s *Store) LastOccurrence(paymentID int) (*models.PaymentOccurrence, error) {
	row := s.DB.QueryRow(`SELECT id, recurring_payment_id, due_date, is_paid, paid_at, paid_by, reminder_sent_at, email_reminder_sent_at, webhook_reminder_sent_at, created_at, updated_at
		FROM payment_occurrences WHERE recurring_payment_id = ? ORDER BY due_date DESC LIMIT 1`, paymentID)
	return scanOccurrence(row)
}

func (s *Store) CreateOccurrence(o *models.PaymentOccurrence) error {
	res, err := s.DB.Exec(`INSERT INTO payment_occurrences (recurring_payment_id, due_date, is_paid, paid_at, paid_by, reminder_sent_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		o.RecurringPaymentID, formatDate(o.DueDate), o.IsPaid, nullableDate(o.PaidAt), o.PaidBy, o.ReminderSentAt)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	o.ID = int(id)
	return s.refreshOccurrence(o)
}

func (s *Store) UpdateOccurrence(o *models.PaymentOccurrence) error {
	_, err := s.DB.Exec(`UPDATE payment_occurrences SET is_paid=?, paid_at=?, paid_by=?, reminder_sent_at=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		o.IsPaid, nullableDate(o.PaidAt), o.PaidBy, o.ReminderSentAt, o.ID)
	return err
}

const occSelectCols = `o.id, o.recurring_payment_id, o.due_date, o.is_paid, o.paid_at, o.paid_by, o.reminder_sent_at, o.email_reminder_sent_at, o.webhook_reminder_sent_at, o.created_at, o.updated_at`

func (s *Store) GetOccurrence(id int) (*models.PaymentOccurrence, error) {
	row := s.DB.QueryRow(`SELECT `+occSelectCols+`,
		p.id, p.title, p.amount, p.currency, p.periodicity, p.next_payment_date, p.is_active, p.created_by, p.created_at, p.updated_at, p.deleted_at
		FROM payment_occurrences o
		JOIN recurring_payments p ON p.id = o.recurring_payment_id
		WHERE o.id = ?`, id)
	return scanOccurrenceWithPayment(row)
}

func (s *Store) ListOccurrences(currency *string, recurringID *int, from, to *time.Time) ([]models.PaymentOccurrence, error) {
	q := `SELECT ` + occSelectCols + `,
		p.id, p.title, p.amount, p.currency, p.periodicity, p.next_payment_date, p.is_active, p.created_by, p.created_at, p.updated_at, p.deleted_at
		FROM payment_occurrences o
		JOIN recurring_payments p ON p.id = o.recurring_payment_id WHERE 1=1`
	var args []interface{}
	if currency != nil {
		q += ` AND p.currency = ?`
		args = append(args, *currency)
	}
	if recurringID != nil {
		q += ` AND o.recurring_payment_id = ?`
		args = append(args, *recurringID)
	}
	if from != nil {
		q += ` AND o.due_date >= ?`
		args = append(args, formatDate(*from))
	}
	if to != nil {
		q += ` AND o.due_date <= ?`
		args = append(args, formatDate(*to))
	}
	q += ` ORDER BY o.due_date, o.id`
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.PaymentOccurrence
	for rows.Next() {
		o, err := scanOccurrenceWithPaymentRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}

func (s *Store) ListOccurrencesForSummary(from, to time.Time) ([]models.PaymentOccurrence, error) {
	rows, err := s.DB.Query(`SELECT `+occSelectCols+`,
		p.id, p.title, p.amount, p.currency, p.periodicity, p.next_payment_date, p.is_active, p.created_by, p.created_at, p.updated_at, p.deleted_at
		FROM payment_occurrences o
		JOIN recurring_payments p ON p.id = o.recurring_payment_id
		WHERE o.due_date >= ? AND o.due_date <= ? AND p.deleted_at IS NULL`,
		formatDate(from), formatDate(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.PaymentOccurrence
	for rows.Next() {
		o, err := scanOccurrenceWithPaymentRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}

func (s *Store) ListDueForReminder(today time.Time, horizon int) ([]models.PaymentOccurrence, error) {
	return s.listDueWithSentColumn(today, horizon, "reminder_sent_at")
}

func (s *Store) ListDueForEmailReminder(today time.Time, horizon int) ([]models.PaymentOccurrence, error) {
	return s.listDueWithSentColumn(today, horizon, "email_reminder_sent_at")
}

func (s *Store) ListDueForWebhookReminder(today time.Time, horizon int) ([]models.PaymentOccurrence, error) {
	return s.listDueWithSentColumn(today, horizon, "webhook_reminder_sent_at")
}

func (s *Store) MarkEmailReminderSent(id int) error {
	_, err := s.DB.Exec(`UPDATE payment_occurrences SET email_reminder_sent_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func (s *Store) MarkWebhookReminderSent(id int) error {
	_, err := s.DB.Exec(`UPDATE payment_occurrences SET webhook_reminder_sent_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func (s *Store) listDueWithSentColumn(today time.Time, horizon int, sentColumn string) ([]models.PaymentOccurrence, error) {
	switch sentColumn {
	case "reminder_sent_at", "email_reminder_sent_at", "webhook_reminder_sent_at":
	default:
		return nil, errors.New("invalid sent column")
	}
	q := `SELECT ` + occSelectCols + `,
		p.id, p.title, p.amount, p.currency, p.periodicity, p.next_payment_date, p.is_active, p.created_by, p.created_at, p.updated_at, p.deleted_at
		FROM payment_occurrences o
		JOIN recurring_payments p ON p.id = o.recurring_payment_id
		WHERE o.is_paid = 0 AND (o.` + sentColumn + ` IS NULL OR o.` + sentColumn + ` = '') AND o.due_date >= ?
		AND p.deleted_at IS NULL AND p.is_active = 1`
	rows, err := s.DB.Query(q, formatDate(today))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.PaymentOccurrence
	for rows.Next() {
		o, err := scanOccurrenceWithPaymentRows(rows)
		if err != nil {
			return nil, err
		}
		dl := int(o.DueDate.Sub(today).Hours() / 24)
		if dl <= horizon {
			out = append(out, *o)
		}
	}
	return out, rows.Err()
}

func scanUser(row *sql.Row) (*models.User, error) {
	var u models.User
	var email, fullName sql.NullString
	var updated sql.NullTime
	err := row.Scan(&u.ID, &u.Login, &email, &fullName, &u.PasswordHash, &u.Role, &u.IsActive, &u.CreatedAt, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if email.Valid {
		u.Email = &email.String
	}
	if fullName.Valid {
		u.FullName = &fullName.String
	}
	if updated.Valid {
		u.UpdatedAt = &updated.Time
	}
	return &u, nil
}

func scanUserRows(rows *sql.Rows) (*models.User, error) {
	var u models.User
	var email, fullName sql.NullString
	var updated sql.NullTime
	err := rows.Scan(&u.ID, &u.Login, &email, &fullName, &u.PasswordHash, &u.Role, &u.IsActive, &u.CreatedAt, &updated)
	if err != nil {
		return nil, err
	}
	if email.Valid {
		u.Email = &email.String
	}
	if fullName.Valid {
		u.FullName = &fullName.String
	}
	if updated.Valid {
		u.UpdatedAt = &updated.Time
	}
	return &u, nil
}

func scanPayment(row *sql.Row) (*models.RecurringPayment, error) {
	var p models.RecurringPayment
	var amount string
	var nextDate time.Time
	var updated, deleted sql.NullTime
	err := row.Scan(&p.ID, &p.Title, &amount, &p.Currency, &p.Periodicity, &nextDate, &p.IsActive, &p.CreatedBy, &p.CreatedAt, &updated, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Amount, _ = decimal.NewFromString(amount)
	p.NextPaymentDate = dateOnly(nextDate)
	if updated.Valid {
		p.UpdatedAt = &updated.Time
	}
	if deleted.Valid {
		p.DeletedAt = &deleted.Time
	}
	return &p, nil
}

func scanPayments(rows *sql.Rows) ([]models.RecurringPayment, error) {
	var out []models.RecurringPayment
	for rows.Next() {
		var p models.RecurringPayment
		var amount string
		var nextDate time.Time
		var updated, deleted sql.NullTime
		if err := rows.Scan(&p.ID, &p.Title, &amount, &p.Currency, &p.Periodicity, &nextDate, &p.IsActive, &p.CreatedBy, &p.CreatedAt, &updated, &deleted); err != nil {
			return nil, err
		}
		p.Amount, _ = decimal.NewFromString(amount)
		p.NextPaymentDate = dateOnly(nextDate)
		if updated.Valid {
			p.UpdatedAt = &updated.Time
		}
		if deleted.Valid {
			p.DeletedAt = &deleted.Time
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanOccurrence(row *sql.Row) (*models.PaymentOccurrence, error) {
	var o models.PaymentOccurrence
	var due time.Time
	var paidAt sql.NullTime
	var paidBy sql.NullInt64
	var reminder, emailSent, webhookSent, updated sql.NullTime
	err := row.Scan(&o.ID, &o.RecurringPaymentID, &due, &o.IsPaid, &paidAt, &paidBy, &reminder, &emailSent, &webhookSent, &o.CreatedAt, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	o.DueDate = dateOnly(due)
	if paidAt.Valid {
		t := dateOnly(paidAt.Time)
		o.PaidAt = &t
	}
	if paidBy.Valid {
		v := int(paidBy.Int64)
		o.PaidBy = &v
	}
	if reminder.Valid {
		o.ReminderSentAt = &reminder.Time
	}
	if emailSent.Valid {
		o.EmailReminderSentAt = &emailSent.Time
	}
	if webhookSent.Valid {
		o.WebhookReminderSentAt = &webhookSent.Time
	}
	if updated.Valid {
		o.UpdatedAt = &updated.Time
	}
	return &o, nil
}

func scanOccurrenceWithPayment(row *sql.Row) (*models.PaymentOccurrence, error) {
	o, p, err := scanOccurrencePaymentFields(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	o.Payment = p
	return o, nil
}

func scanOccurrenceWithPaymentRows(rows *sql.Rows) (*models.PaymentOccurrence, error) {
	o, p, err := scanOccurrencePaymentFieldsRows(rows)
	if err != nil {
		return nil, err
	}
	o.Payment = p
	return o, nil
}

func scanOccurrencePaymentFields(row *sql.Row) (*models.PaymentOccurrence, *models.RecurringPayment, error) {
	var o models.PaymentOccurrence
	var p models.RecurringPayment
	var due time.Time
	var paidAt sql.NullTime
	var paidBy sql.NullInt64
	var reminder, emailSent, webhookSent, updatedO sql.NullTime
	var amount string
	var nextDate time.Time
	var updatedP, deleted sql.NullTime
	err := row.Scan(
		&o.ID, &o.RecurringPaymentID, &due, &o.IsPaid, &paidAt, &paidBy, &reminder, &emailSent, &webhookSent, &o.CreatedAt, &updatedO,
		&p.ID, &p.Title, &amount, &p.Currency, &p.Periodicity, &nextDate, &p.IsActive, &p.CreatedBy, &p.CreatedAt, &updatedP, &deleted,
	)
	if err != nil {
		return nil, nil, err
	}
	o.DueDate = dateOnly(due)
	fillOccurrenceOptional(&o, paidAt, paidBy, reminder, emailSent, webhookSent, updatedO)
	p.Amount, _ = decimal.NewFromString(amount)
	p.NextPaymentDate = dateOnly(nextDate)
	if updatedP.Valid {
		p.UpdatedAt = &updatedP.Time
	}
	if deleted.Valid {
		p.DeletedAt = &deleted.Time
	}
	return &o, &p, nil
}

func scanOccurrencePaymentFieldsRows(rows *sql.Rows) (*models.PaymentOccurrence, *models.RecurringPayment, error) {
	var o models.PaymentOccurrence
	var p models.RecurringPayment
	var due time.Time
	var paidAt sql.NullTime
	var paidBy sql.NullInt64
	var reminder, emailSent, webhookSent, updatedO sql.NullTime
	var amount string
	var nextDate time.Time
	var updatedP, deleted sql.NullTime
	err := rows.Scan(
		&o.ID, &o.RecurringPaymentID, &due, &o.IsPaid, &paidAt, &paidBy, &reminder, &emailSent, &webhookSent, &o.CreatedAt, &updatedO,
		&p.ID, &p.Title, &amount, &p.Currency, &p.Periodicity, &nextDate, &p.IsActive, &p.CreatedBy, &p.CreatedAt, &updatedP, &deleted,
	)
	if err != nil {
		return nil, nil, err
	}
	o.DueDate = dateOnly(due)
	fillOccurrenceOptional(&o, paidAt, paidBy, reminder, emailSent, webhookSent, updatedO)
	p.Amount, _ = decimal.NewFromString(amount)
	p.NextPaymentDate = dateOnly(nextDate)
	if updatedP.Valid {
		p.UpdatedAt = &updatedP.Time
	}
	if deleted.Valid {
		p.DeletedAt = &deleted.Time
	}
	return &o, &p, nil
}

func fillOccurrenceOptional(o *models.PaymentOccurrence, paidAt sql.NullTime, paidBy sql.NullInt64, reminder, emailSent, webhookSent, updated sql.NullTime) {
	if paidAt.Valid {
		t := dateOnly(paidAt.Time)
		o.PaidAt = &t
	}
	if paidBy.Valid {
		v := int(paidBy.Int64)
		o.PaidBy = &v
	}
	if reminder.Valid {
		o.ReminderSentAt = &reminder.Time
	}
	if emailSent.Valid {
		o.EmailReminderSentAt = &emailSent.Time
	}
	if webhookSent.Valid {
		o.WebhookReminderSentAt = &webhookSent.Time
	}
	if updated.Valid {
		o.UpdatedAt = &updated.Time
	}
}

func (s *Store) refreshUser(u *models.User) error {
	got, err := s.GetUserByID(u.ID)
	if err != nil || got == nil {
		return err
	}
	*u = *got
	return nil
}

func (s *Store) refreshPayment(p *models.RecurringPayment) error {
	got, err := s.GetRecurringPayment(p.ID)
	if err != nil || got == nil {
		return err
	}
	*p = *got
	return nil
}

func (s *Store) refreshOccurrence(o *models.PaymentOccurrence) error {
	got, err := s.GetOccurrence(o.ID)
	if err != nil || got == nil {
		return err
	}
	*o = *got
	return nil
}

func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

func parseDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return dateOnly(t)
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func nullableDate(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return formatDate(*t)
}
