package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Messages []Message `json:"messages"`
}

type Response struct {
	Content      string
	InputTokens  int
	OutputTokens int
	Model        string
	Provider     string
}

type PersonaProvider interface {
	SystemPrompt() string
}

type Client struct {
	chain            *Chain
	tracker          *Tracker
	persona          PersonaProvider
	prompts          *PromptRegistry
	httpClient       *http.Client
	skipFallbackModel bool
}

func NewClient(chain *Chain, tracker *Tracker, persona PersonaProvider, prompts *PromptRegistry, timeoutSec int, skipFallbackModel bool) *Client {
	return &Client{
		chain:   chain,
		tracker: tracker,
		persona: persona,
		prompts: prompts,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
		skipFallbackModel: skipFallbackModel,
	}
}

func (c *Client) buildMessages(systemExtra, userPrompt string) []Message {
	system := c.persona.SystemPrompt()
	if systemExtra != "" {
		system += "\n\n" + systemExtra
	}
	return []Message{
		{Role: "system", Content: system},
		{Role: "user", Content: userPrompt},
	}
}

func (c *Client) Call(ctx context.Context, reqType, systemExtra, userPrompt string, requiresVision bool) (*Response, error) {
	if c.tracker.IsLimitReached() {
		return nil, fmt.Errorf("daily token limit reached")
	}

	requiredCaps := []Capability{}
	if requiresVision {
		requiredCaps = append(requiredCaps, CapabilityMultimodal)
	}

	msgs := c.buildMessages(systemExtra, userPrompt)
	systemContent := ""
	userContent := userPrompt
	if len(msgs) == 2 {
		systemContent = msgs[0].Content
	}

	providers := c.chain.Providers()
	var lastErr error
	tried := 0

	for _, provider := range providers {
		if !provider.IsActive() {
			continue
		}

		modelsToTry := []string{provider.Model}
		if !c.skipFallbackModel {
			modelsToTry = append(modelsToTry, provider.FallbackModels...)
		}

		for mi, model := range modelsToTry {
			tried++

			slog.Debug("llm request",
				"type", reqType,
				"provider", provider.Name,
				"model", model,
				"requires_vision", requiresVision,
				"system_prompt", truncate(systemContent, 500),
				"user_prompt", truncate(userPrompt, 2000),
			)

			provider.Model = model
			resp, err := c.callProvider(ctx, provider, systemContent, userContent, msgs)
			if err == nil {
				resp.Provider = provider.Name
				resp.Model = model

				slog.Debug("llm response",
					"type", reqType,
					"provider", provider.Name,
					"input_tokens", resp.InputTokens,
					"output_tokens", resp.OutputTokens,
					"content", resp.Content,
				)

				if err := c.tracker.LogRequest(ctx, reqType, provider.Name, model, resp.InputTokens, resp.OutputTokens, 0); err != nil {
					return resp, fmt.Errorf("llm ok but failed to log: %w", err)
				}
				return resp, nil
			}

			lastErr = err
			if mi < len(modelsToTry)-1 {
				slog.Warn("llm model failed, trying fallback on same key",
					"provider", provider.Name,
					"model", model,
					"error", err,
				)
			}
		}

		provider.RecordFailure()
		slog.Warn("llm provider failed, trying next provider",
			"provider", provider.Name,
			"error", lastErr,
			"tried", tried,
			"remaining", len(providers)-tried,
		)
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed (%d tried), last error: %w", tried, lastErr)
	}
	return nil, fmt.Errorf("no active provider available")
}

func (c *Client) callProvider(ctx context.Context, provider *Provider, systemContent, userContent string, msgs []Message) (*Response, error) {
	var apiURL string
	var httpReq *http.Request
	var errReq error

	switch provider.Type {
	case "gemini":
		apiURL = provider.BaseURL + "/v1beta/models/" + provider.Model + ":generateContent?key=" + provider.APIKey
		geminiReq := map[string]interface{}{
			"system_instruction": map[string]interface{}{
				"parts": []map[string]string{{"text": systemContent}},
			},
			"contents": []map[string]interface{}{
				{
					"parts": []map[string]string{{"text": userContent}},
				},
			},
		}
		var b []byte
		b, errReq = json.Marshal(geminiReq)
		if errReq == nil {
			httpReq, errReq = http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(b))
		}
	default:
		apiURL = provider.BaseURL + "/v1/chat/completions"
		openAIReq := map[string]interface{}{
			"model":    provider.Model,
			"messages": msgs,
		}
		var b []byte
		b, errReq = json.Marshal(openAIReq)
		if errReq == nil {
			httpReq, errReq = http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(b))
			httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
		}
	}

	if errReq != nil {
		return nil, fmt.Errorf("failed to create request: %w", errReq)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != 200 {
		return nil, fmt.Errorf("llm returned status %d: %s", httpResp.StatusCode, truncate(string(respBody), 500))
	}

	content := string(respBody)
	switch provider.Type {
	case "gemini":
		var geminiResp struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
			UsageMetadata struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
			} `json:"usageMetadata"`
		}
		if err := json.Unmarshal(respBody, &geminiResp); err == nil {
			if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
				content = geminiResp.Candidates[0].Content.Parts[0].Text
			}
		}
	default:
		var openAIResp struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(respBody, &openAIResp); err == nil {
			if len(openAIResp.Choices) > 0 {
				content = openAIResp.Choices[0].Message.Content
			}
		}
	}

	return &Response{Content: content}, nil
}

func (c *Client) SetSkipFallbackModel(skip bool) {
	c.skipFallbackModel = skip
}

func ExtractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Check for markdown code blocks first
	if idx := strings.Index(s, "```"); idx != -1 {
		rest := s[idx+3:]
		if end := strings.Index(rest, "```"); end != -1 {
			candidate := strings.TrimSpace(rest[:end])
			if strings.HasPrefix(candidate, "json") {
				candidate = strings.TrimSpace(candidate[4:])
			}
			if len(candidate) > 0 && (candidate[0] == '{' || candidate[0] == '[') {
				return trimToBalanced(candidate)
			}
		}
	}
	// Find first { or [
	start := -1
	for i, ch := range s {
		if ch == '{' || ch == '[' {
			start = i
			break
		}
	}
	if start == -1 {
		return s
	}
	return trimToBalanced(s[start:])
}

func trimToBalanced(s string) string {
	if len(s) == 0 {
		return s
	}
	openChar := s[0]
	closeChar := byte('}')
	if openChar == '[' {
		closeChar = ']'
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		if s[i] == openChar {
			depth++
		} else if s[i] == closeChar {
			depth--
			if depth == 0 {
				return s[:i+1]
			}
		}
	}
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
