package db

import (
	"context"
	"fmt"
	"time"
)

type Group struct {
	ChatID           int64     `json:"chat_id"`
	Title            string    `json:"title"`
	APIToken         string    `json:"api_token"`
	OmsuGroupID      int       `json:"omsu_group_id"`
	AnnounceThreadID int       `json:"announce_thread_id"`
	IsActive         bool      `json:"is_active"`
	IsVIP            bool      `json:"is_vip"`
	CreatedAt        time.Time `json:"created_at"`
}

type Superadmin struct {
	UserID    int64     `json:"user_id"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateGroup inserts a new group into the database
func (d *DB) CreateGroup(ctx context.Context, g *Group) error {
	_, err := d.ExecContext(ctx,
		`INSERT INTO groups (chat_id, title, api_token, omsu_group_id, announce_thread_id, is_active, is_vip, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		g.ChatID, g.Title, g.APIToken, g.OmsuGroupID, g.AnnounceThreadID, g.IsActive, g.IsVIP,
	)
	if err != nil {
		return fmt.Errorf("failed to create group: %w", err)
	}
	return nil
}

// GetGroup retrieves a group by chat_id
func (d *DB) GetGroup(ctx context.Context, chatID int64) (*Group, error) {
	var g Group
	var createdAt string
	err := d.QueryRowContext(ctx,
		`SELECT chat_id, title, api_token, omsu_group_id, announce_thread_id, is_active, is_vip, created_at
		 FROM groups WHERE chat_id = ?`,
		chatID,
	).Scan(&g.ChatID, &g.Title, &g.APIToken, &g.OmsuGroupID, &g.AnnounceThreadID, &g.IsActive, &g.IsVIP, &createdAt)
	if err != nil {
		return nil, err
	}

	t, err := time.Parse("2006-01-02 15:04:05", createdAt)
	if err == nil {
		g.CreatedAt = t
	} else {
		// Fallback parse if formatting is different
		t, err = time.Parse(time.RFC3339, createdAt)
		if err == nil {
			g.CreatedAt = t
		}
	}
	return &g, nil
}

// GetGroupByToken retrieves a group by API token
func (d *DB) GetGroupByToken(ctx context.Context, token string) (*Group, error) {
	var g Group
	var createdAt string
	err := d.QueryRowContext(ctx,
		`SELECT chat_id, title, api_token, omsu_group_id, announce_thread_id, is_active, is_vip, created_at
		 FROM groups WHERE api_token = ?`,
		token,
	).Scan(&g.ChatID, &g.Title, &g.APIToken, &g.OmsuGroupID, &g.AnnounceThreadID, &g.IsActive, &g.IsVIP, &createdAt)
	if err != nil {
		return nil, err
	}

	t, err := time.Parse("2006-01-02 15:04:05", createdAt)
	if err == nil {
		g.CreatedAt = t
	}
	return &g, nil
}

// UpdateGroup updates group settings
func (d *DB) UpdateGroup(ctx context.Context, g *Group) error {
	_, err := d.ExecContext(ctx,
		`UPDATE groups SET title = ?, api_token = ?, omsu_group_id = ?, announce_thread_id = ?, is_active = ?, is_vip = ?
		 WHERE chat_id = ?`,
		g.Title, g.APIToken, g.OmsuGroupID, g.AnnounceThreadID, g.IsActive, g.IsVIP, g.ChatID,
	)
	if err != nil {
		return fmt.Errorf("failed to update group: %w", err)
	}
	return nil
}

// DeleteGroup removes a group
func (d *DB) DeleteGroup(ctx context.Context, chatID int64) error {
	_, err := d.ExecContext(ctx, "DELETE FROM groups WHERE chat_id = ?", chatID)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}
	return nil
}

// ListGroups returns all groups
func (d *DB) ListGroups(ctx context.Context) ([]Group, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT chat_id, title, api_token, omsu_group_id, announce_thread_id, is_active, is_vip, created_at
		 FROM groups ORDER BY title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		var createdAt string
		err := rows.Scan(&g.ChatID, &g.Title, &g.APIToken, &g.OmsuGroupID, &g.AnnounceThreadID, &g.IsActive, &g.IsVIP, &createdAt)
		if err != nil {
			return nil, err
		}
		t, err := time.Parse("2006-01-02 15:04:05", createdAt)
		if err == nil {
			g.CreatedAt = t
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// IsSuperadmin checks if a user is a superadmin
func (d *DB) IsSuperadmin(ctx context.Context, userID int64) (bool, error) {
	var count int
	err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM superadmins WHERE user_id = ?", userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// AddSuperadmin registers a superadmin
func (d *DB) AddSuperadmin(ctx context.Context, userID int64, note string) error {
	_, err := d.ExecContext(ctx,
		`INSERT OR REPLACE INTO superadmins (user_id, note, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)`,
		userID, note,
	)
	return err
}

// RemoveSuperadmin deletes a superadmin
func (d *DB) RemoveSuperadmin(ctx context.Context, userID int64) error {
	_, err := d.ExecContext(ctx, "DELETE FROM superadmins WHERE user_id = ?", userID)
	return err
}

// IsGroupActive checks if the group exists and is active in the database
func (d *DB) IsGroupActive(ctx context.Context, chatID int64) bool {
	var active int
	err := d.QueryRowContext(ctx, "SELECT is_active FROM groups WHERE chat_id = ?", chatID).Scan(&active)
	if err != nil {
		return false
	}
	return active == 1
}
