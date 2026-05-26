package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"omsu_bot/internal/schedule"

	"github.com/gofiber/fiber/v2"
)

const (
	maxClockSkew   = 5 * time.Minute
	eventDedupTTL  = 24 * time.Hour
)

type WebhookHandler struct {
	diffEngine       *schedule.DiffEngine
	db               *sql.DB
	scheduleSecret   string
	announceThreadID int
}

func NewWebhookHandler(diffEngine *schedule.DiffEngine, db *sql.DB, scheduleSecret string, announceThreadID int) *WebhookHandler {
	return &WebhookHandler{
		diffEngine:       diffEngine,
		db:               db,
		scheduleSecret:   scheduleSecret,
		announceThreadID: announceThreadID,
	}
}

func (h *WebhookHandler) Handle(c *fiber.Ctx) error {
	signature := c.Get("X-Webhook-Signature")
	if signature == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing signature"})
	}

	body := c.Body()

	// Replay protection: verify timestamp-signed HMAC (P1#11)
	timestamp := c.Get("X-Webhook-Timestamp")
	eventID := c.Get("X-Webhook-Event-ID")

	if timestamp != "" {
		if err := h.verifyTimestamp(timestamp); err != nil {
			slog.Warn("webhook timestamp rejected", "error", err, "timestamp", timestamp)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}

		if !h.verifyTimestampedHMAC(body, timestamp, signature) {
			slog.Warn("invalid webhook HMAC signature with timestamp")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid signature"})
		}
	} else {
		// Legacy: no-timestamp fallback
		if !h.verifyHMAC(body, signature) {
			slog.Warn("invalid webhook HMAC signature (legacy)")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid signature"})
		}
	}

	// Deduplicate by event_id (P1#11)
	if eventID != "" {
		isDuplicate, err := h.checkDedup(c.Context(), eventID)
		if err != nil {
			slog.Error("failed to check webhook dedup", "event_id", eventID, "error", err)
		} else if isDuplicate {
			slog.Info("duplicate webhook event, skipping", "event_id", eventID)
			return c.JSON(fiber.Map{"status": "duplicate"})
		}
	}

	var payload schedule.WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Error("failed to parse webhook payload", "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	// Only accept "group" entity_type for now (P0#4)
	if payload.EntityType != "" && payload.EntityType != "group" {
		slog.Warn("unsupported entity_type in webhook", "entity_type", payload.EntityType)
		return c.JSON(fiber.Map{"status": "skipped", "reason": "unsupported entity_type"})
	}

	slog.Info("received schedule webhook",
		"group_id", payload.GroupID,
		"entity_type", payload.EntityType,
		"event_id", eventID,
		"timestamp", timestamp,
		"changes", len(payload.Changes),
	)
	bodyPreview := string(body)
	if len(bodyPreview) > 500 {
		bodyPreview = bodyPreview[:500] + "..."
	}
	slog.Debug("webhook payload",
		"body", bodyPreview,
		"body_size", len(body),
	)

	// Resolve Telegram chat_id from omsu_group_id (P0#2: multi-tenancy)
	chatID, err := h.resolveChatID(c.Context(), payload.GroupID)
	if err != nil {
		slog.Error("failed to resolve chat_id for webhook group", "group_id", payload.GroupID, "error", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "group not found"})
	}

	if err := h.diffEngine.ProcessWebhook(c.Context(), &payload, chatID, h.announceThreadID); err != nil {
		slog.Error("failed to process webhook", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "processing failed"})
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *WebhookHandler) resolveChatID(ctx context.Context, omsuGroupID int) (int64, error) {
	var chatID int64
	err := h.db.QueryRowContext(ctx,
		"SELECT chat_id FROM groups WHERE omsu_group_id = ? AND is_active = 1", omsuGroupID,
	).Scan(&chatID)
	if err != nil {
		return 0, err
	}
	return chatID, nil
}

func (h *WebhookHandler) verifyHMAC(body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(h.scheduleSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (h *WebhookHandler) verifyTimestampedHMAC(body []byte, timestamp, signature string) bool {
	signed := timestamp + "." + string(body)
	mac := hmac.New(sha256.New, []byte(h.scheduleSecret))
	mac.Write([]byte(signed))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (h *WebhookHandler) verifyTimestamp(ts string) error {
	// Accept RFC3339 and Unix timestamp formats
	var t time.Time
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		// Try Unix timestamp (seconds or milliseconds)
		var sec int64
		if strings.Contains(ts, ".") {
			// Try parsing as millis
			var millis int64
			_, parseErr := fmt.Sscanf(ts, "%d", &millis)
			if parseErr != nil {
				return fmt.Errorf("invalid timestamp format: %s", ts)
			}
			// Heuristic: if > 1e12, assume millis
			if millis > 1e12 {
				t = time.UnixMilli(millis)
			} else {
				t = time.Unix(millis, 0)
			}
		} else {
			_, parseErr := fmt.Sscanf(ts, "%d", &sec)
			if parseErr != nil {
				return fmt.Errorf("invalid timestamp format: %s", ts)
			}
			t = time.Unix(sec, 0)
		}
	}

	skew := time.Since(t)
	if skew < 0 {
		skew = -skew
	}
	if skew > maxClockSkew {
		return fmt.Errorf("clock skew %v exceeds max %v", skew, maxClockSkew)
	}
	return nil
}

func (h *WebhookHandler) checkDedup(ctx context.Context, eventID string) (bool, error) {
	// Clean up expired events opportunistically
	h.cleanupExpiredEvents(ctx)

	// Check if event already processed
	var exists int
	err := h.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM webhook_events WHERE event_id = ?", eventID,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	if exists > 0 {
		return true, nil
	}

	// Record new event
	_, err = h.db.ExecContext(ctx,
		"INSERT INTO webhook_events (event_id, received_at, expires_at) VALUES (?, CURRENT_TIMESTAMP, datetime('now', '+24 hours'))",
		eventID,
	)
	if err != nil {
		return false, err
	}

	return false, nil
}

func (h *WebhookHandler) cleanupExpiredEvents(ctx context.Context) {
	_, err := h.db.ExecContext(ctx,
		"DELETE FROM webhook_events WHERE expires_at < datetime('now')",
	)
	if err != nil {
		slog.Warn("failed to cleanup expired webhook events", "error", err)
	}
}
