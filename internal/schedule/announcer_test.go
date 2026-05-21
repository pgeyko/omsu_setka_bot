package schedule

import (
	"context"
	"omsu_bot/internal/llm"
	"testing"
)

type mockLLMClient struct {
	resp *llm.Response
	err  error
}

func (m *mockLLMClient) Call(ctx context.Context, reqType, systemExtra, userPrompt string, requiresVision bool) (*llm.Response, error) {
	return m.resp, m.err
}

type mockPrompts struct {
	content string
}

func (m *mockPrompts) Get(name string) string {
	return m.content
}

func TestAnnouncer_Generate(t *testing.T) {
	client := &mockLLMClient{
		resp: &llm.Response{
			Content: "Внимание! Изменения в расписании.",
		},
	}
	prompts := &mockPrompts{content: "Сгенерируй объявление"}

	a := NewAnnouncer(client, prompts)
	text, err := a.GenerateAnnouncement(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty announcement")
	}
}

func TestAnnouncer_LLMError(t *testing.T) {
	client := &mockLLMClient{
		err: assertAnError("llm failure"),
	}
	prompts := &mockPrompts{content: "prompt"}

	a := NewAnnouncer(client, prompts)
	_, err := a.GenerateAnnouncement(context.Background(), nil, nil)
	if err == nil {
		t.Error("expected error when LLM fails")
	}
}

func assertAnError(msg string) error {
	return &testError{msg: msg}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
