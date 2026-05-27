package main

import (
	"context"
	"fmt"
	"html"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func handleResend(ctx context.Context, b *tgbot.Bot, msg *models.Message, args string, deps *CommandDeps) error {
	if args == "" {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: deps.BotMsgs.ResendTopicNotFound})
		return nil
	}

	var tgThreadID int
	err := deps.SQLDB.QueryRowContext(ctx,
		`SELECT tg_thread_id FROM topics WHERE group_id = ? AND (slug = ? OR name = ?) AND is_active = 1 LIMIT 1`,
		msg.Chat.ID, args, args,
	).Scan(&tgThreadID)
	if err != nil {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: deps.BotMsgs.Format(deps.BotMsgs.ResendTopicNotFound, map[string]string{"topic": args})})
		return nil
	}

	forwardMsgID := msg.ID
	if msg.ReplyToMessage != nil {
		forwardMsgID = msg.ReplyToMessage.ID
	} else {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: deps.BotMsgs.ResendNoReply})
		return nil
	}

	fromChatID := fmt.Sprintf("%d", msg.Chat.ID)
	_, err = b.CopyMessage(ctx, &tgbot.CopyMessageParams{
		ChatID: msg.Chat.ID, FromChatID: fromChatID, MessageID: forwardMsgID, MessageThreadID: tgThreadID,
	})
	if err != nil {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: deps.BotMsgs.ResendError})
	}
	return nil
}

func handleTag(ctx context.Context, b *tgbot.Bot, msg *models.Message, args string, deps *CommandDeps) error {
	tagName := strings.TrimPrefix(args, "#")
	if tagName == "" {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: deps.BotMsgs.TagUsage})
		return nil
	}

	messages, err := deps.DB.GetMessagesByTag(ctx, msg.Chat.ID, tagName, 20)
	if err != nil {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: deps.BotMsgs.TagError})
		return nil
	}
	if len(messages) == 0 {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: deps.BotMsgs.Format(deps.BotMsgs.TagEmpty, map[string]string{"tag": tagName})})
		return nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📌 <b>#%s</b> — найдено %d:\n\n", tagName, len(messages)))
	chatIDPos := msg.Chat.ID
	if chatIDPos < 0 {
		chatIDPos = -chatIDPos
	}
	for i, m := range messages {
		if i >= 10 {
			sb.WriteString(fmt.Sprintf("\n... и ещё %d", len(messages)-10))
			break
		}
		link := fmt.Sprintf("https://t.me/c/%d/%d", chatIDPos, m.MessageID)
		preview := m.Text
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		sb.WriteString(fmt.Sprintf("<a href=\"%s\">🔗</a> %s @%s\n<code>%s</code>\n\n", link, m.CreatedAt.Format("02.01 15:04"), m.Username, html.EscapeString(preview)))
	}
	b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: sb.String(), ParseMode: models.ParseModeHTML})
	return nil
}

func handleSettings(ctx context.Context, b *tgbot.Bot, msg *models.Message, args string, deps *CommandDeps) error {
	if deps.SettingsHandler != nil {
		update := &models.Update{Message: msg}
		deps.SettingsHandler.HandleSettingsCommand(ctx, b, update)
	}
	return nil
}

func handleSummary(ctx context.Context, b *tgbot.Bot, msg *models.Message, args string, deps *CommandDeps) error {
	if deps.MentionHandler != nil {
		update := &models.Update{Message: msg}
		update.Message.Text = "саммари"
		deps.MentionHandler.Handle(ctx, b, update)
	}
	return nil
}

func init() {
	registerCommand(&CommandHandler{Names: []string{"resend", "перешли"}, Handle: handleResend})
	registerCommand(&CommandHandler{Names: []string{"tag", "тег"}, Handle: handleTag})
	registerCommand(&CommandHandler{Names: []string{"settings", "настройки"}, Handle: handleSettings})
	registerCommand(&CommandHandler{Names: []string{"summary", "саммари"}, Handle: handleSummary})
}
