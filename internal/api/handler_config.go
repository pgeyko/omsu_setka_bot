package api

import "github.com/gofiber/fiber/v2"

type configResponse struct {
	SkipFallbackModel bool `json:"skip_fallback_model"`
}

func (s *Server) handleGetConfig(c *fiber.Ctx) error {
	return respondSuccess(c, configResponse{
		SkipFallbackModel: s.SkipFallbackModel,
	})
}

func (s *Server) handleUpdateConfig(c *fiber.Ctx) error {
	var req configResponse
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request")
	}

	s.SkipFallbackModel = req.SkipFallbackModel
	if s.LLMClient != nil {
		s.LLMClient.SetSkipFallbackModel(req.SkipFallbackModel)
	}

	return respondSuccess(c, configResponse{
		SkipFallbackModel: s.SkipFallbackModel,
	})
}
