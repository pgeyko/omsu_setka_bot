package db

import (
	"context"
	"fmt"
)

// InsertProcessedMessage records a message as processed.
func (d *DB) InsertProcessedMessage(ctx context.Context, messageID int, chatID int64, action string) error {
	_, err := d.ExecContext(ctx,
		`INSERT OR IGNORE INTO processed_messages (message_id, chat_id, action, processed_at)
		 VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		messageID, chatID, action,
	)
	if err != nil {
		return fmt.Errorf("failed to insert processed message: %w", err)
	}
	return nil
}

// UpdateProcessedMessage updates processing details for a previously inserted message.
func (d *DB) UpdateProcessedMessage(ctx context.Context, messageID int, chatID int64, threadID int, action string, targetThreadID int) error {
	_, err := d.ExecContext(ctx,
		`UPDATE processed_messages SET thread_id = ?, action = ?, target_thread_id = ?, processed_at = CURRENT_TIMESTAMP
		 WHERE message_id = ? AND chat_id = ?`,
		threadID, action, targetThreadID, messageID, chatID,
	)
	if err != nil {
		return fmt.Errorf("failed to update processed message: %w", err)
	}
	return nil
}


