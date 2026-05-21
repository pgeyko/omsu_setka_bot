package db

import (
	"fmt"
	"log/slog"
)

func (d *DB) Migrate() error {
	// Check if we need to migrate from legacy single-tenant schema
	var exists int
	err := d.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='groups'`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check for groups table: %w", err)
	}

	if exists == 0 {
		slog.Info("legacy database schema detected, performing clean schema drop for migration")
		dropQueries := []string{
			"DROP TABLE IF EXISTS bot_persona;",
			"DROP TABLE IF EXISTS topics;",
			"DROP TABLE IF EXISTS processed_messages;",
			"DROP TABLE IF EXISTS llm_requests;",
			"DROP TABLE IF EXISTS schedule_snapshots;",
			"DROP TABLE IF EXISTS schedule_anomalies;",
			"DROP TABLE IF EXISTS command_permissions;",
			"DROP TABLE IF EXISTS summary_requests;",
			"DROP TABLE IF EXISTS message_buffer;",
		}
		for _, q := range dropQueries {
			if _, err := d.Exec(q); err != nil {
				return fmt.Errorf("failed to drop legacy table: %w\nQuery: %s", err, q)
			}
		}
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS groups (
			chat_id            INTEGER PRIMARY KEY,
			title              TEXT NOT NULL,
			api_token          TEXT NOT NULL UNIQUE,
			omsu_group_id      INTEGER NOT NULL DEFAULT 0,
			announce_thread_id INTEGER NOT NULL DEFAULT 0,
			is_active          INTEGER NOT NULL DEFAULT 1,
			is_vip             INTEGER NOT NULL DEFAULT 0,
			created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS superadmins (
			user_id    INTEGER PRIMARY KEY,
			note       TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS topics (
			id           INTEGER PRIMARY KEY,
			group_id     INTEGER NOT NULL REFERENCES groups(chat_id) ON DELETE CASCADE,
			tg_thread_id INTEGER NOT NULL,
			name         TEXT NOT NULL,
			slug         TEXT NOT NULL,
			aliases      TEXT NOT NULL DEFAULT '[]',
			description  TEXT NOT NULL DEFAULT '',
			hashtags     TEXT NOT NULL DEFAULT '[]',
			is_active    INTEGER NOT NULL DEFAULT 1,
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(group_id, tg_thread_id),
			UNIQUE(group_id, slug)
		);`,

		`CREATE TABLE IF NOT EXISTS processed_messages (
			id               INTEGER PRIMARY KEY,
			message_id       INTEGER NOT NULL,
			chat_id          INTEGER NOT NULL REFERENCES groups(chat_id) ON DELETE CASCADE,
			thread_id        INTEGER,
			action           TEXT NOT NULL,
			target_thread_id INTEGER,
			processed_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(message_id, chat_id)
		);`,

		`CREATE TABLE IF NOT EXISTS llm_requests (
			id            INTEGER PRIMARY KEY,
			group_id      INTEGER REFERENCES groups(chat_id) ON DELETE SET NULL,
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
			group_id   INTEGER NOT NULL DEFAULT 0,
			data       TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS schedule_anomalies (
			id          INTEGER PRIMARY KEY,
			snapshot_id INTEGER NOT NULL REFERENCES schedule_snapshots(id) ON DELETE CASCADE,
			type        TEXT NOT NULL,
			details     TEXT NOT NULL,
			notified    INTEGER NOT NULL DEFAULT 0,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS command_permissions (
			group_id     INTEGER NOT NULL REFERENCES groups(chat_id) ON DELETE CASCADE,
			command      TEXT NOT NULL,
			allowed_role TEXT NOT NULL DEFAULT 'everyone',
			PRIMARY KEY (group_id, command)
		);`,

		`CREATE TABLE IF NOT EXISTS summary_requests (
			user_id      INTEGER NOT NULL,
			chat_id      INTEGER NOT NULL REFERENCES groups(chat_id) ON DELETE CASCADE,
			requested_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, chat_id)
		);`,

		`CREATE TABLE IF NOT EXISTS message_buffer (
			chat_id    INTEGER NOT NULL REFERENCES groups(chat_id) ON DELETE CASCADE,
			thread_id  INTEGER NOT NULL,
			username   TEXT NOT NULL,
			text       TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE INDEX IF NOT EXISTS idx_processed_messages_chat ON processed_messages(chat_id, message_id);`,
		`CREATE INDEX IF NOT EXISTS idx_llm_requests_date ON llm_requests(created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_anomalies_notified ON schedule_anomalies(notified);`,
		`CREATE INDEX IF NOT EXISTS idx_buffer_chat_thread ON message_buffer(chat_id, thread_id, created_at DESC);`,
	}

	for _, q := range queries {
		if _, err := d.Exec(q); err != nil {
			return fmt.Errorf("migration failed: %w\nQuery: %s", err, q)
		}
	}

	slog.Info("database migration completed")
	return nil
}
