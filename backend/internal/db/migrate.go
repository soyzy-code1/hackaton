package db

import (
	"database/sql"
	_ "embed"
	"strings"
)

//go:embed migrations/001_init.sql
var initSQL string

//go:embed migrations/002_notify_channels.sql
var notifyChannelsSQL string

//go:embed migrations/003_notifications.sql
var notificationsSQL string

//go:embed migrations/004_notification_users.sql
var notificationUsersSQL string

//go:embed migrations/005_user_budgets.sql
var userBudgetsSQL string

//go:embed migrations/006_payment_assignments.sql
var paymentAssignmentsSQL string

//go:embed migrations/007_audit_log.sql
var auditLogSQL string

//go:embed migrations/008_categories.sql
var categoriesSQL string

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(initSQL); err != nil {
		return err
	}
	for _, stmt := range strings.Split(notifyChannelsSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}
	for _, stmt := range strings.Split(notificationsSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return err
		}
	}
	for _, stmt := range strings.Split(notificationUsersSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") && !strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return err
		}
	}
	for _, stmt := range strings.Split(userBudgetsSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return err
		}
	}
	for _, stmt := range strings.Split(paymentAssignmentsSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" { continue }
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return err
		}
	}
	for _, stmt := range strings.Split(auditLogSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" { continue }
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return err
		}
	}
	for _, stmt := range strings.Split(categoriesSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" { continue }
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") && !strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return err
		}
	}
	return nil
}
