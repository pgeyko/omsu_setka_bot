package main

import (
	"context"
	"fmt"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"omsu_bot/internal/util"
)

func handleRegister(ctx context.Context, b *tgbot.Bot, msg *models.Message, args string, deps *CommandDeps) error {
	if args == "" {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: deps.BotMsgs.RegisterUsage})
		return nil
	}
	if msg.MessageThreadID == 0 {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, Text: deps.BotMsgs.RegisterGeneral})
		return nil
	}
	var existing int
	deps.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM topics WHERE group_id = ? AND tg_thread_id = ?`, msg.Chat.ID, msg.MessageThreadID).Scan(&existing)
	if existing > 0 {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: deps.BotMsgs.RegisterExists})
		return nil
	}
	slug := util.MakeSlug(args)
	_, err := deps.DB.ExecContext(ctx,
		`INSERT INTO topics (group_id, tg_thread_id, name, slug, aliases, description, hashtags, is_active, created_at)
		 VALUES (?, ?, ?, ?, '[]', '', '[]', 1, CURRENT_TIMESTAMP)`,
		msg.Chat.ID, msg.MessageThreadID, args, slug)
	if err != nil {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: deps.BotMsgs.RegisterError})
		return nil
	}
	reply := deps.BotMsgs.Format(deps.BotMsgs.RegisterOK, map[string]string{"name": args, "id": fmt.Sprintf("%d", msg.MessageThreadID)})
	b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: reply})
	return nil
}

func handleTopics(ctx context.Context, b *tgbot.Bot, msg *models.Message, args string, deps *CommandDeps) error {
	rows, err := deps.DB.QueryContext(ctx, `SELECT name, COALESCE(slug, ''), is_active FROM topics WHERE group_id = ? ORDER BY name`, msg.Chat.ID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var list string
	for rows.Next() {
		var name, slug string
		var active int
		if err := rows.Scan(&name, &slug, &active); err != nil {
			continue
		}
		if active == 1 {
			list += fmt.Sprintf("• %s (%s)\n", name, slug)
		} else {
			list += fmt.Sprintf("• %s (%s) 🔒\n", name, slug)
		}
	}
	if list == "" {
		list = deps.BotMsgs.TopicsEmpty
	}
	b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
		Text: deps.BotMsgs.TopicsHeader + "\n" + list, ParseMode: models.ParseModeHTML,
	})
	return nil
}

func handleID(ctx context.Context, b *tgbot.Bot, msg *models.Message, args string, deps *CommandDeps) error {
	if msg.MessageThreadID != 0 {
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
			Text: deps.BotMsgs.Format(deps.BotMsgs.IDTopic, map[string]string{"id": fmt.Sprintf("%d", msg.MessageThreadID)}),
		})
	} else {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, Text: deps.BotMsgs.IDGeneral})
	}
	return nil
}

func init() {
	registerCommand(&CommandHandler{Names: []string{"register", "зарегистрируй"}, Handle: handleRegister})
	registerCommand(&CommandHandler{Names: []string{"topics", "топики"}, Handle: handleTopics})
	registerCommand(&CommandHandler{Names: []string{"id", "topic_id"}, Handle: handleID})
}
