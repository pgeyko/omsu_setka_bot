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

	// Use _pragma= in DSN so PRAGMAs apply to ALL connections from the pool,
	// not just the first one returned by db.Exec().
	dsn := dbPath
	if dbPath != ":memory:" {
		dsn += "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	}

	conn, err := sql.Open("sqlite", dsn)
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

	slog.Info("database opened", "path", dbPath)
	return &DB{conn}, nil
}

func (d *DB) Close() error {
	return d.DB.Close()
}
