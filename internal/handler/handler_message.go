package handlers

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"

	"omsu_bot/internal/buffer"
	"omsu_bot/internal/classifier"
	"omsu_bot/internal/forwarder"
	"omsu_bot/internal/telegram"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Handler struct {
	classifier    *classifier.Classifier
	forwarder     *forwarder.Forwarder
	db            *sql.DB
	buffer        *buffer.SummaryBuffer
	usernameCache *telegram.UsernameCache
}

func NewHandler(classifier *classifier.Classifier, forwarder *forwarder.Forwarder, db *sql.DB, buffer *buffer.SummaryBuffer, usernameCache *telegram.UsernameCache) *Handler {
	return &Handler{
		classifier:    classifier,
		forwarder:     forwarder,
		db:            db,
		buffer:        buffer,
		usernameCache: usernameCache,
	}
}

func (h *Handler) HandleMessage(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	msg := update.Message
	if h.usernameCache != nil && msg.From != nil {
		h.usernameCache.Store(msg.From.Username, msg.From.ID)
	}

	var active int
	if err := h.db.QueryRowContext(ctx, "SELECT is_active FROM groups WHERE chat_id = ?", msg.Chat.ID).Scan(&active); err != nil || active != 1 {
		return
	}
	slog.Debug("tg message",
		"msg_id", msg.ID,
		"from", msg.From.ID,
		"username", msg.From.Username,
		"text", truncate(msg.Text, 500),
		"chat", msg.Chat.ID,
		"thread", msg.MessageThreadID,
	)

	text := msg.Text
	if text == "" {
		text = msg.Caption
	}

	if h.buffer != nil && text != "" {
		h.buffer.Push(msg.Chat.ID, msg.MessageThreadID, msg.ID, msg.From.Username, text)
	}

	if h.isProcessed(ctx, msg.ID, msg.Chat.ID) {
		return
	}

	var activeTopicsCount int
	err := h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM topics WHERE group_id = ? AND is_active = 1", msg.Chat.ID).Scan(&activeTopicsCount)
	if err != nil || activeTopicsCount == 0 {
		h.markProcessed(ctx, msg.ID, msg.Chat.ID, msg.MessageThreadID, "skipped_no_topics", 0)
		return
	}

	if !h.prefilter(msg) {
		h.markProcessed(ctx, msg.ID, msg.Chat.ID, msg.MessageThreadID, "skipped", 0)
		return
	}

	var fileID string
	if len(msg.Photo) > 0 {
		fileID = msg.Photo[len(msg.Photo)-1].FileID
	} else if msg.Document != nil {
		fileID = msg.Document.FileID
	}

	if text == "" && fileID == "" {
		h.markProcessed(ctx, msg.ID, msg.Chat.ID, msg.MessageThreadID, "skipped", 0)
		return
	}

	result, err := h.classifier.ClassifyMessage(ctx, msg.Chat.ID, text, fileID)
	if err != nil {
		slog.Warn("classification skipped", "reason", err, "msg_id", msg.ID)
		h.markProcessed(ctx, msg.ID, msg.Chat.ID, msg.MessageThreadID, "skipped", 0)
		return
	}

	if result.Confidence < 0.75 || result.Topic == "" {
		h.markProcessed(ctx, msg.ID, msg.Chat.ID, msg.MessageThreadID, "low_confidence", 0)
		return
	}

	targetThreadID, err := h.lookupThreadID(ctx, msg.Chat.ID, result.Topic)
	if err != nil || targetThreadID == 0 {
		slog.Warn("target topic not found", "topic", result.Topic)
		return
	}

	fromTopicName := ""
	if msg.MessageThreadID != 0 {
		fromTopicName = h.getTopicName(ctx, msg.Chat.ID, msg.MessageThreadID)
	}

	_, err = h.forwarder.Duplicate(ctx, msg.Chat.ID, msg.MessageThreadID, targetThreadID, msg.From.Username, fromTopicName, result.Hashtags, msg.ID)
	if err != nil {
		slog.Error("forward failed", "error", err, "msg_id", msg.ID)
		return
	}

	h.forwarder.ReplyWithLink(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID, result.Topic)

	h.markProcessed(ctx, msg.ID, msg.Chat.ID, msg.MessageThreadID, "forwarded", targetThreadID)
}

func (h *Handler) prefilter(msg *models.Message) bool {
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}

	if text != "" && countWords(text) >= 15 {
		return true
	}

	if len(msg.Photo) > 0 || msg.Document != nil {
		return true
	}

	return false
}

func (h *Handler) isProcessed(ctx context.Context, messageID int, chatID int64) bool {
	var count int
	err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM processed_messages WHERE message_id = ? AND chat_id = ?`,
		messageID, chatID,
	).Scan(&count)
	return err == nil && count > 0
}

func (h *Handler) markProcessed(ctx context.Context, messageID int, chatID int64, threadID int, action string, targetThreadID int) {
	var threadIDPtr *int
	if threadID != 0 {
		threadIDPtr = &threadID
	}
	h.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO processed_messages (message_id, chat_id, thread_id, action, target_thread_id, processed_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		messageID, chatID, threadIDPtr, action, targetThreadID,
	)
}

func (h *Handler) getTopicName(ctx context.Context, chatID int64, threadID int) string {
	var name string
	h.db.QueryRowContext(ctx, `SELECT name FROM topics WHERE group_id = ? AND tg_thread_id = ?`, chatID, threadID).Scan(&name)
	return name
}

func (h *Handler) lookupThreadID(ctx context.Context, chatID int64, slug string) (int, error) {
	var tgThreadID int
	err := h.db.QueryRowContext(ctx,
		`SELECT tg_thread_id FROM topics WHERE group_id = ? AND slug = ? AND is_active = 1`, chatID, slug,
	).Scan(&tgThreadID)
	return tgThreadID, err
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func countWords(s string) int {
	if len(s) < 15 {
		return 0
	}
	return len(strings.Fields(s))
}
