package util

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestMessageSender(t *testing.T) {
	tests := []struct {
		name     string
		msg      *models.Message
		wantID   int64
		wantUser string
		wantOK   bool
	}{
		{"nil message", nil, 0, "", false},
		{"no from", &models.Message{}, 0, "", false},
		{"with user", &models.Message{From: &models.User{ID: 123, Username: "testuser"}}, 123, "testuser", true},
		{"with user no username", &models.Message{From: &models.User{ID: 456}}, 456, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, user, ok := MessageSender(tt.msg)
			if id != tt.wantID || user != tt.wantUser || ok != tt.wantOK {
				t.Errorf("MessageSender() = (%d, %q, %t), want (%d, %q, %t)",
					id, user, ok, tt.wantID, tt.wantUser, tt.wantOK)
			}
		})
	}
}

func TestStripMarkdown(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello **world**", "Hello world"},
		{"`code` here", "code here"},
		{"__underline__ text", "underline text"},
		{"**bold** and `code`", "bold and code"},
		{"no markdown", "no markdown"},
		{"single * not stripped", "single * not stripped"},
		{"single _ not stripped", "single _ not stripped"},
		{"nested **bold __and__**", "nested bold and"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := StripMarkdown(tt.input); got != tt.want {
				t.Errorf("StripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
