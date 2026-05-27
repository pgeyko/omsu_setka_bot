package db

import (
	"context"
	"fmt"
)

type Topic struct {
	ID          int64
	GroupID     int64
	ThreadID    int
	Name        string
	Slug        string
	Aliases     string
	Description string
	Hashtags    string
	IsActive    bool
}

// CountActiveByGroup returns the number of active topics for a group.
func (d *DB) CountActiveByGroup(ctx context.Context, chatID int64) (int, error) {
	var count int
	err := d.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM topics WHERE group_id = ? AND is_active = 1", chatID,
	).Scan(&count)
	return count, err
}

// GetByThreadID returns a topic name by Telegram thread ID.
func (d *DB) GetByThreadID(ctx context.Context, chatID int64, threadID int) (string, error) {
	var name string
	err := d.QueryRowContext(ctx,
		"SELECT name FROM topics WHERE group_id = ? AND tg_thread_id = ?", chatID, threadID,
	).Scan(&name)
	return name, err
}

// GetBySlug returns a topic's thread ID by its slug.
func (d *DB) GetBySlug(ctx context.Context, chatID int64, slug string) (int, error) {
	var threadID int
	err := d.QueryRowContext(ctx,
		"SELECT tg_thread_id FROM topics WHERE group_id = ? AND slug = ? AND is_active = 1", chatID, slug,
	).Scan(&threadID)
	return threadID, err
}

// FindByName looks up a topic by case-insensitive name match.
func (d *DB) FindByName(ctx context.Context, chatID int64, name string) (int, error) {
	var threadID int
	err := d.QueryRowContext(ctx,
		"SELECT tg_thread_id FROM topics WHERE group_id = ? AND LOWER(name) = ? AND is_active = 1 LIMIT 1",
		chatID, name,
	).Scan(&threadID)
	return threadID, err
}

type TopicNameRow struct {
	Name     string
	ThreadID int
}

// GetActiveByGroup returns slugs and thread IDs of all active topics.
func (d *DB) GetActiveByGroup(ctx context.Context, chatID int64) ([]TopicNameRow, error) {
	rows, err := d.QueryContext(ctx,
		"SELECT slug, tg_thread_id FROM topics WHERE group_id = ? AND is_active = 1", chatID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TopicNameRow
	for rows.Next() {
		var r TopicNameRow
		if err := rows.Scan(&r.Name, &r.ThreadID); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetNamesByGroup returns names and thread IDs of all active topics.
func (d *DB) GetNamesByGroup(ctx context.Context, chatID int64) ([]TopicNameRow, error) {
	rows, err := d.QueryContext(ctx,
		"SELECT name, tg_thread_id FROM topics WHERE group_id = ? AND is_active = 1 ORDER BY name",
		chatID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TopicNameRow
	for rows.Next() {
		var r TopicNameRow
		if err := rows.Scan(&r.Name, &r.ThreadID); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// SearchBySlugPrefix returns topics whose slug starts with the given prefix.
func (d *DB) SearchBySlugPrefix(ctx context.Context, chatID int64, prefix string) ([]TopicNameRow, error) {
	rows, err := d.QueryContext(ctx,
		"SELECT slug, tg_thread_id FROM topics WHERE group_id = ? AND is_active = 1 AND slug LIKE ?",
		chatID, prefix+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TopicNameRow
	for rows.Next() {
		var r TopicNameRow
		if err := rows.Scan(&r.Name, &r.ThreadID); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// Create inserts a new topic.
func (d *DB) CreateTopic(ctx context.Context, groupID int64, threadID int, name, slug string) error {
	_, err := d.ExecContext(ctx,
		`INSERT OR IGNORE INTO topics (group_id, tg_thread_id, name, slug, is_active) VALUES (?, ?, ?, ?, 1)`,
		groupID, threadID, name, slug,
	)
	if err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}
	return nil
}

// UpdateName changes the name of a topic.
func (d *DB) UpdateTopicName(ctx context.Context, groupID int64, threadID int, name string) error {
	_, err := d.ExecContext(ctx,
		"UPDATE topics SET name = ? WHERE group_id = ? AND tg_thread_id = ?",
		name, groupID, threadID,
	)
	return err
}

// SetActive toggles a topic's active state.
func (d *DB) SetTopicActive(ctx context.Context, groupID int64, threadID int, active bool) error {
	_, err := d.ExecContext(ctx,
		"UPDATE topics SET is_active = ? WHERE group_id = ? AND tg_thread_id = ?",
		active, groupID, threadID,
	)
	return err
}
