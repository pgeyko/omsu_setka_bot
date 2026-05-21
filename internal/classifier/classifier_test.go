package classifier

import (
	"context"
	"strings"
	"testing"

	"omsu_bot/internal/llm"
)

type mockLLMClient struct {
	resp *llm.Response
	err  error
}

func (m *mockLLMClient) Call(ctx context.Context, reqType, systemExtra, userPrompt string, requiresVision bool) (*llm.Response, error) {
	return m.resp, m.err
}

type mockTopicsProvider struct {
	topics []TopicInfo
}

func (m *mockTopicsProvider) GetTopics(ctx context.Context, chatID int64) ([]TopicInfo, error) {
	return m.topics, nil
}

type mockPromptRegistry struct {
	prompts map[string]string
}

func (m *mockPromptRegistry) Get(name string) string {
	return m.prompts[name]
}

func TestFillPrompt(t *testing.T) {
	c := &Classifier{}
	topics := []TopicInfo{
		{Slug: "session", Name: "Сессия", Description: "Экзамены"},
		{Slug: "lectures", Name: "Лекции", Description: "Лекционные занятия"},
	}

	result := c.fillPrompt("{topics}\n{text}", topics, "Привет!")
	if result == "" {
		t.Fatal("expected non-empty prompt")
	}
}

func TestReplacePlaceholder(t *testing.T) {
	result := replacePlaceholder("Hello {name}!", "{name}", "World")
	if result != "Hello World!" {
		t.Errorf("expected 'Hello World!', got '%s'", result)
	}
}

func TestReplacePlaceholder_Multiple(t *testing.T) {
	result := replacePlaceholder("{a} and {a}", "{a}", "X")
	if result != "X and X" {
		t.Errorf("expected 'X and X', got '%s'", result)
	}
}

func countWords(s string) int {
	if len(s) < 15 {
		return 0
	}
	return len(strings.Fields(s))
}

func TestCountWords_BelowThreshold(t *testing.T) {
	result := countWords("hello world")
	if result != 0 {
		t.Errorf("expected 0 for short text, got %d", result)
	}
}
