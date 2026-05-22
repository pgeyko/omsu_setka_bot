package api

import (
	"database/sql"
	"errors"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"omsu_bot/internal/db"
)

type groupRequest struct {
	ChatID      int64  `json:"chat_id"`
	Title       string `json:"title"`
	APIToken    string `json:"api_token"`
	OmsuGroupID int    `json:"omsu_group_id"`
	IsActive    bool   `json:"is_active"`
	IsVIP       bool   `json:"is_vip"`
}

func (s *Server) handleListGroups(c *fiber.Ctx) error {
	d := &db.DB{DB: s.DB}
	groups, err := d.ListGroups(c.Context())
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to list groups")
	}
	// Strip api_token from public response.
	public := make([]db.GroupPublic, len(groups))
	for i := range groups {
		public[i] = groups[i].ToPublic()
	}
	return respondSuccess(c, public)
}

func (s *Server) handleCreateGroup(c *fiber.Ctx) error {
	var req groupRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request body")
	}

	if req.ChatID == 0 || req.Title == "" {
		return respondError(c, fiber.StatusUnprocessableEntity, ErrValidation, "chat_id and title are required")
	}

	g := &db.Group{
		ChatID:      req.ChatID,
		Title:       req.Title,
		APIToken:    req.APIToken,
		OmsuGroupID: req.OmsuGroupID,
		IsActive:    req.IsActive,
		IsVIP:       req.IsVIP,
	}

	d := &db.DB{DB: s.DB}
	if err := d.CreateGroup(c.Context(), g); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, err.Error())
	}

	publicGrp := g.ToPublic()

	slog.Info("handleGetGroup returning structure", "chat_id", publicGrp.ChatID, "omsu_group_id", publicGrp.OmsuGroupID, "title", publicGrp.Title)

	return respondSuccess(c, publicGrp)
}

func (s *Server) handleGetGroup(c *fiber.Ctx) error {
	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid chat_id")
	}

	d := &db.DB{DB: s.DB}
	g, err := d.GetGroup(c.Context(), chatID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return respondError(c, fiber.StatusNotFound, ErrNotFound, "group not found")
		}
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, err.Error())
	}

	slog.Info("handleGetGroup returning structure", "chat_id", g.ChatID, "omsu_group_id", g.OmsuGroupID, "title", g.Title)

	return respondSuccess(c, g.ToPublic())
}

func (s *Server) handleUpdateGroup(c *fiber.Ctx) error {
	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid chat_id")
	}

	var req groupRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request body")
	}

	d := &db.DB{DB: s.DB}
	g, err := d.GetGroup(c.Context(), chatID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return respondError(c, fiber.StatusNotFound, ErrNotFound, "group not found")
		}
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, err.Error())
	}

	if req.Title != "" {
		g.Title = req.Title
	}
	if req.APIToken != "" {
		g.APIToken = req.APIToken
	}
	g.OmsuGroupID = req.OmsuGroupID
	g.IsActive = req.IsActive
	g.IsVIP = req.IsVIP

	if err := d.UpdateGroup(c.Context(), g); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, err.Error())
	}

	return respondSuccess(c, g.ToPublic())
}

func (s *Server) handleDeleteGroup(c *fiber.Ctx) error {
	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid chat_id")
	}

	d := &db.DB{DB: s.DB}
	if err := d.SoftDeleteGroup(c.Context(), chatID); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, err.Error())
	}

	return respondSuccess(c, fiber.Map{"status": "deactivated"})
}
