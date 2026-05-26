package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"omsu_bot/internal/persona"
	"omsu_bot/internal/util"
)

const (
	interModelDelay      = 120 * time.Millisecond
	logTruncateLen       = 500
	contentTruncateLen   = 500
	errorTruncateLen     = 500
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
	ID                string       `json:"id"`
	Type              string       `json:"type"` // e.g. "function"
	Function          FunctionCall `json:"function"`
	ThoughtSignature  string       `json:"thought_signature,omitempty"` // Gemini-specific
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
	// ThoughtSignature is used by Gemini to correlate function calls across turns
	ThoughtSignature string `json:"thought_signature,omitempty"`

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

// LLMClient is the public interface consumed by external packages.
type LLMClient interface {
	Call(ctx context.Context, reqType, systemExtra, userPrompt string, requiresVision bool) (*Response, error)
	CallGroupHistory(ctx context.Context, chatID int64, reqType, systemExtra string, history []AgentMessage, tools []Tool, requiresVision bool) (*Response, error)
	CallWithSystemHistory(ctx context.Context, chatID int64, reqType, systemPrompt string, history []AgentMessage, requiresVision bool) (*Response, error)
	CallWithSystemPrompt(ctx context.Context, reqType, systemPrompt, userPrompt string) (*Response, error)
	HasMultimodalProvider() bool
	SetSkipFallbackModel(skip bool)
}

type Client struct {
	agentChain        *Chain
	simpleChain       *Chain
	visionChain       *Chain
	audioChain        *Chain
	tracker           *Tracker
	persona           PersonaProvider
	prompts           *PromptRegistry
	httpClient        *http.Client
	skipFallbackModel bool
}

func NewClient(agentChain, simpleChain, visionChain, audioChain *Chain, tracker *Tracker, persona PersonaProvider, prompts *PromptRegistry, timeoutSec int, skipFallbackModel bool) *Client {
	return &Client{
		agentChain:  agentChain,
		simpleChain: simpleChain,
		visionChain: visionChain,
		audioChain:  audioChain,
		tracker:     tracker,
		persona:     persona,
		prompts:     prompts,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
		skipFallbackModel: skipFallbackModel,
	}
}

func (c *Client) PickChain(taskType string, requiresVision bool) *Chain {
	if requiresVision && c.visionChain != nil {
		return c.visionChain
	}
	switch taskType {
	case "agent_loop":
		if c.agentChain != nil {
			return c.agentChain
		}
		return c.simpleChain
	case "ocr", "stt":
		if taskType == "stt" && c.audioChain != nil {
			return c.audioChain
		}
		if c.visionChain != nil {
			return c.visionChain
		}
		return c.simpleChain
	default:
		if c.simpleChain != nil {
			return c.simpleChain
		}
		return c.agentChain
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

func (c *Client) CallWithSystemPrompt(ctx context.Context, reqType, systemPrompt, userPrompt string) (*Response, error) {
	return c.CallGroupWithSystem(ctx, 0, reqType, systemPrompt, userPrompt)
}

func (c *Client) CallGroupWithSystem(ctx context.Context, chatID int64, reqType, systemPrompt, userPrompt string) (*Response, error) {
	history := []AgentMessage{{Role: "user", Content: userPrompt}}
	return c.callHistoryWithSystem(ctx, chatID, reqType, systemPrompt, history, nil, false)
}

func (c *Client) CallWithSystemHistory(ctx context.Context, chatID int64, reqType, systemPrompt string, history []AgentMessage, requiresVision bool) (*Response, error) {
	return c.callHistoryWithSystem(ctx, chatID, reqType, systemPrompt, history, nil, requiresVision)
}

func (c *Client) callHistoryWithSystem(ctx context.Context, chatID int64, reqType, systemContent string, history []AgentMessage, tools []Tool, requiresVision bool) (*Response, error) {
	if c.tracker.IsLimitReached() {
		return nil, fmt.Errorf("daily token limit reached")
	}

	// Reject if estimated input would exceed remaining daily budget
	estInput := len(systemContent)/4 + historyTokens(history)
	if c.tracker.WouldExceed(estInput) {
		return nil, fmt.Errorf("request would exceed daily token limit (est %d tokens)", estInput)
	}

	chain := c.PickChain(reqType, requiresVision)
	if chain == nil {
		return nil, fmt.Errorf("no chain available for request type=%s (vision=%v)", reqType, requiresVision)
	}

	requiredCaps := []Capability{}
	if requiresVision {
		requiredCaps = append(requiredCaps, CapabilityMultimodal)
	}

	providers := chain.Providers()
	var lastErr error
	tried := 0

	for _, provider := range providers {
		slog.Debug("provider iteration",
			"provider", provider.Name,
			"tried", tried,
			"remaining", len(providers)-tried,
		)
		if !provider.IsActive() {
			continue
		}
		hasAllCaps := true
		for _, cap := range requiredCaps {
			if !provider.HasCapability(cap) {
				hasAllCaps = false
				break
			}
		}
		if !hasAllCaps {
			continue
		}

		modelsToTry := []string{provider.Model}
		if !c.skipFallbackModel {
			modelsToTry = append(modelsToTry, provider.FallbackModels...)
		}

		for mi, model := range modelsToTry {
			// Small delay between requests to avoid rate-limit bursts (part of retry loop)
			if tried > 0 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(interModelDelay):
				}
			}
			tried++
			slog.Debug("llm request history",
				"type", reqType,
				"provider", provider.Name,
				"model", model,
				"requires_vision", requiresVision,
				"system_prompt", util.Truncate(systemContent, logTruncateLen),
				"history_len", len(history),
				"tools_len", len(tools),
			)

			resp, err := c.callProviderHistory(ctx, provider, model, systemContent, history, tools)
			if err == nil {
				resp.Provider = provider.Name
				resp.Model = model
				provider.recordCall(resp.InputTokens + resp.OutputTokens)
				slog.Debug("llm response history",
					"type", reqType,
					"provider", provider.Name,
					"input_tokens", resp.InputTokens,
					"output_tokens", resp.OutputTokens,
					"tool_calls_len", len(resp.ToolCalls),
					"content_truncated", util.Truncate(resp.Content, logTruncateLen),
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

func (c *Client) CallGroupHistory(ctx context.Context, chatID int64, reqType, systemExtra string, history []AgentMessage, tools []Tool, requiresVision bool) (*Response, error) {
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
	return c.callHistoryWithSystem(ctx, chatID, reqType, systemContent, history, tools, requiresVision)
}

func (c *Client) callWhisper(ctx context.Context, provider *Provider, model string, history []AgentMessage) (*Response, error) {
	var audioData []byte
	for _, msg := range history {
		if len(msg.MediaParts) > 0 {
			audioData = msg.MediaParts[0].Data
			break
		}
	}
	if len(audioData) == 0 {
		return nil, fmt.Errorf("no audio data in whisper request")
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", "audio.ogg")
	if err != nil {
		slog.Warn("whisper create form file", "error", err)
		return nil, fmt.Errorf("whisper create form file failed: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		slog.Warn("whisper write audio", "error", err)
	}
	if err := w.WriteField("model", model); err != nil {
		slog.Warn("whisper write field model", "error", err)
	}
	if err := w.WriteField("language", "ru"); err != nil {
		slog.Warn("whisper write field language", "error", err)
	}
	if err := w.Close(); err != nil {
		slog.Warn("whisper close multipart", "error", err)
	}

	apiURL := provider.BaseURL + "/v1/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("whisper request failed: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("whisper request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read whisper response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("whisper returned status %d: %s", httpResp.StatusCode, util.Truncate(string(respBody), errorTruncateLen))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse whisper response: %w", err)
	}

	return &Response{
		Content:      strings.TrimSpace(result.Text),
		InputTokens:  0,
		OutputTokens: 0,
	}, nil
}

func (c *Client) callProviderHistory(ctx context.Context, provider *Provider, model string, systemContent string, history []AgentMessage, tools []Tool) (*Response, error) {
	if strings.HasPrefix(model, "whisper") {
		return c.callWhisper(ctx, provider, model, history)
	}

	var httpReq *http.Request
	var errReq error

	switch provider.Type {
	case "gemini":
		httpReq, errReq = c.buildGeminiRequest(ctx, provider, model, systemContent, history, tools)
	default:
		httpReq, errReq = c.buildOpenAIRequest(ctx, provider, model, systemContent, history, tools)
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

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm returned status %d: %s", httpResp.StatusCode, util.Truncate(string(respBody), errorTruncateLen))
	}

	var resp *Response
	switch provider.Type {
	case "gemini":
		resp, err = parseGeminiResponse(respBody)
	default:
		resp, err = parseOpenAIResponse(respBody)
	}
	if err != nil {
		return nil, err
	}
	resp.Content = stripCJK(resp.Content)
	return resp, nil
}

// stripCJK removes CJK (Chinese, Japanese, Korean) characters from s
// and strips <think>...</think> reasoning tags.
func stripCJK(s string) string {
	// Strip <think>...</think> tags (qwen3, deepseek reasoning models)
	if idx := strings.Index(s, "<think>"); idx != -1 {
		end := strings.Index(s[idx:], "</think>")
		if end != -1 {
			s = s[:idx] + s[idx+end+8:]
		}
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= 0x4E00 && r <= 0x9FFF) ||
			(r >= 0x3400 && r <= 0x4DBF) ||
			(r >= 0x3040 && r <= 0x30FF) ||
			(r >= 0xAC00 && r <= 0xD7AF) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func (c *Client) buildGeminiRequest(ctx context.Context, provider *Provider, model, systemContent string, history []AgentMessage, tools []Tool) (*http.Request, error) {
	apiURL := provider.BaseURL + "/v1beta/models/" + model + ":generateContent"

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

	b, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("gemini marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("gemini request: %w", err)
	}
	req.Header.Set("x-goog-api-key", provider.APIKey)
	return req, nil
}

func (c *Client) buildOpenAIRequest(ctx context.Context, provider *Provider, model, systemContent string, history []AgentMessage, tools []Tool) (*http.Request, error) {
	apiURL := provider.BaseURL + "/v1/chat/completions"

	var msgs []interface{}
	msgs = append(msgs, map[string]interface{}{
		"role":    "system",
		"content": systemContent,
	})
	msgs = append(msgs, buildOpenAIContents(history)...)

	openAIReq := map[string]interface{}{
		"model":    model,
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

	b, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("openai marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	return req, nil
}

func parseGeminiResponse(respBody []byte) (*Response, error) {
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Role  string `json:"role"`
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name             string                 `json:"name"`
						Args             map[string]interface{} `json:"args"`
						ThoughtSignature string                 `json:"thought_signature"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal gemini response: %w", err)
	}

	resp := &Response{}
	if len(geminiResp.Candidates) > 0 {
		candidate := geminiResp.Candidates[0]
		for _, part := range candidate.Content.Parts {
			if part.FunctionCall != nil {
				argsBytes, err := json.Marshal(part.FunctionCall.Args)
				if err != nil {
					slog.Warn("gemini marshal function call args", "error", err)
				}
				resp.ToolCalls = append(resp.ToolCalls, ToolCall{
					ID:   fmt.Sprintf("call_%d", time.Now().UnixNano()),
					Type: "function",
					Function: FunctionCall{
						Name:      part.FunctionCall.Name,
						Arguments: string(argsBytes),
					},
					ThoughtSignature: part.FunctionCall.ThoughtSignature,
				})
			}
			if part.Text != "" {
				resp.Content = part.Text
			}
		}
	}
	resp.InputTokens = geminiResp.UsageMetadata.PromptTokenCount
	resp.OutputTokens = geminiResp.UsageMetadata.CandidatesTokenCount
	return resp, nil
}

func parseOpenAIResponse(respBody []byte) (*Response, error) {
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
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal openai response: %w", err)
	}

	resp := &Response{}
	if len(openAIResp.Choices) > 0 {
		choice := openAIResp.Choices[0]
		resp.Content = choice.Message.Content
		for _, tc := range choice.Message.ToolCalls {
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}
	resp.InputTokens = openAIResp.Usage.PromptTokens
	resp.OutputTokens = openAIResp.Usage.CompletionTokens
	return resp, nil
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
				fc := map[string]interface{}{
					"name": tc.Function.Name,
				}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
					fc["args"] = args
				} else {
					fc["args"] = map[string]interface{}{}
				}
				if tc.ThoughtSignature != "" {
					fc["thought_signature"] = tc.ThoughtSignature
				}
				parts = append(parts, map[string]interface{}{
					"functionCall": fc,
				})
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
					} else if strings.HasPrefix(media.MimeType, "audio/") {
						base64Str := base64.StdEncoding.EncodeToString(media.Data)
						contentParts = append(contentParts, map[string]interface{}{
							"type": "input_audio",
							"input_audio": map[string]interface{}{
								"data":   base64Str,
								"format": strings.TrimPrefix(media.MimeType, "audio/"),
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

// HasMultimodalProvider returns true when at least one active provider in
// vision or audio chain supports the multimodal capability.
func (c *Client) HasMultimodalProvider() bool {
	for _, chain := range []*Chain{c.visionChain, c.audioChain} {
		if chain == nil {
			continue
		}
		for _, p := range chain.Providers() {
			if p.IsActive() && p.HasCapability(CapabilityMultimodal) {
				return true
			}
		}
	}
	// Log all providers for diagnostics
	for _, chain := range []*Chain{c.visionChain, c.audioChain} {
		if chain == nil {
			slog.Debug("multimodal chain is nil")
			continue
		}
		for _, p := range chain.Providers() {
			slog.Debug("multimodal provider state",
				"name", p.Name,
				"active", p.IsActive(),
				"has_multimodal", p.HasCapability(CapabilityMultimodal),
			)
		}
	}
	return false
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

func historyTokens(history []AgentMessage) int {
	total := 0
	for _, msg := range history {
		total += len(msg.Content) / 4
		for _, tc := range msg.ToolCalls {
			total += len(tc.Function.Name)/4 + len(tc.Function.Arguments)/4
		}
	}
	return total
}
