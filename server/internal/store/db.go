// SQLite connection and setup.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "embed"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schema string

func InitializeSchema(db *sql.DB) error {
	for _, stmt := range strings.Split(schema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	if err := runMigrations(db); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

// runMigrations applies additive schema changes for existing databases.
// Each migration is idempotent — safe to run multiple times.
func runMigrations(db *sql.DB) error {
	// Add app_name column to pending_captures for client-reported foreground app.
	db.Exec(`ALTER TABLE pending_captures ADD COLUMN app_name TEXT DEFAULT ''`)

	return nil
}

func Open(dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := InitializeSchema(db); err != nil {
		return nil, fmt.Errorf("initialize schema: %w", err)
	}

	return db, nil
}
