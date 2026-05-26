package api

import (
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type topicResponse struct {
	ID          int      `json:"id"`
	GroupID     int64    `json:"group_id"`
	TgThreadID  int64    `json:"tg_thread_id"`
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Aliases     []string `json:"aliases"`
	Description string   `json:"description"`
	Hashtags    []string `json:"hashtags"`
	IsActive    bool     `json:"is_active"`
	CreatedAt   string   `json:"created_at"`
}

type topicCreateRequest struct {
	GroupID     int64    `json:"group_id"`
	TgThreadID  int64    `json:"tg_thread_id"`
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Aliases     []string `json:"aliases,omitempty"`
	Description string   `json:"description,omitempty"`
	Hashtags    []string `json:"hashtags,omitempty"`
}

type topicUpdateRequest struct {
	Name        *string   `json:"name,omitempty"`
	Slug        *string   `json:"slug,omitempty"`
	Aliases     *[]string `json:"aliases,omitempty"`
	Description *string   `json:"description,omitempty"`
	Hashtags    *[]string `json:"hashtags,omitempty"`
}

func scanTopic(scanner interface {
	Scan(dest ...interface{}) error
}) (topicResponse, error) {
	var t topicResponse
	var id int
	var groupID int64
	var tgThreadID int64
	var name, slug, aliasesJSON, description, hashtagsJSON, createdAt string
	var isActive int

	err := scanner.Scan(&id, &groupID, &tgThreadID, &name, &slug, &aliasesJSON, &description, &hashtagsJSON, &isActive, &createdAt)
	if err != nil {
		return t, err
	}

	t.ID = id
	t.GroupID = groupID
	t.TgThreadID = tgThreadID
	t.Name = name
	t.Slug = slug
	t.Description = description
	t.IsActive = isActive == 1
	t.CreatedAt = createdAt

	json.Unmarshal([]byte(aliasesJSON), &t.Aliases)
	json.Unmarshal([]byte(hashtagsJSON), &t.Hashtags)

	if t.Aliases == nil {
		t.Aliases = []string{}
	}
	if t.Hashtags == nil {
		t.Hashtags = []string{}
	}

	return t, nil
}

func (s *Server) handleGetTopics(c *fiber.Ctx) error {
	groupIDStr := c.Query("group_id")
	var groupID int64
	if groupIDStr != "" {
		groupID, _ = strconv.ParseInt(groupIDStr, 10, 64)
	} else {
		groupID = s.Config.TelegramGroupID
	}

	limit, offset := parsePagination(c)

	var total int
	s.DB.QueryRowContext(c.Context(), `SELECT COUNT(*) FROM topics WHERE group_id = ?`, groupID).Scan(&total)

	rows, err := s.DB.QueryContext(c.Context(),
		`SELECT id, group_id, tg_thread_id, name, slug, aliases, description, hashtags, is_active, created_at
		 FROM topics WHERE group_id = ? ORDER BY created_at ASC LIMIT ? OFFSET ?`, groupID, limit, offset)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to fetch topics")
	}
	defer rows.Close()

	topics := make([]topicResponse, 0)
	for rows.Next() {
		t, err := scanTopic(rows)
		if err != nil {
			continue
		}
		topics = append(topics, t)
	}

	return respondPaginated(c, topics, total, limit, offset)
}

func (s *Server) handleGetTopic(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid id")
	}

	row := s.DB.QueryRowContext(c.Context(),
		`SELECT id, group_id, tg_thread_id, name, slug, aliases, description, hashtags, is_active, created_at
		 FROM topics WHERE id = ?`, id)

	t, err := scanTopic(row)
	if err != nil {
		return respondError(c, fiber.StatusNotFound, ErrNotFound, "topic not found")
	}

	return respondSuccess(c, t)
}

func (s *Server) handleCreateTopic(c *fiber.Ctx) error {
	var req topicCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request body")
	}

	if req.Name == "" || len(req.Name) > 128 {
		return respondError(c, fiber.StatusUnprocessableEntity, ErrValidation, "name is required (max 128)")
	}
	if req.Slug == "" || len(req.Slug) > 64 {
		return respondError(c, fiber.StatusUnprocessableEntity, ErrValidation, "slug is required (max 64)")
	}

	groupID := req.GroupID
	if groupID == 0 {
		groupIDStr := c.Query("group_id")
		if groupIDStr != "" {
			groupID, _ = strconv.ParseInt(groupIDStr, 10, 64)
		} else {
			groupID = s.Config.TelegramGroupID
		}
	}

	aliasesJSON, _ := json.Marshal(req.Aliases)
	hashtagsJSON, _ := json.Marshal(req.Hashtags)

	result, err := s.DB.ExecContext(c.Context(),
		`INSERT INTO topics (group_id, tg_thread_id, name, slug, aliases, description, hashtags, is_active, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP)`,
		groupID, req.TgThreadID, req.Name, req.Slug, string(aliasesJSON), req.Description, string(hashtagsJSON))
	if err != nil {
		return respondError(c, fiber.StatusConflict, ErrConflict, "slug or tg_thread_id already exists")
	}

	id, _ := result.LastInsertId()
	return respondSuccess(c, fiber.Map{"id": id})
}

func (s *Server) handleUpdateTopic(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid id")
	}

	var req topicUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request body")
	}

	query := "UPDATE topics SET "
	var args []interface{}
	var sets []string

	if req.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *req.Name)
	}
	if req.Slug != nil {
		sets = append(sets, "slug = ?")
		args = append(args, *req.Slug)
	}
	if req.Description != nil {
		sets = append(sets, "description = ?")
		args = append(args, *req.Description)
	}
	if req.Aliases != nil {
		sets = append(sets, "aliases = ?")
		j, _ := json.Marshal(*req.Aliases)
		args = append(args, string(j))
	}
	if req.Hashtags != nil {
		sets = append(sets, "hashtags = ?")
		j, _ := json.Marshal(*req.Hashtags)
		args = append(args, string(j))
	}

	if len(sets) == 0 {
		return respondError(c, fiber.StatusUnprocessableEntity, ErrValidation, "no fields to update")
	}

	query += joinStrings(sets, ", ") + " WHERE id = ?"
	args = append(args, id)

	result, err := s.DB.ExecContext(c.Context(), query, args...)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to update topic")
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return respondError(c, fiber.StatusNotFound, ErrNotFound, "topic not found")
	}

	return respondSuccess(c, fiber.Map{"id": id})
}

func (s *Server) handleDeleteTopic(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid id")
	}

	result, err := s.DB.ExecContext(c.Context(), `DELETE FROM topics WHERE id = ?`, id)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to delete topic")
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return respondError(c, fiber.StatusNotFound, ErrNotFound, "topic not found")
	}

	return respondSuccess(c, fiber.Map{"deleted_id": id})
}

func (s *Server) handleCloseTopic(c *fiber.Ctx) error {
	return s.toggleTopicActive(c, 0)
}

func (s *Server) handleOpenTopic(c *fiber.Ctx) error {
	return s.toggleTopicActive(c, 1)
}

func (s *Server) toggleTopicActive(c *fiber.Ctx, active int) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid id")
	}

	result, err := s.DB.ExecContext(c.Context(),
		`UPDATE topics SET is_active = ? WHERE id = ?`, active, id)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to update topic")
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return respondError(c, fiber.StatusNotFound, ErrNotFound, "topic not found")
	}

	return respondSuccess(c, fiber.Map{"id": id, "is_active": active == 1})
}

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
