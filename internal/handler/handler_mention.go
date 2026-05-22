package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"omsu_bot/internal/agent"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type MentionHandler struct {
	orchestrator *agent.AgentOrchestrator
	db           *sql.DB
	botUsername  string
	cmd          *CommandRegistry
}

func NewMentionHandler(orchestrator *agent.AgentOrchestrator, db *sql.DB, botUsername string, cmd *CommandRegistry) *MentionHandler {
	return &MentionHandler{
		orchestrator: orchestrator,
		db:           db,
		botUsername:  botUsername,
		cmd:          cmd,
	}
}

func (h *MentionHandler) Handle(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	msg := update.Message

	var active int
	if err := h.db.QueryRowContext(ctx, "SELECT is_active FROM groups WHERE chat_id = ?", msg.Chat.ID).Scan(&active); err != nil || active != 1 {
		return
	}

	text := msg.Text
	if text == "" {
		text = msg.Caption
	}

	var payload string
	if msg.ReplyToMessage != nil {
		replyText := msg.ReplyToMessage.Text
		if replyText == "" {
			replyText = msg.ReplyToMessage.Caption
		}
		payload = fmt.Sprintf(
			"Команда пользователя: %s\n\nКонтент исходного сообщения для пересылки:\n%s",
			text,
			replyText,
		)
	} else {
		payload = text
	}

	if payload == "" {
		return
	}

	// Explicit rule-based routing to forward classification
	lowerText := strings.ToLower(text)
	if strings.Contains(lowerText, "перешли") ||
		strings.Contains(lowerText, "скинь в") ||
		strings.Contains(lowerText, "отправь в") ||
		strings.Contains(lowerText, "закинь в") {
		// Replace text payload to force a "forward message" tool behavior if available,
		// but since we are handling mention via orchestrator, we can prepend a clear instruction
		payload = "ПРИНУДИТЕЛЬНОЕ ДЕЙСТВИЕ: Это запрос на пересылку сообщения. Используй ТОЛЬКО инструмент forward_message. " + payload
	}

	// 2. Run through Agent Orchestrator
	response, err := h.orchestrator.Run(ctx, msg.Chat.ID, msg.MessageThreadID, payload, msg.From.Username, msg.From.ID)
	if err != nil {
		slog.Error("agent orchestrator failed", "error", err)
		h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID, "Не удалось обработать запрос.")
		return
	}

	if response != "" {
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          msg.Chat.ID,
			MessageThreadID: msg.MessageThreadID,
			Text:            response,
			ParseMode:       models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: msg.ID,
			},
		})
	}
}

func (h *MentionHandler) reply(ctx context.Context, b *tgbot.Bot, chatID int64, threadID int, replyToID int, text string) {
	b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          chatID,
		MessageThreadID: threadID,
		Text:            text,
		ReplyParameters: &models.ReplyParameters{
			MessageID: replyToID,
		},
	})
}
