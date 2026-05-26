package api

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type permissionResponse struct {
	Command     string `json:"command"`
	AllowedRole string `json:"allowed_role"`
}

type updatePermissionRequest struct {
	AllowedRole string `json:"allowed_role"`
}

func (s *Server) handleGetPermissions(c *fiber.Ctx) error {
	groupIDStr := c.Query("group_id")
	var groupID int64
	if groupIDStr != "" {
		groupID, _ = strconv.ParseInt(groupIDStr, 10, 64)
	} else {
		groupID = s.Config.TelegramGroupID
	}

	// Seed defaults: ensure every known command has a row regardless of FK
	defaultCommands := []struct {
		Command     string
		AllowedRole string
	}{
		{"forward", "everyone"},
		{"moderate_user", "admin"},
		{"run_protocol", "admin"},
		{"manage_topic", "admin"},
		{"get_schedule", "everyone"},
		{"generate_summary", "everyone"},
	}
	for _, dc := range defaultCommands {
		s.DB.ExecContext(c.Context(),
			`INSERT OR IGNORE INTO command_permissions (group_id, command, allowed_role) VALUES (?, ?, ?)`,
			groupID, dc.Command, dc.AllowedRole)
	}

	rows, err := s.DB.QueryContext(c.Context(),
		`SELECT command, allowed_role FROM command_permissions WHERE group_id = ? ORDER BY command`, groupID)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to query permissions")
	}
	defer rows.Close()

	perms := make([]permissionResponse, 0)
	for rows.Next() {
		var p permissionResponse
		if err := rows.Scan(&p.Command, &p.AllowedRole); err != nil {
			continue
		}
		perms = append(perms, p)
	}

	return respondSuccess(c, perms)
}

func (s *Server) handleUpdatePermission(c *fiber.Ctx) error {
	command := c.Params("command")
	if command == "" {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "command is required")
	}

	groupIDStr := c.Query("group_id")
	var groupID int64
	if groupIDStr != "" {
		groupID, _ = strconv.ParseInt(groupIDStr, 10, 64)
	} else {
		groupID = s.Config.TelegramGroupID
	}

	var req updatePermissionRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request body")
	}

	if req.AllowedRole != "everyone" && req.AllowedRole != "admin" {
		return respondError(c, fiber.StatusUnprocessableEntity, ErrValidation, "allowed_role must be 'everyone' or 'admin'")
	}

	_, err := s.DB.ExecContext(c.Context(),
		`INSERT INTO command_permissions (group_id, command, allowed_role)
		 VALUES (?, ?, ?)
		 ON CONFLICT(group_id, command) DO UPDATE SET allowed_role = excluded.allowed_role`,
		groupID, command, req.AllowedRole)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to update permission")
	}

	return respondSuccess(c, fiber.Map{"command": command, "allowed_role": req.AllowedRole})
}
