package handlers

import (
	"context"
	"database/sql"
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
		slog.Debug("mention handler: group not active", "chat_id", msg.Chat.ID, "err", err)
		return
	}

	text := msg.Text
	if text == "" {
		text = msg.Caption
	}

	if text == "" {
		return
	}

	slog.Debug("mention handler: processing", "msg_id", msg.ID, "text", text[:min(len(text), 200)], "chat", msg.Chat.ID, "thread", msg.MessageThreadID)

	// 2. Run through Agent Orchestrator
	var replyToMsgID int
	if msg.ReplyToMessage != nil {
		replyToMsgID = msg.ReplyToMessage.ID
	}
	response, err := h.orchestrator.RunWithContext(ctx, msg.Chat.ID, msg.MessageThreadID, text, msg.From.Username, msg.From.ID, msg.ID, replyToMsgID)
	if err != nil {
		slog.Error("agent orchestrator failed", "error", err)
		h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID, "Не удалось обработать запрос.")
		return
	}

	if response != "" {
		response = stripXMLTags(response)
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

var validHTMLTags = map[string]bool{
	"b": true, "i": true, "u": true, "s": true,
	"code": true, "pre": true, "a": true, "tg-spoiler": true,
}

func stripXMLTags(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '<' {
			close := strings.IndexByte(s[i:], '>')
			if close == -1 {
				result.WriteByte(s[i])
				i++
				continue
			}
			tag := s[i+1 : i+close]
			tagName := tag
			if strings.HasPrefix(tag, "/") {
				tagName = tag[1:]
			}
			if idx := strings.IndexAny(tagName, " ="); idx != -1 {
				tagName = tagName[:idx]
			}
			if validHTMLTags[tagName] {
				result.WriteString(s[i : i+close+1])
			}
			i += close + 1
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
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
