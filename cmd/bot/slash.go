package main

import (
	"context"
	"database/sql"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	omsudb "omsu_bot/internal/db"
	"omsu_bot/internal/handler"
	"omsu_bot/internal/llm"
	"omsu_bot/internal/messages"
)

func handleSlashCommand(ctx context.Context, b *tgbot.Bot, update *models.Update, d *commandDeps) {
	msg := update.Message
	if msg == nil || msg.From == nil || msg.Text == "" || msg.Text[0] != '/' {
		return
	}

	text := msg.Text
	parts := strings.SplitN(text, " ", 2)
	cmdName := strings.TrimPrefix(parts[0], "/")
	cmdName = strings.Split(cmdName, "@")[0]
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	deps := &CommandDeps{
		DB:              d.db,
		SQLDB:           d.sqlDB,
		MentionHandler:  d.mh,
		SettingsHandler: d.settingsH,
		BotMsgs:         d.botMsgs,
		HelpText:        d.helpText,
		PromptRegistry:  d.promptReg,
	}

	h, ok := Commands[cmdName]
	if ok {
		h.Handle(ctx, b, msg, args, deps)
	} else {
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
			Text: d.botMsgs.UnknownCommand,
		})
	}
}

type commandDeps struct {
	db          *omsudb.DB
	sqlDB       *sql.DB
	mh          *handler.MentionHandler
	settingsH   *handler.SettingsHandler
	helpText    string
	botMsgs     *messages.Messages
	promptReg   *llm.PromptRegistry
}
