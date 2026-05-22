package api

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"omsu_bot/internal/db"
	"omsu_bot/internal/telegram"
)

type superadminAddRequest struct {
	UserID int64  `json:"user_id"`
	Note   string `json:"note"`
}

func (s *Server) handleListSuperadmins(c *fiber.Ctx) error {
	d := &db.DB{DB: s.DB}
	admins, err := d.ListSuperadmins(c.Context())
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to list superadmins: "+err.Error())
	}
	if admins == nil {
		admins = []db.Superadmin{}
	}
	return respondSuccess(c, admins)
}

func (s *Server) handleAddSuperadmin(c *fiber.Ctx) error {
	var req superadminAddRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request body")
	}

	if req.UserID == 0 {
		return respondError(c, fiber.StatusUnprocessableEntity, ErrValidation, "user_id is required")
	}

	d := &db.DB{DB: s.DB}
	if err := d.AddSuperadmin(c.Context(), req.UserID, req.Note); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to add superadmin: "+err.Error())
	}

	return respondSuccess(c, fiber.Map{"status": "ok", "user_id": req.UserID})
}

func (s *Server) handleRemoveSuperadmin(c *fiber.Ctx) error {
	userIDStr := c.Params("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid user_id")
	}

	d := &db.DB{DB: s.DB}
	if err := d.RemoveSuperadmin(c.Context(), userID); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to remove superadmin: "+err.Error())
	}

	return respondSuccess(c, fiber.Map{"status": "ok", "user_id": userID})
}

func (s *Server) handleRegisterWebhooks(c *fiber.Ctx) error {
	groupIDs, err := telegram.RegisterWebhooksWithSetka(
		c.Context(),
		s.DB,
		s.SetkaBaseURL,
		s.SetkaAdminKey,
		s.WebhookSecret,
		s.SetkaPublicURL,
	)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to register webhooks with Setka: "+err.Error())
	}

	if len(groupIDs) == 0 {
		return respondSuccess(c, fiber.Map{"status": "no_active_groups_to_register"})
	}

	return respondSuccess(c, fiber.Map{"status": "ok", "registered_groups": groupIDs})
}
