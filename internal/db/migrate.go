package db

import (
	"fmt"
	"log/slog"
)

func (d *DB) Migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS bot_persona (
			id            INTEGER PRIMARY KEY CHECK (id = 1),
			name          TEXT NOT NULL DEFAULT 'Помощник',
			system_prompt TEXT NOT NULL DEFAULT '',
			signature     TEXT NOT NULL DEFAULT '',
			updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS topics (
			id           INTEGER PRIMARY KEY,
			tg_thread_id INTEGER NOT NULL UNIQUE,
			name         TEXT NOT NULL,
			slug         TEXT NOT NULL UNIQUE,
			aliases      TEXT NOT NULL DEFAULT '[]',
			description  TEXT NOT NULL DEFAULT '',
			hashtags     TEXT NOT NULL DEFAULT '[]',
			is_active    INTEGER NOT NULL DEFAULT 1,
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS processed_messages (
			id               INTEGER PRIMARY KEY,
			message_id       INTEGER NOT NULL,
			chat_id          INTEGER NOT NULL,
			thread_id        INTEGER,
			action           TEXT NOT NULL,
			target_thread_id INTEGER,
			processed_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(message_id, chat_id)
		);`,

		`CREATE TABLE IF NOT EXISTS llm_requests (
			id            INTEGER PRIMARY KEY,
			type          TEXT NOT NULL,
			provider      TEXT NOT NULL DEFAULT '',
			input_tokens  INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			model         TEXT NOT NULL,
			cost_usd      REAL NOT NULL DEFAULT 0,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS schedule_snapshots (
			id         INTEGER PRIMARY KEY,
			data       TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS schedule_anomalies (
			id          INTEGER PRIMARY KEY,
			snapshot_id INTEGER NOT NULL REFERENCES schedule_snapshots(id),
			type        TEXT NOT NULL,
			details     TEXT NOT NULL,
			notified    INTEGER NOT NULL DEFAULT 0,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS command_permissions (
			command      TEXT PRIMARY KEY,
			allowed_role TEXT NOT NULL DEFAULT 'everyone'
		);`,

		`CREATE TABLE IF NOT EXISTS summary_requests (
			user_id      INTEGER NOT NULL,
			chat_id      INTEGER NOT NULL,
			requested_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, chat_id)
		);`,

		`CREATE TABLE IF NOT EXISTS message_buffer (
			thread_id  INTEGER NOT NULL,
			username   TEXT NOT NULL,
			text       TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE INDEX IF NOT EXISTS idx_processed_messages_chat ON processed_messages(chat_id, message_id);`,
		`CREATE INDEX IF NOT EXISTS idx_llm_requests_date ON llm_requests(created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_anomalies_notified ON schedule_anomalies(notified);`,
		`CREATE INDEX IF NOT EXISTS idx_buffer_thread ON message_buffer(thread_id, created_at DESC);`,
	}

	for _, q := range queries {
		if _, err := d.Exec(q); err != nil {
			return fmt.Errorf("migration failed: %w\nQuery: %s", err, q)
		}
	}

	slog.Info("database migration completed")
	return nil
}
