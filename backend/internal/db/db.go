package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

func Open(databaseURL string) (*sql.DB, error) {
	driver, dsn, err := parseURL(databaseURL)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	if driver == "sqlite" {
		db.SetMaxOpenConns(1)
	}
	return db, db.Ping()
}

func parseURL(databaseURL string) (driver string, dsn string, err error) {
	if strings.HasPrefix(databaseURL, "sqlite:") {
		path := strings.TrimPrefix(databaseURL, "sqlite:///")
		if path == ":memory:" {
			return "sqlite", ":memory:", nil
		}
		return "sqlite", path, nil
	}
	if strings.Contains(databaseURL, "postgresql+psycopg") {
		databaseURL = strings.Replace(databaseURL, "postgresql+psycopg", "postgres", 1)
	}
	if strings.HasPrefix(databaseURL, "postgres://") || strings.HasPrefix(databaseURL, "postgresql://") {
		return "pgx", databaseURL, nil
	}
	return "", "", fmt.Errorf("unsupported DATABASE_URL")
}
