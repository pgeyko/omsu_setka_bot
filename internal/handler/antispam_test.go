package handlers

import (
	"context"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestAntispam_CheckFloodAndLinks(t *testing.T) {
	a := NewAntispam()

	msg := &models.Message{
		Chat: models.Chat{ID: 123},
		From: &models.User{ID: 456, Username: "testuser"},
		Text: "Hello, world!",
	}

	// 1. Should not block normal message
	if blocked := a.CheckFloodAndLinks(context.Background(), nil, msg); blocked {
		t.Error("expected first message to not be blocked")
	}

	// 2. Flood check: 6 messages in a row should trigger flood control
	for i := 0; i < 4; i++ {
		if blocked := a.CheckFloodAndLinks(context.Background(), nil, msg); blocked {
			t.Errorf("message %d should not be blocked", i+2)
		}
	}

	// The 6th message should trigger flood block
	if blocked := a.CheckFloodAndLinks(context.Background(), nil, msg); !blocked {
		t.Error("expected 6th message to be blocked by flood control")
	}
}
