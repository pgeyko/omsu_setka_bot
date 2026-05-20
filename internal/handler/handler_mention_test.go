package handlers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommandRegistry_Load(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "commands.json")
	os.WriteFile(p, []byte(`{"topic_commands":["создай топик"],"summary_commands":["саммари"],"id_queries":["какой id"],"responses":{}}`), 0644)

	r, err := LoadCommands(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsTopicCommand("создай топик тест") {
		t.Error("expected topic command match")
	}
	if r.IsTopicCommand("привет") {
		t.Error("expected no match")
	}
}

func TestCommandRegistry_IsTopicCommand(t *testing.T) {
	r := &CommandRegistry{TopicCommands: []string{"создай топик", "закрой топик"}}

	tests := []struct{ text string; result bool }{
		{"создай топик Практика", true},
		{"закрой топик сессия", true},
		{"перешли в сессию", false},
		{"привет", false},
	}
	for _, tt := range tests {
		got := r.IsTopicCommand(tt.text)
		if got != tt.result {
			t.Errorf("IsTopicCommand('%s') = %v, want %v", tt.text, got, tt.result)
		}
	}
}

func TestCommandRegistry_IsSummaryCommand(t *testing.T) {
	r := &CommandRegistry{SummaryCmds: []string{"саммари", "что пропустил"}}

	tests := []struct{ text string; result bool }{
		{"саммари", true},
		{"что пропустил?", true},
		{"перешли в сессию", false},
	}
	for _, tt := range tests {
		got := r.IsSummaryCommand(tt.text)
		if got != tt.result {
			t.Errorf("IsSummaryCommand('%s') = %v, want %v", tt.text, got, tt.result)
		}
	}
}

func TestCommandRegistry_IsTopicIDQuery(t *testing.T) {
	r := &CommandRegistry{IDQueries: []string{"какой id", "id топика"}}

	tests := []struct{ text string; result bool }{
		{"какой id этого топика?", true},
		{"id топика", true},
		{"привет", false},
	}
	for _, tt := range tests {
		got := r.IsTopicIDQuery(tt.text)
		if got != tt.result {
			t.Errorf("IsTopicIDQuery('%s') = %v, want %v", tt.text, got, tt.result)
		}
	}
}

func TestCommandRegistry_Response(t *testing.T) {
	r := &CommandRegistry{Responses: map[string]string{
		"hello": "Привет, {name}!",
	}}

	got := r.Response("hello", map[string]string{"name": "Борис"})
	if got != "Привет, Борис!" {
		t.Errorf("expected 'Привет, Борис!', got '%s'", got)
	}

	got = r.Response("nonexistent", nil)
	if got != "nonexistent" {
		t.Errorf("expected key itself, got '%s'", got)
	}
}

func TestCommandRegistry_Load_MissingFile(t *testing.T) {
	_, err := LoadCommands("/nonexistent/commands.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
