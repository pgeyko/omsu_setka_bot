package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"omsu_bot/internal/db"
	"omsu_bot/internal/schedule"

	"github.com/gofiber/fiber/v2"
)

func setupWebhookTest(t *testing.T) (*WebhookHandler, *db.DB, func()) {
	t.Helper()
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := database.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	diffEngine := schedule.NewDiffEngine(database.DB, nil, nil, nil)
	handler := NewWebhookHandler(diffEngine, database.DB, "test-secret")

	cleanup := func() { database.Close() }
	return handler, database, cleanup
}

func makeSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func makeTimestampedSignature(secret, timestamp string, body []byte) string {
	signed := timestamp + "." + string(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signed))
	return hex.EncodeToString(mac.Sum(nil))
}

func seedTestGroup(t *testing.T, handler *WebhookHandler, groupID int, chatID int64) {
	t.Helper()
	ctx := context.Background()
	_, err := handler.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO groups (chat_id, title, api_token, omsu_group_id, is_active) VALUES (?, ?, ?, ?, 1)`,
		chatID, "Test Group", "token", groupID,
	)
	if err != nil {
		t.Fatalf("failed to seed group: %v", err)
	}
}

func TestWebhookHandler_HMAC_Valid(t *testing.T) {
	t.Parallel()
	handler, _, cleanup := setupWebhookTest(t)
	defer cleanup()
	seedTestGroup(t, handler, 1, -1001)

	app := fiber.New()
	app.Post("/webhook/schedule", handler.Handle)

	payload := schedule.WebhookPayload{
		Type:    "change",
		GroupID: 1,
		Changes: []schedule.Change{},
	}
	body, _ := json.Marshal(payload)
	ts := time.Now().UTC().Format(time.RFC3339)
	sig := makeTimestampedSignature("test-secret", ts, body)

	req := httptest.NewRequest("POST", "/webhook/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)
	req.Header.Set("X-Webhook-Timestamp", ts)

	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_HMAC_InvalidSignature(t *testing.T) {
	t.Parallel()
	handler, _, cleanup := setupWebhookTest(t)
	defer cleanup()

	app := fiber.New()
	app.Post("/webhook/schedule", handler.Handle)

	body := []byte(`{"type":"change"}`)
	ts := time.Now().UTC().Format(time.RFC3339)

	req := httptest.NewRequest("POST", "/webhook/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", "invalid-signature")
	req.Header.Set("X-Webhook-Timestamp", ts)

	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_HMAC_MissingSignature(t *testing.T) {
	t.Parallel()
	handler, _, cleanup := setupWebhookTest(t)
	defer cleanup()

	app := fiber.New()
	app.Post("/webhook/schedule", handler.Handle)

	body := []byte(`{"type":"change"}`)
	ts := time.Now().UTC().Format(time.RFC3339)

	req := httptest.NewRequest("POST", "/webhook/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Timestamp", ts)

	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_HMAC_MissingTimestamp(t *testing.T) {
	t.Parallel()
	handler, _, cleanup := setupWebhookTest(t)
	defer cleanup()

	app := fiber.New()
	app.Post("/webhook/schedule", handler.Handle)

	body := []byte(`{"type":"change"}`)
	sig := makeSignature("test-secret", body)

	req := httptest.NewRequest("POST", "/webhook/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)

	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 400 {
		t.Errorf("expected 400 for missing timestamp, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_TimestampSkew_Valid(t *testing.T) {
	t.Parallel()
	handler, _, cleanup := setupWebhookTest(t)
	defer cleanup()
	seedTestGroup(t, handler, 1, -1001)

	app := fiber.New()
	app.Post("/webhook/schedule", handler.Handle)

	payload := schedule.WebhookPayload{Type: "change", GroupID: 1}
	body, _ := json.Marshal(payload)
	ts := time.Now().UTC().Format(time.RFC3339)
	sig := makeTimestampedSignature("test-secret", ts, body)

	req := httptest.NewRequest("POST", "/webhook/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)
	req.Header.Set("X-Webhook-Timestamp", ts)

	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for valid timestamp, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_TimestampSkew_Exceeded(t *testing.T) {
	t.Parallel()
	handler, _, cleanup := setupWebhookTest(t)
	defer cleanup()

	app := fiber.New()
	app.Post("/webhook/schedule", handler.Handle)

	payload := schedule.WebhookPayload{Type: "change", GroupID: 1}
	body, _ := json.Marshal(payload)
	// Timestamp 10 minutes in the past — exceeds maxClockSkew (5 min)
	ts := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)
	sig := makeTimestampedSignature("test-secret", ts, body)

	req := httptest.NewRequest("POST", "/webhook/schedule", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)
	req.Header.Set("X-Webhook-Timestamp", ts)

	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 401 {
		t.Errorf("expected 401 for expired timestamp, got %d", resp.StatusCode)
	}
}

func TestWebhookHandler_EventDedup_New(t *testing.T) {
	t.Parallel()
	handler, database, cleanup := setupWebhookTest(t)
	defer cleanup()

	ctx := context.Background()
	// Verify event doesn't exist yet
	dup, err := handler.checkDedup(ctx, "event-001")
	if err != nil {
		t.Fatalf("checkDedup failed: %v", err)
	}
	if dup {
		t.Error("expected false for new event")
	}

	// Verify it's now stored
	dup, err = handler.checkDedup(ctx, "event-001")
	if err != nil {
		t.Fatalf("checkDedup failed: %v", err)
	}
	if !dup {
		t.Error("expected true for duplicate event")
	}

	// Cleanup
	database.Close()
}

func TestWebhookHandler_EventDedup_Duplicate(t *testing.T) {
	t.Parallel()
	handler, database, cleanup := setupWebhookTest(t)
	defer cleanup()

	ctx := context.Background()

	// First event
	dup, err := handler.checkDedup(ctx, "event-002")
	if err != nil { t.Fatalf("checkDedup: %v", err) }
	if dup { t.Error("expected false for first event") }

	// Same event again
	dup, err = handler.checkDedup(ctx, "event-002")
	if err != nil { t.Fatalf("checkDedup: %v", err) }
	if !dup { t.Error("expected true for duplicate event") }

	database.Close()
}

func TestWebhookHandler_EventDedup_Expired(t *testing.T) {
	t.Parallel()
	handler, database, cleanup := setupWebhookTest(t)
	defer cleanup()

	ctx := context.Background()

	// Insert an expired event directly
	database.ExecContext(ctx,
		`INSERT INTO webhook_events (event_id, received_at, expires_at) VALUES (?, datetime('now', '-2 days'), datetime('now', '-1 day'))`,
		"event-expired",
	)

	// Verify it is treated as new (dedup returns false)
	dup, err := handler.checkDedup(ctx, "event-expired")
	if err != nil { t.Fatalf("checkDedup: %v", err) }
	if dup {
		t.Error("expected false for expired event (should be cleaned up)")
	}

	database.Close()
}

func TestWebhookHandler_ChatIDResolution_Found(t *testing.T) {
	t.Parallel()
	handler, database, cleanup := setupWebhookTest(t)
	defer cleanup()

	ctx := context.Background()
	database.ExecContext(ctx,
		`INSERT INTO groups (chat_id, title, api_token, omsu_group_id, is_active) VALUES (?, ?, ?, ?, 1)`,
		int64(-100123), "Test Group", "token", 42,
	)

	chatID, threadID, err := handler.resolveGroupInfo(ctx, 42)
	if err != nil { t.Fatalf("resolveGroupInfo: %v", err) }
	if chatID != -100123 {
		t.Errorf("expected -100123, got %d", chatID)
	}
	if threadID != 0 {
		t.Errorf("expected 0, got %d", threadID)
	}
}

func TestWebhookHandler_ChatIDResolution_NotFound(t *testing.T) {
	t.Parallel()
	handler, _, cleanup := setupWebhookTest(t)
	defer cleanup()

	ctx := context.Background()
	_, _, err := handler.resolveGroupInfo(ctx, 999)
	if err == nil {
		t.Error("expected error for non-existent group")
	}
}
