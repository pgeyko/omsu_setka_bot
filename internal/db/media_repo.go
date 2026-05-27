package db

import (
	"context"
	"fmt"
)

type MediaItem struct {
	MessageID    int
	MediaGroupID string
	ChatID       int64
	MediaType    string
}

// InsertMediaItem records a media group item.
func (d *DB) InsertMediaItem(ctx context.Context, messageID int, mediaGroupID string, chatID int64, mediaType string) error {
	_, err := d.ExecContext(ctx,
		`INSERT OR IGNORE INTO media_group_items (message_id, media_group_id, chat_id, media_type, created_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		messageID, mediaGroupID, chatID, mediaType,
	)
	if err != nil {
		return fmt.Errorf("failed to insert media item: %w", err)
	}
	return nil
}

// GetMediaItemsByGroupID returns all items for a given media group.
func (d *DB) GetMediaItemsByGroupID(ctx context.Context, mediaGroupID string) ([]MediaItem, error) {
	rows, err := d.QueryContext(ctx,
		"SELECT message_id, media_group_id, chat_id, COALESCE(media_type, '') FROM media_group_items WHERE media_group_id = ?",
		mediaGroupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []MediaItem
	for rows.Next() {
		var item MediaItem
		if err := rows.Scan(&item.MessageID, &item.MediaGroupID, &item.ChatID, &item.MediaType); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// DeleteExpiredMediaItems removes media group items older than the given TTL.
func (d *DB) DeleteExpiredMediaItems(ctx context.Context, ttlMinutes int) (int, error) {
	res, err := d.ExecContext(ctx,
		"DELETE FROM media_group_items WHERE created_at < datetime('now', ?)",
		fmt.Sprintf("-%d minutes", ttlMinutes),
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
