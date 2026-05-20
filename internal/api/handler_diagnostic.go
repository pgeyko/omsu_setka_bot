package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

type providerStatus struct {
	Name     string `json:"name"`
	Model    string `json:"model"`
	Reachable bool  `json:"reachable"`
	Latency  string `json:"latency,omitempty"`
	Error    string `json:"error,omitempty"`
}

type testModelRequest struct {
	Provider string `json:"provider"`
	Prompt   string `json:"prompt"`
}

type testModelResponse struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Response string `json:"response"`
	Latency  string `json:"latency"`
}

type sendMessageRequest struct {
	Text   string `json:"text"`
	Thread int    `json:"thread_id,omitempty"`
}

func (s *Server) handleCheckProviders(c *fiber.Ctx) error {
	if s.Chain == nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "LLM chain not available")
	}

	statuses := make([]providerStatus, 0)

	for _, provider := range s.Chain.Providers() {
		ps := providerStatus{
			Name:  provider.Name,
			Model: provider.Model,
		}

		start := time.Now()
		client := &http.Client{Timeout: 5 * time.Second}
		baseURL := provider.BaseURL
		if baseURL == "" {
			baseURL = "https://generativelanguage.googleapis.com"
		}

		resp, err := client.Get(baseURL)
		latency := time.Since(start)

		if err != nil {
			ps.Reachable = false
			ps.Error = err.Error()
		} else {
			resp.Body.Close()
			ps.Reachable = true
			ps.Latency = latency.Round(time.Millisecond).String()
		}

		statuses = append(statuses, ps)
	}

	return respondSuccess(c, statuses)
}

func (s *Server) handleTestModel(c *fiber.Ctx) error {
	if s.LLMClient == nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "LLM client not available")
	}

	var req testModelRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request")
	}
	if req.Prompt == "" {
		req.Prompt = "Ответь одним словом: ты работаешь?"
	}

	start := time.Now()
	resp, err := s.LLMClient.Call(c.Context(), "diagnostic", "", req.Prompt, false)
	latency := time.Since(start)

	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, fmt.Sprintf("LLM call failed: %v", err))
	}

	return respondSuccess(c, testModelResponse{
		Provider: resp.Provider,
		Model:    resp.Model,
		Response: resp.Content,
		Latency:  latency.Round(time.Millisecond).String(),
	})
}

func (s *Server) handleSendMessage(c *fiber.Ctx) error {
	if s.TelegramBot == nil {
		return respondError(c, fiber.StatusServiceUnavailable, ErrInternal, "Telegram bot not available (not connected or invalid token)")
	}

	var req sendMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request")
	}
	if req.Text == "" {
		return respondError(c, fiber.StatusUnprocessableEntity, ErrValidation, "text is required")
	}

	if err := s.TelegramBot.Send(c.Context(), s.TelegramGroupID, req.Text); err != nil {
		slog.Error("failed to send message via bot", "error", err)
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, fmt.Sprintf("failed to send: %v", err))
	}

	return respondSuccess(c, fiber.Map{"sent": true})
}
