package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"omsu_bot/internal/llm"
	"omsu_bot/internal/telegram"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type SummaryHandler struct {
	llmClient   *llm.Client
	prompts     *llm.PromptRegistry
	db          *sql.DB
	bot         *tgbot.Bot
	groupID     int64
	buffer      *SummaryBuffer
	botUsername string
	adminCache  *telegram.AdminCache
}

func NewSummaryHandler(llmClient *llm.Client, prompts *llm.PromptRegistry, db *sql.DB, bot *tgbot.Bot, groupID int64, buffer *SummaryBuffer, botUsername string, adminCache *telegram.AdminCache) *SummaryHandler {
	return &SummaryHandler{
		llmClient:   llmClient,
		prompts:     prompts,
		db:          db,
		bot:         bot,
		groupID:     groupID,
		buffer:      buffer,
		botUsername: botUsername,
		adminCache:  adminCache,
	}
}

func (h *SummaryHandler) Handle(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Chat.ID != h.groupID {
		return
	}

	msg := update.Message
	userID := msg.From.ID

	allowed, err := h.checkPermission(ctx, userID)
	if err != nil {
		slog.Error("permission check failed", "error", err)
		return
	}
	if !allowed {
		h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"⛔ У тебя нет прав на эту команду.")
		return
	}

	if !h.checkRateLimit(ctx, userID, msg.Chat.ID) {
		h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"⏳ Саммари можно запрашивать раз в 30 минут.")
		return
	}

	threadID := msg.MessageThreadID

	messages := h.buffer.GetMessages(threadID)
	if messages == "" && threadID != 0 {
		messages = h.buffer.GetMessages(0)
	}
	if messages == "" {
		h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"📭 Нет сообщений для саммари. Сообщения копятся с момента последнего запуска бота — если он недавно перезапускался, буфер пуст.")
		return
	}

	summaryPrompt := fmt.Sprintf(
		"Сделай краткое саммари последних сообщений в топике. "+
			"Выдели главные темы, вопросы, дедлайны. "+
			"Напиши на русском, коротко и по делу.\n\n"+
			"Сообщения:\n%s", messages)

	resp, err := h.llmClient.Call(ctx, "summary", "", summaryPrompt, false)
	if err != nil {
		slog.Error("summary generation failed", "error", err)
		h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"Не удалось сгенерировать саммари. Попробуй позже.")
		return
	}

	summaryText := fmt.Sprintf("📋 Саммари:\n\n%s", resp.Content)

	_, err = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          msg.Chat.ID,
		MessageThreadID: msg.MessageThreadID,
		Text:            summaryText,
		ParseMode:       models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: msg.ID,
		},
	})
	if err != nil {
		slog.Error("failed to post summary", "error", err)
	}
}

func (h *SummaryHandler) checkPermission(ctx context.Context, userID int64) (bool, error) {
	role := "everyone"
	err := h.db.QueryRowContext(ctx,
		`SELECT COALESCE(allowed_role, 'everyone') FROM command_permissions WHERE command = 'summary'`,
	).Scan(&role)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}

	if role == "everyone" {
		return true, nil
	}

	if role == "admin" {
		return h.adminCache.IsAdmin(ctx, userID), nil
	}

	return true, nil
}

func (h *SummaryHandler) checkRateLimit(ctx context.Context, userID int64, chatID int64) bool {
	var count int
	err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM summary_requests
		 WHERE user_id = ? AND chat_id = ? AND requested_at > datetime('now', '-30 minutes')`,
		userID, chatID,
	).Scan(&count)
	if err != nil {
		return true
	}

	if count > 0 {
		return false
	}

	h.db.ExecContext(ctx,
		`INSERT INTO summary_requests (user_id, chat_id, requested_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP)`,
		userID, chatID,
	)
	return true
}

func (h *SummaryHandler) reply(ctx context.Context, b *tgbot.Bot, chatID int64, threadID int, replyToID int, text string) {
	b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          chatID,
		MessageThreadID: threadID,
		Text:            text,
		ReplyParameters: &models.ReplyParameters{
			MessageID: replyToID,
		},
	})
}

func isSummaryCommand(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "саммари") ||
		strings.Contains(lower, "что пропустил") ||
		strings.Contains(lower, "summary") ||
		strings.Contains(lower, "what did i miss")
}
