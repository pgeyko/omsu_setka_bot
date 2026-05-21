package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type contextUploadRequest struct {
	Content string `json:"content"`
}

func (s *Server) handleUploadPersona(c *fiber.Ctx) error {
	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid chat_id")
	}

	var req contextUploadRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request body")
	}

	dir := fmt.Sprintf("data/groups/%d", chatID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to create directory")
	}

	filePath := filepath.Join(dir, "persona.md")
	if err := os.WriteFile(filePath, []byte(req.Content), 0644); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to write persona file")
	}

	return respondSuccess(c, fiber.Map{"status": "ok", "file": "persona.md"})
}

func (s *Server) handleUploadSystemPrompt(c *fiber.Ctx) error {
	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid chat_id")
	}

	var req contextUploadRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request body")
	}

	dir := fmt.Sprintf("data/groups/%d", chatID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to create directory")
	}

	filePath := filepath.Join(dir, "system_prompt.txt")
	if err := os.WriteFile(filePath, []byte(req.Content), 0644); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to write system_prompt file")
	}

	return respondSuccess(c, fiber.Map{"status": "ok", "file": "system_prompt.txt"})
}

func (s *Server) handleUploadKnowledge(c *fiber.Ctx) error {
	chatIDStr := c.Params("chat_id")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid chat_id")
	}

	var req contextUploadRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request body")
	}

	dir := fmt.Sprintf("data/groups/%d", chatID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to create directory")
	}

	filePath := filepath.Join(dir, "knowledge_base.txt")
	if err := os.WriteFile(filePath, []byte(req.Content), 0644); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to write knowledge_base file")
	}

	return respondSuccess(c, fiber.Map{"status": "ok", "file": "knowledge_base.txt"})
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

	filePath := fmt.Sprintf("data/groups/%d/system_prompt.txt", chatID)
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

	filePath := fmt.Sprintf("data/groups/%d/knowledge_base.txt", chatID)
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
	if err := os.MkdirAll(dir, 0755); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to create directory")
	}

	bytes, err := json.Marshal(req)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to serialize features")
	}

	filePath := filepath.Join(dir, "features.json")
	if err := os.WriteFile(filePath, bytes, 0644); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to write features file")
	}

	return respondSuccess(c, fiber.Map{"status": "ok", "file": "features.json"})
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
			return respondSuccess(c, fiber.Map{"features": fiber.Map{}})
		}
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to read file")
	}

	var features map[string]bool
	if err := json.Unmarshal(content, &features); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to parse features")
	}

	return respondSuccess(c, fiber.Map{"features": features})
}
