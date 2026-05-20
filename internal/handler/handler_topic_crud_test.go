package handlers

import (
	"testing"
)

func TestMakeSlug(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Сессия", "sessiya"},
		{"Лекции и Практика", "lektsii_i_praktika"},
		{"ИВТ-101", "ivt_101"},
		{"test", "test"},
	}

	for _, tt := range tests {
		result := makeSlug(tt.input)
		if result != tt.expected {
			t.Errorf("makeSlug('%s') = '%s', expected '%s'", tt.input, result, tt.expected)
		}
	}
}

func TestCommandRegistry_HappyPath(t *testing.T) {
	r := &CommandRegistry{
		TopicCommands: []string{"создай топик", "закрой топик"},
		SummaryCmds:   []string{"саммари", "что пропустил"},
		IDQueries:     []string{"какой id", "id топика"},
		Responses: map[string]string{
			"hello": "Привет, {name}!",
		},
	}

	if !r.IsTopicCommand("создай топик Практика") {
		t.Error("expected topic command match")
	}
	if !r.IsSummaryCommand("саммари") {
		t.Error("expected summary command match")
	}
	if !r.IsTopicIDQuery("какой ID этого топика?") {
		t.Error("expected ID query match")
	}
	got := r.Response("hello", map[string]string{"name": "Бот"})
	if got != "Привет, Бот!" {
		t.Errorf("expected 'Привет, Бот!', got '%s'", got)
	}
}
