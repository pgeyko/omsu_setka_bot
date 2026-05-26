package handler

import (
	"context"
	"testing"

	"omsu_bot/internal/db"

	"github.com/go-telegram/bot/models"
)

func TestCountWords(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 0},
		{"hello world", 0},
		{"a b c d e f g h i j k l m n o", 15},
		{"one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen", 15},
	}

	for _, tt := range tests {
		got := countWords(tt.input)
		if got != tt.want {
			t.Errorf("countWords(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestHandleMessage_TopiclessBypass(t *testing.T) {
	t.Parallel()
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	ctx := context.Background()
	// Insert active group
	err = database.CreateGroup(ctx, &db.Group{
		ChatID:   12345,
		Title:    "Test Chat",
		APIToken: "token",
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("failed to create group: %v", err)
	}

	h := &Handler{
		db: database,
	}

	// We pass a message with 15+ words which would normally trigger prefilter and classification
	msg := &models.Message{
		ID: 1,
		Chat: models.Chat{
			ID: 12345,
		},
		From: &models.User{
			ID:       999,
			Username: "testuser",
		},
		Text: "one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen",
	}

	update := &models.Update{
		Message: msg,
	}

	// Since there are 0 topics in the database, it should exit early
	// and record the action as "skipped_no_topics" in processed_messages.
	h.HandleMessage(ctx, nil, update)

	var action string
	err = database.QueryRowContext(ctx, "SELECT action FROM processed_messages WHERE message_id = 1").Scan(&action)
	if err != nil {
		t.Fatalf("failed to query processed message: %v", err)
	}

	if action != "skipped_no_topics" {
		t.Errorf("expected action 'skipped_no_topics', got '%s'", action)
	}
}
