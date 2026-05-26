package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"omsu_bot/internal/db"
)

// maxContextSize is the maximum allowed size of a per-group context file (512 KB).
const maxContextSize = 512 * 1024

type contextUploadRequest struct {
	Content string `json:"content"`
}

func (s *Server) handleUploadPersona(c *fiber.Ctx) error {
	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid chat_id")
	}

	d := &db.DB{DB: s.DB}
	exists, err := d.GroupExists(c.Context(), chatID)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "db error")
	}
	if !exists {
		return respondError(c, fiber.StatusNotFound, ErrNotFound, "group not found")
	}

	var req contextUploadRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request body")
	}
	if len(req.Content) > maxContextSize {
		return respondError(c, fiber.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "persona file must be ≤512 KB")
	}

	dir := fmt.Sprintf("data/groups/%d", chatID)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to create directory")
	}

	filePath := filepath.Join(dir, "persona.md")
	if err := os.WriteFile(filePath, []byte(req.Content), 0640); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to write persona file")
	}

	// Reload the global store if we just modified group context, because it will be dynamically merged with the system prompt or we can let GetGroupSystemPrompt read it live. It reads it live.
	return respondSuccess(c, fiber.Map{"status": "ok", "file": "persona.md"})
}

func (s *Server) handleUploadSystemPrompt(c *fiber.Ctx) error {
	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid chat_id")
	}

	d := &db.DB{DB: s.DB}
	exists, err := d.GroupExists(c.Context(), chatID)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "db error")
	}
	if !exists {
		return respondError(c, fiber.StatusNotFound, ErrNotFound, "group not found")
	}

	var req contextUploadRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request body")
	}
	if len(req.Content) > maxContextSize {
		return respondError(c, fiber.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "system prompt must be ≤512 KB")
	}

	dir := fmt.Sprintf("data/groups/%d", chatID)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to create directory")
	}

	filePath := filepath.Join(dir, "system_prompt.md")
	if err := os.WriteFile(filePath, []byte(req.Content), 0640); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to write system_prompt file")
	}

	// Also re-read for dynamic prompts

	return respondSuccess(c, fiber.Map{"status": "ok", "file": "system_prompt.md"})
}

func (s *Server) handleUploadKnowledge(c *fiber.Ctx) error {
	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid chat_id")
	}

	d := &db.DB{DB: s.DB}
	exists, err := d.GroupExists(c.Context(), chatID)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "db error")
	}
	if !exists {
		return respondError(c, fiber.StatusNotFound, ErrNotFound, "group not found")
	}

	var req contextUploadRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request body")
	}
	if len(req.Content) > maxContextSize {
		return respondError(c, fiber.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "knowledge base must be ≤512 KB")
	}

	dir := fmt.Sprintf("data/groups/%d", chatID)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to create directory")
	}

	filePath := filepath.Join(dir, "knowledge_base.md")
	if err := os.WriteFile(filePath, []byte(req.Content), 0640); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to write knowledge_base file")
	}

	return respondSuccess(c, fiber.Map{"status": "ok", "file": "knowledge_base.md"})
}

func (s *Server) handleGetGroupPersona(c *fiber.Ctx) error {
	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid chat_id")
	}

	filePath := fmt.Sprintf("data/groups/%d/persona.md", chatID)
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return respondSuccess(c, fiber.Map{"content": ""})
		}
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to read file")
	}

	return respondSuccess(c, fiber.Map{"content": string(content)})
}

func (s *Server) handleGetGroupSystemPrompt(c *fiber.Ctx) error {
	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid chat_id")
	}

	filePath := fmt.Sprintf("data/groups/%d/system_prompt.md", chatID)
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return respondSuccess(c, fiber.Map{"content": ""})
		}
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to read file")
	}

	return respondSuccess(c, fiber.Map{"content": string(content)})
}

func (s *Server) handleGetGroupKnowledge(c *fiber.Ctx) error {
	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid chat_id")
	}

	filePath := fmt.Sprintf("data/groups/%d/knowledge_base.md", chatID)
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return respondSuccess(c, fiber.Map{"content": ""})
		}
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to read file")
	}

	return respondSuccess(c, fiber.Map{"content": string(content)})
}

func (s *Server) handleUploadFeatures(c *fiber.Ctx) error {
	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid chat_id")
	}

	var req map[string]bool
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request body")
	}

	dir := fmt.Sprintf("data/groups/%d", chatID)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to create directory")
	}

	bytes, err := json.Marshal(req)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to serialize features")
	}

	filePath := filepath.Join(dir, "features.json")
	if err := os.WriteFile(filePath, bytes, 0640); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to write features file")
	}

	return respondSuccess(c, fiber.Map{"status": "ok", "file": "features.json"})
}

func defaultFeatures() map[string]bool {
	return map[string]bool{
		"enable_schedule":            true,
		"enable_summary":             true,
		"enable_moderation":          true,
		"enable_captcha":             true,
		"enable_link_filter":         true,
		"enable_flood_control":        true,
		"enable_voice_transcription": true,
		"enable_photo_processing":    true,
		"photo_on_mention":          false,
	}
}

func (s *Server) handleGetGroupFeatures(c *fiber.Ctx) error {
	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid chat_id")
	}

	filePath := fmt.Sprintf("data/groups/%d/features.json", chatID)
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return respondSuccess(c, fiber.Map{"features": defaultFeatures()})
		}
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to read file")
	}

	var features map[string]bool
	if err := json.Unmarshal(content, &features); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to parse features")
	}

	// fill defaults for missing features
	defaults := defaultFeatures()
	for k, v := range defaults {
		if _, exists := features[k]; !exists {
			features[k] = v
		}
	}

	return respondSuccess(c, fiber.Map{"features": features})
}
