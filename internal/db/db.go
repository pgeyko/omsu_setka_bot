package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func New(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db dir: %w", err)
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// SQLite creates a separate in-memory database per connection, so tests that
	// use :memory: must stay on a single shared connection.
	if dbPath == ":memory:" {
		conn.SetMaxOpenConns(1)
		conn.SetMaxIdleConns(1)
	} else {
		conn.SetMaxOpenConns(4)
		conn.SetMaxIdleConns(4)
		conn.SetConnMaxLifetime(30 * time.Minute)
	}

	if _, err := conn.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL: %w", err)
	}
	if _, err := conn.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
		return nil, fmt.Errorf("failed to set synchronous mode: %w", err)
	}
	if _, err := conn.Exec("PRAGMA busy_timeout=5000;"); err != nil {
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}
	if _, err := conn.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	slog.Info("database opened", "path", dbPath)
	return &DB{conn}, nil
}

func (d *DB) Close() error {
	return d.DB.Close()
}
