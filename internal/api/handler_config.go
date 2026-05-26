package api

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"omsu_bot/internal/db"
)

type configResponse struct {
	SkipFallbackModel           bool `json:"skip_fallback_model"`
	GlobalVoiceTranscription    bool `json:"global_voice_transcription"`
	GlobalPhotoProcessing       bool `json:"global_photo_processing"`
	GroupRegistrationRestricted bool `json:"group_registration_restricted"`
}

// LoadConfigFromDB reads runtime config from bot_config table and syncs Server fields.
// Missing keys fall back to the current in-memory value (set from env at startup).
// Call this once after NewServer and Migrate to restore persisted settings.
func (s *Server) LoadConfigFromDB(ctx context.Context) {
	d := &db.DB{DB: s.DB}
	if val, err := d.GetConfig(ctx, "skip_fallback_model", strconv.FormatBool(s.Config.SkipFallbackModel)); err == nil {
		s.Config.SkipFallbackModel, _ = strconv.ParseBool(val)
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
	if val, err := d.GetConfig(ctx, "group_registration_restricted", strconv.FormatBool(s.GroupRegistrationRestricted)); err == nil {
		s.GroupRegistrationRestricted, _ = strconv.ParseBool(val)
	}
	if s.LLMClient != nil {
		s.LLMClient.SetSkipFallbackModel(s.Config.SkipFallbackModel)
	}
}

func (s *Server) handleGetConfig(c *fiber.Ctx) error {
	return respondSuccess(c, configResponse{
		SkipFallbackModel:           s.Config.SkipFallbackModel,
		GlobalVoiceTranscription:    s.GlobalVoiceTranscription.Load(),
		GlobalPhotoProcessing:       s.GlobalPhotoProcessing.Load(),
		GroupRegistrationRestricted: s.GroupRegistrationRestricted,
	})
}

func (s *Server) handleUpdateConfig(c *fiber.Ctx) error {
	var req configResponse
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request")
	}

	s.Config.SkipFallbackModel = req.SkipFallbackModel
	s.GlobalVoiceTranscription.Store(req.GlobalVoiceTranscription)
	s.GlobalPhotoProcessing.Store(req.GlobalPhotoProcessing)
	s.GroupRegistrationRestricted = req.GroupRegistrationRestricted

	if s.LLMClient != nil {
		s.LLMClient.SetSkipFallbackModel(req.SkipFallbackModel)
	}

	// Persist to DB so config survives restarts.
	d := &db.DB{DB: s.DB}
	if err := d.SetConfig(c.Context(), "skip_fallback_model", strconv.FormatBool(req.SkipFallbackModel)); err != nil {
		slog.Warn("persist skip_fallback_model", "error", err)
	}
	if err := d.SetConfig(c.Context(), "global_voice_transcription", strconv.FormatBool(req.GlobalVoiceTranscription)); err != nil {
		slog.Warn("persist global_voice_transcription", "error", err)
	}
	if err := d.SetConfig(c.Context(), "global_photo_processing", strconv.FormatBool(req.GlobalPhotoProcessing)); err != nil {
		slog.Warn("persist global_photo_processing", "error", err)
	}
	if err := d.SetConfig(c.Context(), "group_registration_restricted", strconv.FormatBool(req.GroupRegistrationRestricted)); err != nil {
		slog.Warn("persist group_registration_restricted", "error", err)
	}

	return respondSuccess(c, configResponse{
		SkipFallbackModel:           s.Config.SkipFallbackModel,
		GlobalVoiceTranscription:    s.GlobalVoiceTranscription.Load(),
		GlobalPhotoProcessing:       s.GlobalPhotoProcessing.Load(),
		GroupRegistrationRestricted: s.GroupRegistrationRestricted,
	})
}
