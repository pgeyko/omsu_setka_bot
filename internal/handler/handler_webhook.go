package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"

	"omsu_bot/internal/schedule"

	"github.com/gofiber/fiber/v2"
)

type WebhookHandler struct {
	diffEngine       *schedule.DiffEngine
	scheduleSecret   string
	announceThreadID int
}

func NewWebhookHandler(diffEngine *schedule.DiffEngine, scheduleSecret string, announceThreadID int) *WebhookHandler {
	return &WebhookHandler{
		diffEngine:       diffEngine,
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

	if !h.verifyHMAC(body, signature) {
		slog.Warn("invalid webhook HMAC signature")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid signature"})
	}

	var payload schedule.WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Error("failed to parse webhook payload", "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	slog.Info("received schedule webhook",
		"group_id", payload.GroupID,
		"changes", len(payload.Changes),
	)
	slog.Debug("webhook payload",
		"body", string(body),
	)

	if err := h.diffEngine.ProcessWebhook(c.Context(), &payload, h.announceThreadID); err != nil {
		slog.Error("failed to process webhook", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "processing failed"})
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *WebhookHandler) verifyHMAC(body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(h.scheduleSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
