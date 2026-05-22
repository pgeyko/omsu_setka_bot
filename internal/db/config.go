package db

import (
	"context"
	"database/sql"
	"errors"
)

// GetConfig reads a key from bot_config, returning the default if not set.
func (d *DB) GetConfig(ctx context.Context, key, defaultVal string) (string, error) {
	var val string
	err := d.QueryRowContext(ctx, "SELECT value FROM bot_config WHERE key = ?", key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultVal, nil
	}
	return val, err
}

// SetConfig upserts a key/value in bot_config.
func (d *DB) SetConfig(ctx context.Context, key, value string) error {
	_, err := d.ExecContext(ctx,
		"INSERT INTO bot_config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	return err
}

// RevokeToken inserts a JWT jti into the revoked_tokens blacklist.
func (d *DB) RevokeToken(ctx context.Context, jti string, expiresAt int64) error {
	_, err := d.ExecContext(ctx,
		"INSERT OR IGNORE INTO revoked_tokens (jti, expires_at) VALUES (?, ?)",
		jti, expiresAt,
	)
	return err
}

// IsTokenRevoked returns true when the given jti has been revoked.
func (d *DB) IsTokenRevoked(ctx context.Context, jti string) (bool, error) {
	var count int
	err := d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM revoked_tokens WHERE jti = ?", jti,
	).Scan(&count)
	return count > 0, err
}

// PruneRevokedTokens removes expired entries from the blacklist.
func (d *DB) PruneRevokedTokens(ctx context.Context) error {
	_, err := d.ExecContext(ctx,
		"DELETE FROM revoked_tokens WHERE expires_at < strftime('%s','now')",
	)
	return err
}
