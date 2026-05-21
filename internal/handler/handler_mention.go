package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

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
	if text == "" {
		return
	}

	// 1. Topic ID Query
	if h.cmd != nil && h.cmd.IsTopicIDQuery(text) {
		if msg.MessageThreadID != 0 {
			h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID,
				fmt.Sprintf("🆔 ID этого топика: %d", msg.MessageThreadID))
		} else {
			h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID,
				"📋 Это общий чат, у него нет ID топика.")
		}
		return
	}

	// 2. Run through Agent Orchestrator
	response, err := h.orchestrator.Run(ctx, msg.Chat.ID, msg.MessageThreadID, text, msg.From.Username)
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
