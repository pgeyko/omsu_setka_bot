package api

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"omsu_bot/internal/db"
)

type configResponse struct {
	SkipFallbackModel        bool `json:"skip_fallback_model"`
	GlobalVoiceTranscription bool `json:"global_voice_transcription"`
	GlobalPhotoProcessing    bool `json:"global_photo_processing"`
}

// LoadConfigFromDB reads runtime config from bot_config table and syncs Server fields.
// Missing keys fall back to the current in-memory value (set from env at startup).
// Call this once after NewServer and Migrate to restore persisted settings.
func (s *Server) LoadConfigFromDB(ctx context.Context) {
	d := &db.DB{DB: s.DB}
	if val, err := d.GetConfig(ctx, "skip_fallback_model", strconv.FormatBool(s.SkipFallbackModel)); err == nil {
		s.SkipFallbackModel, _ = strconv.ParseBool(val)
	}
	if val, err := d.GetConfig(ctx, "global_voice_transcription", strconv.FormatBool(s.GlobalVoiceTranscription.Load())); err == nil {
		if parsed, err := strconv.ParseBool(val); err == nil {
			s.GlobalVoiceTranscription.Store(parsed)
		}
	}
	if val, err := d.GetConfig(ctx, "global_photo_processing", strconv.FormatBool(s.GlobalPhotoProcessing.Load())); err == nil {
		if parsed, err := strconv.ParseBool(val); err == nil {
			s.GlobalPhotoProcessing.Store(parsed)
		}
	}
	if s.LLMClient != nil {
		s.LLMClient.SetSkipFallbackModel(s.SkipFallbackModel)
	}
}

func (s *Server) handleGetConfig(c *fiber.Ctx) error {
	return respondSuccess(c, configResponse{
		SkipFallbackModel:        s.SkipFallbackModel,
		GlobalVoiceTranscription: s.GlobalVoiceTranscription.Load(),
		GlobalPhotoProcessing:    s.GlobalPhotoProcessing.Load(),
	})
}

func (s *Server) handleUpdateConfig(c *fiber.Ctx) error {
	var req configResponse
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request")
	}

	s.SkipFallbackModel = req.SkipFallbackModel
	s.GlobalVoiceTranscription.Store(req.GlobalVoiceTranscription)
	s.GlobalPhotoProcessing.Store(req.GlobalPhotoProcessing)

	if s.LLMClient != nil {
		s.LLMClient.SetSkipFallbackModel(req.SkipFallbackModel)
	}

	// Persist to DB so config survives restarts.
	d := &db.DB{DB: s.DB}
	_ = d.SetConfig(c.Context(), "skip_fallback_model", strconv.FormatBool(req.SkipFallbackModel))
	_ = d.SetConfig(c.Context(), "global_voice_transcription", strconv.FormatBool(req.GlobalVoiceTranscription))
	_ = d.SetConfig(c.Context(), "global_photo_processing", strconv.FormatBool(req.GlobalPhotoProcessing))

	return respondSuccess(c, configResponse{
		SkipFallbackModel:        s.SkipFallbackModel,
		GlobalVoiceTranscription: s.GlobalVoiceTranscription.Load(),
		GlobalPhotoProcessing:    s.GlobalPhotoProcessing.Load(),
	})
}
