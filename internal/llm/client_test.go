package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"omsu_bot/internal/db"
)

type mockPersonaProvider struct {
	prompt string
}

func (m *mockPersonaProvider) SystemPrompt() string {
	return m.prompt
}

func TestBuildMessages(t *testing.T) {
	persona := &mockPersonaProvider{prompt: "Ты — помощник"}
	client := &Client{persona: persona}

	msgs := client.buildMessages("", "привет")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("expected system role, got '%s'", msgs[0].Role)
	}
	if msgs[0].Content != "Ты — помощник" {
		t.Errorf("expected system prompt, got '%s'", msgs[0].Content)
	}
	if msgs[1].Role != "user" {
		t.Errorf("expected user role, got '%s'", msgs[1].Role)
	}
	if msgs[1].Content != "привет" {
		t.Errorf("expected user content 'привет', got '%s'", msgs[1].Content)
	}
}

func TestBuildMessages_WithSystemExtra(t *testing.T) {
	persona := &mockPersonaProvider{prompt: "Ты — помощник"}
	client := &Client{persona: persona}

	msgs := client.buildMessages("Дополнительная инструкция", "привет")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	expected := "Ты — помощник\n\nДополнительная инструкция"
	if msgs[0].Content != expected {
		t.Errorf("expected combined system prompt, got '%s'", msgs[0].Content)
	}
}

func TestClient_TokenParsing(t *testing.T) {
	// 1. Setup in-memory DB and Migrate
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory db: %v", err)
	}
	defer database.Close()
	if err := database.Migrate(); err != nil {
		t.Fatalf("failed to migrate db: %v", err)
	}

	// 2. Setup httptest Server
	var responseBody []byte
	var responseStatus int = 200

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(responseStatus)
		w.Write(responseBody)
	}))
	defer ts.Close()

	// 3. Setup client and tracker
	tracker := NewTracker(database.DB, 1000, 0.8)
	persona := &mockPersonaProvider{prompt: "test"}

	// Test Gemini response
	geminiBody := `{
		"candidates": [
			{
				"content": {
					"role": "model",
					"parts": [
						{"text": "Hello, this is Gemini."}
					]
				}
			}
		],
		"usageMetadata": {
			"promptTokenCount": 12,
			"candidatesTokenCount": 34
		}
	}`

	// Prepare Gemini provider
	geminiProvider := &Provider{
		Name: "test-gemini",
		Type: "gemini",
		BaseURL: ts.URL,
		APIKey: "key",
		Model: "gemini-1.5-flash",
	}

	textChain := NewChain([]*Provider{geminiProvider})
	client := NewClient(nil, textChain, nil, nil, tracker, persona, nil, 5, true)

	responseBody = []byte(geminiBody)
	responseStatus = 200

	resp, err := client.Call(context.Background(), "test", "", "hello", false)
	if err != nil {
		t.Fatalf("gemini call failed: %v", err)
	}
	if resp.InputTokens != 12 {
		t.Errorf("expected 12 input tokens, got %d", resp.InputTokens)
	}
	if resp.OutputTokens != 34 {
		t.Errorf("expected 34 output tokens, got %d", resp.OutputTokens)
	}
	if resp.Content != "Hello, this is Gemini." {
		t.Errorf("expected content 'Hello, this is Gemini.', got '%s'", resp.Content)
	}

	// Test OpenAI/DeepSeek response
	openAIBody := `{
		"choices": [
			{
				"message": {
					"content": "Hello, this is OpenAI."
				}
			}
		],
		"usage": {
			"prompt_tokens": 56,
			"completion_tokens": 78
		}
	}`

	openAIProvider := &Provider{
		Name: "test-openai",
		Type: "openai",
		BaseURL: ts.URL,
		APIKey: "key",
		Model: "gpt-4o",
	}

	chainOpenAI := NewChain([]*Provider{openAIProvider})
	clientOpenAI := NewClient(nil, chainOpenAI, nil, nil, tracker, persona, nil, 5, true)

	responseBody = []byte(openAIBody)

	respOpenAI, err := clientOpenAI.Call(context.Background(), "test", "", "hello", false)
	if err != nil {
		t.Fatalf("openai call failed: %v", err)
	}
	if respOpenAI.InputTokens != 56 {
		t.Errorf("expected 56 input tokens, got %d", respOpenAI.InputTokens)
	}
	if respOpenAI.OutputTokens != 78 {
		t.Errorf("expected 78 output tokens, got %d", respOpenAI.OutputTokens)
	}
	if respOpenAI.Content != "Hello, this is OpenAI." {
		t.Errorf("expected content 'Hello, this is OpenAI.', got '%s'", respOpenAI.Content)
	}
}

