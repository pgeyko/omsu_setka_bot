package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type providerStatus struct {
	Name      string `json:"name"`
	Model     string `json:"model"`
	Reachable bool   `json:"reachable"`
	Latency   string `json:"latency,omitempty"`
	Error     string `json:"error,omitempty"`
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
	ChatID int64  `json:"chat_id,omitempty"`
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
		if s.Prompts != nil {
			req.Prompt = s.Prompts.Get("diagnostic_test")
		}
		if req.Prompt == "" {
			req.Prompt = "Ответь одним словом: ты работаешь?"
		}
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

func (s *Server) handleTestAllModels(c *fiber.Ctx) error {
	if s.Chain == nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "LLM chain not available")
	}

	type testResult struct {
		Success  bool   `json:"success"`
		Latency  string `json:"latency,omitempty"`
		Response string `json:"response,omitempty"`
		Error    string `json:"error,omitempty"`
	}

	type modelResult struct {
		Name     string      `json:"name"`
		Type     string      `json:"type"`
		Model    string      `json:"model"`
		Priority int         `json:"priority"`
		Active   bool        `json:"active"`
		Test     *testResult `json:"test,omitempty"`
	}

	byType := make(map[string][]modelResult)
	total, ok := 0, 0

	testPrompt := "Ответь одним словом: ты работаешь?"

	for _, provider := range s.Chain.Providers() {
		total++
		active := provider.IsActive()
		result := modelResult{
			Name:     provider.Name,
			Type:     provider.Type,
			Model:    provider.Model,
			Priority: provider.Priority,
			Active:   active,
		}

		if strings.HasPrefix(provider.Model, "whisper") {
			result.Test = &testResult{Success: true}
			ok++
			byType[provider.Type] = append(byType[provider.Type], result)
			continue
		}

		start := time.Now()
		httpClient := &http.Client{Timeout: 15 * time.Second}
		tr := &testResult{}
		ctx := c.Context()

		switch provider.Type {
		case "gemini":
			apiURL := provider.BaseURL + "/v1beta/models/" + provider.Model + ":generateContent?key=" + provider.APIKey
			body := map[string]interface{}{
				"contents": []map[string]interface{}{
					{"parts": []map[string]string{{"text": testPrompt}}},
				},
			}
			b, err := json.Marshal(body)
			if err != nil {
				tr.Success = false
				tr.Error = fmt.Sprintf("marshal: %v", err)
				break
			}
			resp, err := httpClient.Post(apiURL, "application/json", bytes.NewReader(b))
			if err != nil {
				tr.Success = false
				tr.Error = err.Error()
				break
			}
			content, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode >= 400 {
				tr.Success = false
				tr.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(content))
				break
			}

			var geminiResp struct {
				Candidates []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					} `json:"content"`
				} `json:"candidates"`
			}
			if jsonErr := json.Unmarshal(content, &geminiResp); jsonErr != nil {
				tr.Success = false
				tr.Error = fmt.Sprintf("parse: %v — %s", jsonErr, string(content))
				break
			}
			if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
				tr.Success = true
				tr.Response = geminiResp.Candidates[0].Content.Parts[0].Text
				ok++
			} else {
				tr.Success = false
				tr.Error = fmt.Sprintf("empty response: %s", string(content))
			}

		default:
			apiURL := provider.BaseURL + "/v1/chat/completions"
			body := map[string]interface{}{
				"model": provider.Model,
				"messages": []map[string]string{
					{"role": "user", "content": testPrompt},
				},
				"max_tokens": 50,
			}
			b, err := json.Marshal(body)
			if err != nil {
				tr.Success = false
				tr.Error = fmt.Sprintf("marshal: %v", err)
				break
			}
			req, reqErr := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(b))
			if reqErr != nil {
				tr.Success = false
				tr.Error = reqErr.Error()
				break
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+provider.APIKey)

			resp, err := httpClient.Do(req)
			if err != nil {
				tr.Success = false
				tr.Error = err.Error()
				break
			}
			content, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode >= 400 {
				tr.Success = false
				tr.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(content))
				break
			}

			var openAIResp struct {
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if jsonErr := json.Unmarshal(content, &openAIResp); jsonErr != nil {
				tr.Success = false
				tr.Error = fmt.Sprintf("parse: %v — %s", jsonErr, string(content))
				break
			}
			if len(openAIResp.Choices) > 0 {
				tr.Success = true
				tr.Response = openAIResp.Choices[0].Message.Content
				ok++
			} else {
				tr.Success = false
				tr.Error = fmt.Sprintf("empty choices: %s", string(content))
			}
		}

		tr.Latency = time.Since(start).Round(time.Millisecond).String()
		result.Test = tr
		byType[provider.Type] = append(byType[provider.Type], result)

		time.Sleep(200 * time.Millisecond)
	}

	return respondSuccess(c, fiber.Map{
		"results": byType,
		"summary": fiber.Map{
			"total":  total,
			"ok":     ok,
			"failed": total - ok,
		},
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

	// Use the request chat_id when provided; fall back to the global default.
	targetChatID := req.ChatID
	if targetChatID == 0 {
		targetChatID = s.TelegramGroupID
	}
	if targetChatID == 0 {
		return respondError(c, fiber.StatusUnprocessableEntity, ErrValidation, "chat_id is required (no default configured)")
	}

	if err := s.TelegramBot.Send(c.Context(), targetChatID, req.Text); err != nil {
		slog.Error("failed to send message via bot", "error", err)
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, fmt.Sprintf("failed to send: %v", err))
	}

	return respondSuccess(c, fiber.Map{"sent": true, "chat_id": targetChatID})
}
