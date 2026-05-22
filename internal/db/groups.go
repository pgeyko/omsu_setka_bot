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

// GroupPublic is a safe view of Group that omits the api_token secret.
type GroupPublic struct {
	ChatID           int64     `json:"chat_id"`
	Title            string    `json:"title"`
	OmsuGroupID      int       `json:"omsu_group_id"`
	AnnounceThreadID int       `json:"announce_thread_id"`
	IsActive         bool      `json:"is_active"`
	IsVIP            bool      `json:"is_vip"`
	CreatedAt        time.Time `json:"created_at"`
}

// ToPublic converts a Group to its public (token-stripped) representation.
func (g *Group) ToPublic() GroupPublic {
	return GroupPublic{
		ChatID:           g.ChatID,
		Title:            g.Title,
		OmsuGroupID:      g.OmsuGroupID,
		AnnounceThreadID: g.AnnounceThreadID,
		IsActive:         g.IsActive,
		IsVIP:            g.IsVIP,
		CreatedAt:        g.CreatedAt,
	}
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

// DeleteGroup removes a group (hard delete — use SoftDeleteGroup for safe removal).
func (d *DB) DeleteGroup(ctx context.Context, chatID int64) error {
	_, err := d.ExecContext(ctx, "DELETE FROM groups WHERE chat_id = ?", chatID)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}
	return nil
}

// SoftDeleteGroup marks a group as deleted without removing it from the database.
// Related data (topics, processed_messages) is preserved for auditing.
func (d *DB) SoftDeleteGroup(ctx context.Context, chatID int64) error {
	_, err := d.ExecContext(ctx,
		"UPDATE groups SET is_active = 0, title = title || ' [DELETED]' WHERE chat_id = ?",
		chatID)
	if err != nil {
		return fmt.Errorf("failed to soft-delete group: %w", err)
	}
	return nil
}

// GroupExists returns true when a group with the given chat_id exists.
func (d *DB) GroupExists(ctx context.Context, chatID int64) (bool, error) {
	var count int
	err := d.QueryRowContext(ctx, "SELECT COUNT(*) FROM groups WHERE chat_id = ?", chatID).Scan(&count)
	return count > 0, err
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

// ListSuperadmins returns all superadmins
func (d *DB) ListSuperadmins(ctx context.Context) ([]Superadmin, error) {
	rows, err := d.QueryContext(ctx, "SELECT user_id, note, created_at FROM superadmins ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var admins []Superadmin
	for rows.Next() {
		var sa Superadmin
		var createdAt string
		if err := rows.Scan(&sa.UserID, &sa.Note, &createdAt); err != nil {
			return nil, err
		}
		t, err := time.Parse("2006-01-02 15:04:05", createdAt)
		if err == nil {
			sa.CreatedAt = t
		} else {
			t, err = time.Parse(time.RFC3339, createdAt)
			if err == nil {
				sa.CreatedAt = t
			}
		}
		admins = append(admins, sa)
	}
	return admins, nil
}

// UpdateGroupOmsuID updates only the Setka group ID for a chat
func (d *DB) UpdateGroupOmsuID(ctx context.Context, chatID int64, omsuGroupID int) error {
	_, err := d.ExecContext(ctx, "UPDATE groups SET omsu_group_id = ? WHERE chat_id = ?", omsuGroupID, chatID)
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

