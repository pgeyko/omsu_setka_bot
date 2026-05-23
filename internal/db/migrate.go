package db

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

func (d *DB) Migrate() error {
	// Check if we need to migrate from legacy single-tenant schema
	var exists int
	err := d.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='groups'`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check for groups table: %w", err)
	}

	if exists == 0 {
		slog.Warn("legacy database schema detected, performing clean schema drop for migration")

		// Backup the database before destructive migration
		if dbPath, err := d.backupBeforeMigration(); err != nil {
			slog.Error("failed to backup database before migration", "error", err)
		} else if dbPath != "" {
			slog.Warn("database backed up before migration", "backup", dbPath)
		}

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
			chat_id       INTEGER PRIMARY KEY,
			title         TEXT NOT NULL,
			api_token     TEXT NOT NULL UNIQUE,
			omsu_group_id INTEGER NOT NULL DEFAULT 0,
			is_active     INTEGER NOT NULL DEFAULT 1,
			is_vip        INTEGER NOT NULL DEFAULT 0,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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
			message_id INTEGER NOT NULL DEFAULT 0,
			username   TEXT NOT NULL,
			text       TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE INDEX IF NOT EXISTS idx_processed_messages_chat ON processed_messages(chat_id, message_id);`,
		`CREATE INDEX IF NOT EXISTS idx_llm_requests_date ON llm_requests(created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_anomalies_notified ON schedule_anomalies(notified);`,
		`CREATE INDEX IF NOT EXISTS idx_buffer_chat_thread ON message_buffer(chat_id, thread_id, created_at DESC);`,

		// Per-message hashtags for search and analytics
		`CREATE TABLE IF NOT EXISTS message_tags (
			id         INTEGER PRIMARY KEY,
			chat_id    INTEGER NOT NULL REFERENCES groups(chat_id) ON DELETE CASCADE,
			message_id INTEGER NOT NULL,
			tag        TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_message_tags_chat_tag ON message_tags(chat_id, tag);`,
		`CREATE INDEX IF NOT EXISTS idx_message_tags_created ON message_tags(created_at);`,

		// Persistent runtime configuration (FIX-04)
		`CREATE TABLE IF NOT EXISTS bot_config (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		);`,

		// JWT revocation blacklist (FIX-11)
		`CREATE TABLE IF NOT EXISTS revoked_tokens (
			jti        TEXT PRIMARY KEY,
			expires_at INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_revoked_tokens_exp ON revoked_tokens(expires_at);`,
		`CREATE TABLE IF NOT EXISTS media_group_items (
			media_group_id TEXT NOT NULL,
			message_id     INTEGER NOT NULL,
			chat_id        INTEGER NOT NULL,
			file_id        TEXT NOT NULL DEFAULT '',
			caption        TEXT NOT NULL DEFAULT '',
			created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (media_group_id, message_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_media_group_items_group ON media_group_items(media_group_id, chat_id);`,
	}

	for _, q := range queries {
		if _, err := d.Exec(q); err != nil {
			return fmt.Errorf("migration failed: %w\nQuery: %s", err, q)
		}
	}

	// Dynamic migration: Check if message_id exists in message_buffer
	var hasMessageID bool
	rows, err := d.Query(`PRAGMA table_info(message_buffer)`)
	if err != nil {
		return fmt.Errorf("failed to get table info for message_buffer: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan table info for message_buffer: %w", err)
		}
		if name == "message_id" {
			hasMessageID = true
			break
		}
	}
	if !hasMessageID {
		slog.Info("migrating message_buffer: adding message_id column")
		if _, err := d.Exec(`ALTER TABLE message_buffer ADD COLUMN message_id INTEGER NOT NULL DEFAULT 0;`); err != nil {
			return fmt.Errorf("failed to add message_id to message_buffer: %w", err)
		}
	}

	slog.Info("database migration completed")
	return nil
}

func (d *DB) backupBeforeMigration() (string, error) {
	backupPath := fmt.Sprintf("data/groupbot.bak.%d", time.Now().Unix())

	// Try VACUUM INTO (SQLite 3.27.0+) for a safe online backup
	_, err := d.Exec(fmt.Sprintf("VACUUM INTO '%s'", backupPath))
	if err == nil {
		return backupPath, nil
	}

	// Fallback: copy the database file from the source path
	slog.Warn("VACUUM INTO failed, trying file copy fallback", "error", err)

	// Try to find the database path from PRAGMA
	var dbPath string
	row := d.QueryRow("PRAGMA database_list")
	var seq int
	var name, file string
	if scanErr := row.Scan(&seq, &name, &file); scanErr == nil && file != "" {
		dbPath = file
	}

	if dbPath == "" {
		return "", fmt.Errorf("cannot determine database file path for backup")
	}

	input, err := os.ReadFile(dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to read database file for backup: %w", err)
	}
	if err := os.WriteFile(backupPath, input, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup file: %w", err)
	}
	return backupPath, nil
}
