package main

import (
	"testing"
)

func TestCommandRegistry_HelpRegistered(t *testing.T) {
	h, ok := Commands["help"]
	if !ok {
		t.Fatal("expected /help to be registered in Commands map")
	}
	if h.Handle == nil {
		t.Error("expected non-nil Handle for /help")
	}
}

func TestCommandRegistry_InitRegistered(t *testing.T) {
	h, ok := Commands["init"]
	if !ok {
		t.Fatal("expected /init to be registered")
	}
	if !h.AdminOnly {
		t.Error("expected /init to be admin_only")
	}
}

func TestCommandRegistry_AllCommandsPresent(t *testing.T) {
	expected := []string{"init", "help", "start", "status", "register",
		"зарегистрируй", "topics", "топики", "id",
		"resend", "перешли", "tag", "тег",
		"settings", "настройки", "summary", "саммари"}
	for _, name := range expected {
		if _, ok := Commands[name]; !ok {
			t.Errorf("expected command /%s to be registered", name)
		}
	}
}

func TestCommandRegistry_UnknownCommand(t *testing.T) {
	if _, ok := Commands["nonexistent"]; ok {
		t.Error("expected nonexistent command to not be registered")
	}
}

func TestCommandRegistry_AllHaveHandlers(t *testing.T) {
	for name, cmd := range Commands {
		if cmd.Handle == nil {
			t.Errorf("command /%s has nil Handle", name)
		}
	}
}
