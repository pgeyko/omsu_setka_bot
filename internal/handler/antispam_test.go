package handler

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

func TestAntispam_HandleCallbackQuery(t *testing.T) {
	// Test 1: Callback with moderation / captcha disabled should bypass verify and return cleanly
	loader := func(chatID int64) map[string]bool {
		return map[string]bool{
			"enable_moderation": false,
			"enable_captcha":    false,
		}
	}
	a := NewAntispam(loader)

	update := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "111",
			From: models.User{ID: 456},
			Data: "captcha:456:10:10",
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{
					Chat: models.Chat{ID: 123},
					ID:   789,
				},
			},
		},
	}

	// Should execute without panic (since b is nil, Bot operations are skipped but features check returns cleanly)
	a.HandleCallbackQuery(context.Background(), nil, update)

	// Test 2: Callback with invalid user ID format or mismatch, with bot nil
	updateMismatch := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "112",
			From: models.User{ID: 999}, // different user
			Data: "captcha:456:10:10",
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{
					Chat: models.Chat{ID: 123},
					ID:   789,
				},
			},
		},
	}
	
	// With moderation enabled, mismatch user ID should return (skipped, no panic)
	aEnabled := NewAntispam() // defaults are enabled
	aEnabled.HandleCallbackQuery(context.Background(), nil, updateMismatch)
}
