package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	omsudb "omsu_bot/internal/db"
)

// InitDB opens the database, runs migrations, and starts cleanup goroutine.
func InitDB(path string) *omsudb.DB {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("app: failed to create db dir", "error", err)
	}

	database, err := omsudb.New(path)
	if err != nil {
		slog.Error("failed to open database", "path", path, "error", err)
		os.Exit(1)
	}

	if err := database.Migrate(); err != nil {
		slog.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	go database.StartCleanup(context.Background())
	slog.Info("app: database maintenance goroutine started")

	return database
}
