package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"omsu_bot/internal/agent"
	omsudb "omsu_bot/internal/db"
	"omsu_bot/internal/llm"
	"omsu_bot/internal/persona"
	"omsu_bot/internal/config"
)

// SetupLogger configures the structured logger based on config.
func SetupLogger(cfg *config.Config) {
	level := slog.LevelInfo
	if cfg.Logging.Level == "debug" {
		level = slog.LevelDebug
	} else if cfg.Logging.Level == "warn" {
		level = slog.LevelWarn
	} else if cfg.Logging.Level == "error" {
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if cfg.Logging.Format == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(h))
}

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

// InitPersona loads the bot persona from the seed file.
func InitPersona(db *omsudb.DB, prompts *llm.PromptRegistry) *persona.Store {
	store := persona.NewStore(db.DB)
	if err := store.Load(context.Background(), "prompts/persona.md"); err != nil {
		slog.Error("failed to load persona", "error", err)
		os.Exit(1)
	}
	return store
}

// StartSighupHandler reloads prompts and protocols on SIGHUP signal.
func StartSighupHandler(sighupCtx context.Context, prompts *llm.PromptRegistry) {
	go func() {
		<-sighupCtx.Done()
		slog.Info("SIGHUP received, reloading prompts and protocols")
		if err := prompts.Reload(); err != nil {
			slog.Error("failed to reload prompts", "error", err)
		} else {
			slog.Info("prompts reloaded successfully")
		}
		agent.ReloadProtocols()
		slog.Info("protocols reloaded")
	}()
}
