package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"omsu_bot/internal/persona"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Messages []Message `json:"messages"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"` // JSON Schema (map[string]interface{})
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // e.g. "function"
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

type MediaPart struct {
	MimeType string `json:"mime_type"`
	Data     []byte `json:"data"`
}

type AgentMessage struct {
	Role       string      `json:"role"` // "system", "user", "assistant", "tool"
	Content    string      `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"` // used for role "tool" in OpenAI
	ToolName   string      `json:"tool_name,omitempty"`    // used for role "tool" in Gemini
	MediaParts []MediaPart `json:"media_parts,omitempty"`
}

type Response struct {
	Content      string
	InputTokens  int
	OutputTokens int
	Model        string
	Provider     string
	ToolCalls    []ToolCall
}

type PersonaProvider interface {
	SystemPrompt() string
}

type Client struct {
	chain             *Chain
	tracker           *Tracker
	persona           PersonaProvider
	prompts           *PromptRegistry
	httpClient        *http.Client
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
	return c.CallGroup(ctx, 0, reqType, systemExtra, userPrompt, requiresVision)
}

func (c *Client) CallGroup(ctx context.Context, chatID int64, reqType, systemExtra, userPrompt string, requiresVision bool) (*Response, error) {
	history := []AgentMessage{
		{Role: "user", Content: userPrompt},
	}
	return c.CallGroupHistory(ctx, chatID, reqType, systemExtra, history, nil, requiresVision)
}

func (c *Client) CallGroupHistory(ctx context.Context, chatID int64, reqType, systemExtra string, history []AgentMessage, tools []Tool, requiresVision bool) (*Response, error) {
	if c.tracker.IsLimitReached() {
		return nil, fmt.Errorf("daily token limit reached")
	}

	requiredCaps := []Capability{}
	if requiresVision {
		requiredCaps = append(requiredCaps, CapabilityMultimodal)
	}

	var systemContent string
	if store, ok := c.persona.(*persona.Store); ok && store != nil {
		defaultPersona := store.Get()
		sysPrompt, _ := persona.GetGroupSystemPrompt(chatID, defaultPersona)
		systemContent = sysPrompt
	} else {
		systemContent = c.persona.SystemPrompt()
	}

	if systemExtra != "" {
		systemContent += "\n\n" + systemExtra
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

			slog.Debug("llm request history",
				"type", reqType,
				"provider", provider.Name,
				"model", model,
				"requires_vision", requiresVision,
				"system_prompt", truncate(systemContent, 500),
				"history_len", len(history),
				"tools_len", len(tools),
			)

			provider.Model = model
			resp, err := c.callProviderHistory(ctx, provider, systemContent, history, tools)
			if err == nil {
				resp.Provider = provider.Name
				resp.Model = model

				slog.Debug("llm response history",
					"type", reqType,
					"provider", provider.Name,
					"input_tokens", resp.InputTokens,
					"output_tokens", resp.OutputTokens,
					"tool_calls_len", len(resp.ToolCalls),
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

func (c *Client) callProviderHistory(ctx context.Context, provider *Provider, systemContent string, history []AgentMessage, tools []Tool) (*Response, error) {
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
			"contents": buildGeminiContents(history),
		}

		if len(tools) > 0 {
			var decls []interface{}
			for _, t := range tools {
				decls = append(decls, map[string]interface{}{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  formatSchemaTypes(t.Parameters, true),
				})
			}
			geminiReq["tools"] = []interface{}{
				map[string]interface{}{
					"function_declarations": decls,
				},
			}
		}

		var b []byte
		b, errReq = json.Marshal(geminiReq)
		if errReq == nil {
			httpReq, errReq = http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(b))
		}

	default: // openai / deepseek
		apiURL = provider.BaseURL + "/v1/chat/completions"
		
		var msgs []interface{}
		msgs = append(msgs, map[string]interface{}{
			"role":    "system",
			"content": systemContent,
		})
		msgs = append(msgs, buildOpenAIContents(history)...)

		openAIReq := map[string]interface{}{
			"model":    provider.Model,
			"messages": msgs,
		}

		if len(tools) > 0 {
			var openAITools []interface{}
			for _, t := range tools {
				openAITools = append(openAITools, map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name":        t.Name,
						"description": t.Description,
						"parameters":  formatSchemaTypes(t.Parameters, false),
					},
				})
			}
			openAIReq["tools"] = openAITools
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

	var inputTokens, outputTokens int
	var toolCalls []ToolCall
	content := ""

	switch provider.Type {
	case "gemini":
		var geminiResp struct {
			Candidates []struct {
				Content struct {
					Role  string `json:"role"`
					Parts []struct {
						Text         string `json:"text"`
						FunctionCall *struct {
							Name string                 `json:"name"`
							Args map[string]interface{} `json:"args"`
						} `json:"functionCall"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
			UsageMetadata struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
			} `json:"usageMetadata"`
		}
		if err := json.Unmarshal(respBody, &geminiResp); err == nil {
			if len(geminiResp.Candidates) > 0 {
				candidate := geminiResp.Candidates[0]
				for _, part := range candidate.Content.Parts {
					if part.FunctionCall != nil {
						argsBytes, _ := json.Marshal(part.FunctionCall.Args)
						toolCalls = append(toolCalls, ToolCall{
							ID:   fmt.Sprintf("call_%d", time.Now().UnixNano()),
							Type: "function",
							Function: FunctionCall{
								Name:      part.FunctionCall.Name,
								Arguments: string(argsBytes),
							},
						})
					}
					if part.Text != "" {
						content = part.Text
					}
				}
			}
			inputTokens = geminiResp.UsageMetadata.PromptTokenCount
			outputTokens = geminiResp.UsageMetadata.CandidatesTokenCount
		} else {
			return nil, fmt.Errorf("failed to unmarshal gemini response: %w", err)
		}
	default:
		var openAIResp struct {
			Choices []struct {
				Message struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"message"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(respBody, &openAIResp); err == nil {
			if len(openAIResp.Choices) > 0 {
				choice := openAIResp.Choices[0]
				content = choice.Message.Content
				if len(choice.Message.ToolCalls) > 0 {
					for _, tc := range choice.Message.ToolCalls {
						toolCalls = append(toolCalls, ToolCall{
							ID:   tc.ID,
							Type: tc.Type,
							Function: FunctionCall{
								Name:      tc.Function.Name,
								Arguments: tc.Function.Arguments,
							},
						})
					}
				}
			}
			inputTokens = openAIResp.Usage.PromptTokens
			outputTokens = openAIResp.Usage.CompletionTokens
		} else {
			return nil, fmt.Errorf("failed to unmarshal openai response: %w", err)
		}
	}

	return &Response{
		Content:      content,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		ToolCalls:    toolCalls,
	}, nil
}

func formatSchemaTypes(v interface{}, uppercase bool) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		res := make(map[string]interface{})
		for k, valItem := range val {
			if k == "type" {
				if valStr, ok := valItem.(string); ok {
					if uppercase {
						res[k] = strings.ToUpper(valStr)
					} else {
						res[k] = strings.ToLower(valStr)
					}
					continue
				}
			}
			res[k] = formatSchemaTypes(valItem, uppercase)
		}
		return res
	case []interface{}:
		res := make([]interface{}, len(val))
		for i, valItem := range val {
			res[i] = formatSchemaTypes(valItem, uppercase)
		}
		return res
	default:
		return v
	}
}

func buildGeminiContents(history []AgentMessage) []interface{} {
	var contents []interface{}
	for _, msg := range history {
		if msg.Role == "system" {
			continue
		}

		var parts []interface{}
		role := "user"

		switch msg.Role {
		case "user":
			role = "user"
			parts = append(parts, map[string]interface{}{"text": msg.Content})
			for _, media := range msg.MediaParts {
				parts = append(parts, map[string]interface{}{
					"inline_data": map[string]interface{}{
						"mime_type": media.MimeType,
						"data":      base64.StdEncoding.EncodeToString(media.Data),
					},
				})
			}

		case "assistant":
			role = "model"
			if msg.Content != "" {
				parts = append(parts, map[string]interface{}{"text": msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
					parts = append(parts, map[string]interface{}{
						"functionCall": map[string]interface{}{
							"name": tc.Function.Name,
							"args": args,
						},
					})
				} else {
					parts = append(parts, map[string]interface{}{
						"functionCall": map[string]interface{}{
							"name": tc.Function.Name,
							"args": map[string]interface{}{},
						},
					})
				}
			}

		case "tool":
			role = "user"
			var resObj interface{}
			var jsonTest interface{}
			if err := json.Unmarshal([]byte(msg.Content), &jsonTest); err == nil {
				resObj = jsonTest
			} else {
				resObj = map[string]interface{}{"result": msg.Content}
			}

			parts = append(parts, map[string]interface{}{
				"functionResponse": map[string]interface{}{
					"name":     msg.ToolName,
					"response": map[string]interface{}{"content": resObj},
				},
			})
		}

		if len(parts) > 0 {
			contents = append(contents, map[string]interface{}{
				"role":  role,
				"parts": parts,
			})
		}
	}
	return contents
}

func buildOpenAIContents(history []AgentMessage) []interface{} {
	var contents []interface{}
	for _, msg := range history {
		m := map[string]interface{}{
			"role": msg.Role,
		}
		if msg.Role == "system" {
			m["content"] = msg.Content
		} else if msg.Role == "user" {
			if len(msg.MediaParts) == 0 {
				m["content"] = msg.Content
			} else {
				var contentParts []interface{}
				if msg.Content != "" {
					contentParts = append(contentParts, map[string]interface{}{
						"type": "text",
						"text": msg.Content,
					})
				}
				for _, media := range msg.MediaParts {
					if strings.HasPrefix(media.MimeType, "image/") {
						base64Str := base64.StdEncoding.EncodeToString(media.Data)
						contentParts = append(contentParts, map[string]interface{}{
							"type": "image_url",
							"image_url": map[string]interface{}{
								"url": fmt.Sprintf("data:%s;base64,%s", media.MimeType, base64Str),
							},
						})
					}
				}
				m["content"] = contentParts
			}
		} else if msg.Role == "assistant" {
			if msg.Content != "" {
				m["content"] = msg.Content
			}
			if len(msg.ToolCalls) > 0 {
				var tc []interface{}
				for _, t := range msg.ToolCalls {
					tc = append(tc, map[string]interface{}{
						"id":   t.ID,
						"type": "function",
						"function": map[string]interface{}{
							"name":      t.Function.Name,
							"arguments": t.Function.Arguments,
						},
					})
				}
				m["tool_calls"] = tc
			}
		} else if msg.Role == "tool" {
			m["tool_call_id"] = msg.ToolCallID
			m["content"] = msg.Content
		}
		contents = append(contents, m)
	}
	return contents
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
