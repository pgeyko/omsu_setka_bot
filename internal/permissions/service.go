package permissions

import (
	"context"
	"database/sql"
)

// Service checks tool/command permissions against the DB.
// Maps to the command_permissions table: (group_id, command) -> allowed_role.
type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// AllowedRole returns the role required for a command in a given group.
// Returns "admin" if the command is in allowed_roles with "admin" role,
// "everyone" if allowed for everyone, or "" if the command is not configured.
func (s *Service) AllowedRole(ctx context.Context, groupID int64, command string) string {
	var role string
	err := s.db.QueryRowContext(ctx,
		`SELECT allowed_role FROM command_permissions WHERE group_id = ? AND command = ?`,
		groupID, command,
	).Scan(&role)
	if err != nil {
		return ""
	}
	return role
}

// IsAdminOnly returns true when the command is restricted to admins for this group.
func (s *Service) IsAdminOnly(ctx context.Context, groupID int64, command string) bool {
	return s.AllowedRole(ctx, groupID, command) == "admin"
}
