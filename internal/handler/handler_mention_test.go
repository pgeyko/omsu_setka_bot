package handler

import (
	"testing"
)

func TestCommandRegistry_NewAndResponse(t *testing.T) {
	r := NewCommandRegistry()
	if r == nil {
		t.Fatal("expected registry to be created")
	}

	got := r.Response("register_exists", nil)
	if got != "⚠️ Этот топик уже зарегистрирован." {
		t.Errorf("expected register_exists translation, got '%s'", got)
	}

	got = r.Response("hello", map[string]string{"name": "Борис"})
	if got != "hello" {
		t.Errorf("expected key itself for nonexistent hello key, got '%s'", got)
	}

	customRegistry := &CommandRegistry{Responses: map[string]string{
		"hello": "Привет, {name}!",
	}}

	got = customRegistry.Response("hello", map[string]string{"name": "Борис"})
	if got != "Привет, Борис!" {
		t.Errorf("expected 'Привет, Борис!', got '%s'", got)
	}
}
