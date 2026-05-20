package llm

import (
	"testing"
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
