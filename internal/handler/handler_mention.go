package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"omsu_bot/internal/llm"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type MentionHandler struct {
	llmClient   *llm.Client
	prompts     *llm.PromptRegistry
	db          *sql.DB
	groupID     int64
	topicCRUD   *TopicCRUD
	summary     *SummaryHandler
	scheduleQ   *ScheduleQueryHandler
	botUsername string
	personaName string
	cmd         *CommandRegistry
}

func NewMentionHandler(llmClient *llm.Client, prompts *llm.PromptRegistry, db *sql.DB, groupID int64, topicCRUD *TopicCRUD, summary *SummaryHandler, scheduleQ *ScheduleQueryHandler, botUsername string, personaName string, cmd *CommandRegistry) *MentionHandler {
	return &MentionHandler{
		llmClient:   llmClient,
		prompts:     prompts,
		db:          db,
		groupID:     groupID,
		topicCRUD:   topicCRUD,
		summary:     summary,
		scheduleQ:   scheduleQ,
		botUsername: botUsername,
		personaName: personaName,
		cmd:         cmd,
	}
}

type ForwardIntent struct {
	Intent      string  `json:"intent"`
	TargetTopic string  `json:"target_topic"`
	Confidence  float64 `json:"confidence"`
}

func (h *MentionHandler) Handle(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Chat.ID != h.groupID {
		return
	}

	msg := update.Message

	text := msg.Text
	if text == "" {
		text = msg.Caption
	}
	if text == "" {
		return
	}

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

	if h.scheduleQ != nil && h.cmd != nil && h.cmd.IsScheduleQuery(text) {
		h.scheduleQ.Handle(ctx, b, update)
		return
	}

	if h.summary != nil && h.cmd != nil && h.cmd.IsSummaryCommand(text) {
		h.summary.Handle(ctx, b, update)
		return
	}

	if h.topicCRUD != nil && h.cmd != nil && h.cmd.IsTopicCommand(text) {
		h.topicCRUD.Handle(ctx, update)
		return
	}

	forwardMsgID := msg.ID
	if msg.ReplyToMessage != nil {
		forwardMsgID = msg.ReplyToMessage.ID
	}

	prompt := h.prompts.Get("forward_intent")
	topicList := h.getTopics(ctx)
	prompt = strings.ReplaceAll(prompt, "{topics}", topicList)
	prompt = strings.ReplaceAll(prompt, "{text}", text)

	resp, err := h.llmClient.Call(ctx, "forward_intent", "", prompt, false)
	if err != nil {
		slog.Error("forward intent parsing failed", "error", err)
		h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID, "Не удалось распознать команду. Попробуй ещё раз.")
		return
	}

	var intent ForwardIntent
	if err := json.Unmarshal([]byte(llm.ExtractJSON(resp.Content)), &intent); err != nil {
		slog.Error("failed to parse forward intent", "error", err)
		if h.tryPersonaChat(ctx, b, msg, text) {
			return
		}
		botMention := "@bot"
		if h.botUsername != "" {
			botMention = "@" + h.botUsername
		}
		h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID, fmt.Sprintf("Не понял команду. Попробуй: %s и напиши команду", botMention))
		return
	}

	if intent.Confidence < 0.75 || intent.Intent != "forward" {
		if h.tryPersonaChat(ctx, b, msg, text) {
			return
		}
		h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			h.cmd.Response("topic_not_found_simple", map[string]string{
				"topics": h.getAvailableTopics(ctx),
			}))
		return
	}

	targetThreadID := h.resolveTopic(ctx, intent.TargetTopic)
	if targetThreadID == 0 {
		available := h.getAvailableTopics(ctx)
		h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			h.cmd.Response("topic_not_found_simple", map[string]string{
				"topics": available,
			}))
		return
	}

	fromChatID := fmt.Sprintf("%d", msg.Chat.ID)
	_, err = b.CopyMessage(ctx, &tgbot.CopyMessageParams{
		ChatID:          msg.Chat.ID,
		FromChatID:      fromChatID,
		MessageID:       forwardMsgID,
		MessageThreadID: targetThreadID,
	})
	if err != nil {
		slog.Error("copy message failed", "error", err)
		h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID, "Ошибка при пересылке.")
		return
	}

	h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID,
		fmt.Sprintf("↗️ Продублировал в «%s»", intent.TargetTopic))
	return
}

func (h *MentionHandler) tryPersonaChat(ctx context.Context, b *tgbot.Bot, msg *models.Message, text string) bool {
	if h.llmClient == nil || h.cmd == nil || !h.cmd.IsPersonaMention(text, h.personaName) {
		return false
	}
	prompt := h.prompts.Get("persona_chat")
	prompt = strings.ReplaceAll(prompt, "{text}", text)
	resp, err := h.llmClient.Call(ctx, "persona_chat", "", prompt, false)
	if err != nil {
		return false
	}
	b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          msg.Chat.ID,
		MessageThreadID: msg.MessageThreadID,
		Text:            resp.Content,
		ParseMode:       models.ParseModeHTML,
	})
	return true
}

func (h *MentionHandler) getTopics(ctx context.Context) string {
	rows, err := h.db.QueryContext(ctx,
		`SELECT name, description FROM topics WHERE is_active = 1`)
	if err != nil {
		return ""
	}
	defer rows.Close()

	var sb strings.Builder
	for rows.Next() {
		var name, desc string
		if err := rows.Scan(&name, &desc); err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("- %s: %s\n", name, desc))
	}
	return sb.String()
}

func (h *MentionHandler) resolveTopic(ctx context.Context, slug string) int {
	var tgThreadID int
	err := h.db.QueryRowContext(ctx,
		`SELECT tg_thread_id FROM topics
		 WHERE (LOWER(slug) = LOWER(?) OR LOWER(name) = LOWER(?) OR aliases LIKE '%' || ? || '%')
		 AND is_active = 1 LIMIT 1`,
		slug, slug, slug,
	).Scan(&tgThreadID)
	if err != nil {
		return 0
	}
	return tgThreadID
}

func (h *MentionHandler) getAvailableTopics(ctx context.Context) string {
	rows, err := h.db.QueryContext(ctx,
		`SELECT name FROM topics WHERE is_active = 1 ORDER BY name`)
	if err != nil {
		return ""
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		names = append(names, name)
	}
	return strings.Join(names, ", ")
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
