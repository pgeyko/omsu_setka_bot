package db

import (
	"context"
	"fmt"
	"time"
)

type TaggedMessage struct {
	MessageID int
	Tag       string
	CreatedAt time.Time
}

func (d *DB) AddTags(ctx context.Context, chatID int64, messageID int, tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	for _, tag := range tags {
		_, err := d.ExecContext(ctx,
			`INSERT INTO message_tags (chat_id, message_id, tag, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
			chatID, messageID, tag,
		)
		if err != nil {
			return fmt.Errorf("failed to save tag %q: %w", tag, err)
		}
	}
	return nil
}

type TagResult struct {
	MessageID  int
	Username   string
	Text       string
	Tag        string
	CreatedAt  time.Time
	ThreadID   int
}

func (d *DB) GetMessagesByTag(ctx context.Context, chatID int64, tag string, limit int) ([]TagResult, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.QueryContext(ctx, `
		SELECT mt.message_id, COALESCE(mb.username, ''), COALESCE(mb.text, ''), mt.tag, mt.created_at, COALESCE(mb.thread_id, 0)
		FROM message_tags mt
		LEFT JOIN message_buffer mb ON mb.chat_id = mt.chat_id AND mb.message_id = mt.message_id
		WHERE mt.chat_id = ? AND mt.tag = ?
		ORDER BY mt.created_at DESC
		LIMIT ?
	`, chatID, tag, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TagResult
	for rows.Next() {
		var r TagResult
		if err := rows.Scan(&r.MessageID, &r.Username, &r.Text, &r.Tag, &r.CreatedAt, &r.ThreadID); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

type TagStat struct {
	Tag    string `json:"tag"`
	Count  int    `json:"count"`
}

func (d *DB) GetTagStats(ctx context.Context, chatID int64) ([]TagStat, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT tag, COUNT(*) as cnt
		FROM message_tags
		WHERE chat_id = ?
		GROUP BY tag
		ORDER BY cnt DESC
		LIMIT 50
	`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []TagStat
	for rows.Next() {
		var s TagStat
		if err := rows.Scan(&s.Tag, &s.Count); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

func (d *DB) HasMessageWithText(ctx context.Context, chatID int64, threadID int, textHash string) (bool, error) {
	var count int
	err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM message_buffer WHERE chat_id = ? AND thread_id = ? AND substr(text, 1, 100) = ?`,
		chatID, threadID, textHash,
	).Scan(&count)
	return count > 0, err
}
