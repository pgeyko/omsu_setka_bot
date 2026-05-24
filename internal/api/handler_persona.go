package api

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
)

type personaResponse struct {
	Name         string `json:"name"`
	SystemPrompt string `json:"system_prompt"`
	Signature    string `json:"signature"`
}

type personaUpdateRequest struct {
	Name         *string `json:"name,omitempty"`
	SystemPrompt *string `json:"system_prompt,omitempty"`
	Signature    *string `json:"signature,omitempty"`
}

func (s *Server) handleGetPersona(c *fiber.Ctx) error {
	p := s.Persona.Get()
	return respondSuccess(c, personaResponse{
		Name:         p.Name,
		SystemPrompt: p.SystemPrompt,
		Signature:    p.Signature,
	})
}

func (s *Server) handleUpdatePersona(c *fiber.Ctx) error {
	var req personaUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request body")
	}

	if req.Name != nil && (len(*req.Name) < 1 || len(*req.Name) > 128) {
		return respondError(c, fiber.StatusUnprocessableEntity, ErrValidation, "name must be 1-128 characters")
	}

	current := s.Persona.Get()
	if req.Name != nil {
		current.Name = *req.Name
	}
	if req.SystemPrompt != nil {
		current.SystemPrompt = *req.SystemPrompt
	}
	if req.Signature != nil {
		current.Signature = *req.Signature
	}

	if err := s.Persona.Update(c.Context(), current); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to update persona")
	}

	err := s.Persona.SaveToFile("prompts/persona.md")
	if err != nil {
		slog.Error("failed to save persona to file", "error", err)
	} else {
		s.Persona.Load(c.Context(), "prompts/persona.md")
	}

	return respondSuccess(c, personaResponse{
		Name:         current.Name,
		SystemPrompt: current.SystemPrompt,
		Signature:    current.Signature,
	})
}

func (s *Server) handleResetPersona(c *fiber.Ctx) error {
	if err := s.Persona.Reset(c.Context(), "prompts/persona.md"); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to reset persona")
	}

	p := s.Persona.Get()
	return respondSuccess(c, personaResponse{
		Name:         p.Name,
		SystemPrompt: p.SystemPrompt,
		Signature:    p.Signature,
	})
}
