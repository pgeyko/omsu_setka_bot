package handlers

import (
	"context"
	"database/sql"
	"log/slog"

	"omsu_bot/internal/agent"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type MentionHandler struct {
	orchestrator *agent.AgentOrchestrator
	db           *sql.DB
	botUsername  string
}

func NewMentionHandler(orchestrator *agent.AgentOrchestrator, db *sql.DB, botUsername string) *MentionHandler {
	return &MentionHandler{
		orchestrator: orchestrator,
		db:           db,
		botUsername:  botUsername,
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

	if text == "" {
		return
	}

	// 2. Run through Agent Orchestrator
	response, err := h.orchestrator.Run(ctx, msg.Chat.ID, msg.MessageThreadID, text, msg.From.Username, msg.From.ID)
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
