package handler

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/go-telegram/bot/models"

	"omsu_bot/internal/db"
	"omsu_bot/internal/telegram"
)

func TestSettingsHandler_Authorization(t *testing.T) {
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
	// Create group
	_ = database.CreateGroup(ctx, &db.Group{
		ChatID:   12345,
		Title:    "Test Group",
		APIToken: "token",
		IsActive: true,
	})

	// Add superadmin
	_ = database.AddSuperadmin(ctx, 777, "Superadmin Note")

	sessionStore := telegram.NewSessionStore()
	adminCache := telegram.NewAdminCache(nil)

	h := NewSettingsHandler(database, sessionStore, adminCache, "", "", "", "", nil, nil)

	// 1. Superadmin should be authorized
	if !h.isAuthorized(ctx, 12345, 777) {
		t.Error("expected superadmin to be authorized")
	}

	// 2. Random user is not authorized (admin cache will query Telegram API, but since bot is nil it will return false)
	if h.isAuthorized(ctx, 12345, 888) {
		t.Error("expected random user to not be authorized")
	}
}

func TestSettingsHandler_LoadSaveFeatures(t *testing.T) {
	t.Parallel()
	chatID := int64(999999)
	defer os.RemoveAll(fmt.Sprintf("data/groups/%d", chatID))

	database, _ := db.New(":memory:")
	defer database.Close()

	sessionStore := telegram.NewSessionStore()
	h := 	NewSettingsHandler(database, sessionStore, nil, "", "", "", "", nil, nil)

	// Test default features
	features := h.LoadFeatures(chatID)
	if !features["enable_schedule"] || !features["enable_summary"] || !features["enable_moderation"] || !features["enable_voice_transcription"] || !features["enable_photo_processing"] {
		t.Error("expected default features to be true")
	}

	// Test saving and loading features
	features["enable_summary"] = false
	err := h.saveFeatures(chatID, features)
	if err != nil {
		t.Fatalf("failed to save features: %v", err)
	}

	loaded := h.LoadFeatures(chatID)
	if loaded["enable_summary"] {
		t.Error("expected enable_summary to be false after save")
	}
	if !loaded["enable_schedule"] {
		t.Error("expected enable_schedule to remain true")
	}
}

func TestSettingsHandler_HandleAdminInput(t *testing.T) {
	t.Parallel()
	database, _ := db.New(":memory:")
	defer database.Close()

	sessionStore := telegram.NewSessionStore()
	h := 	NewSettingsHandler(database, sessionStore, nil, "", "", "", "", nil, nil)

	chatID := int64(888888)
	userID := int64(111)
	defer os.RemoveAll(fmt.Sprintf("data/groups/%d", chatID))

	ctx := context.Background()

	// Test waiting for KB
	msgKB := &models.Message{
		Chat: models.Chat{ID: chatID},
		From: &models.User{ID: userID},
		Text: "This is test KB content.",
	}
	updateKB := &models.Update{Message: msgKB}

	h.HandleAdminInput(ctx, nil, updateKB, telegram.StateWaitingForKB)

	kbContent, err := os.ReadFile(fmt.Sprintf("data/groups/%d/knowledge_base.txt", chatID))
	if err != nil {
		t.Fatalf("failed to read kb file: %v", err)
	}
	if string(kbContent) != "This is test KB content." {
		t.Errorf("expected '%s', got '%s'", "This is test KB content.", string(kbContent))
	}

	// Test waiting for Persona
	msgPersona := &models.Message{
		Chat: models.Chat{ID: chatID},
		From: &models.User{ID: userID},
		Text: "This is test Persona markdown.",
	}
	updatePersona := &models.Update{Message: msgPersona}

	h.HandleAdminInput(ctx, nil, updatePersona, telegram.StateWaitingForPersona)

	personaContent, err := os.ReadFile(fmt.Sprintf("data/groups/%d/persona.md", chatID))
	if err != nil {
		t.Fatalf("failed to read persona file: %v", err)
	}
	if string(personaContent) != "This is test Persona markdown." {
		t.Errorf("expected '%s', got '%s'", "This is test Persona markdown.", string(personaContent))
	}
}
