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
	return nil
}
