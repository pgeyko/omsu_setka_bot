package main

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	omsudb "omsu_bot/internal/db"
	"omsu_bot/internal/handler"
	"omsu_bot/internal/llm"
	"omsu_bot/internal/messages"
)

// CommandHandler defines a single bot slash command.
type CommandHandler struct {
	Names    []string
	AdminOnly bool
	Handle   func(ctx context.Context, b *tgbot.Bot, msg *models.Message, args string, deps *CommandDeps) error
}

// CommandDeps groups all dependencies for command handlers.
type CommandDeps struct {
	DB              *omsudb.DB
	SQLDB           *sql.DB
	MentionHandler  *handler.MentionHandler
	SettingsHandler *handler.SettingsHandler
	BotMsgs         *messages.Messages
	HelpText        string
	PromptRegistry  *llm.PromptRegistry
}

// Commands registry — populated via init().
var Commands = map[string]*CommandHandler{}

func registerCommand(h *CommandHandler) {
	for _, name := range h.Names {
		Commands[name] = h
	}
}

func dispatchSlashCommand(ctx context.Context, b *tgbot.Bot, msg *models.Message, deps *CommandDeps) error {
	parts := strings.SplitN(msg.Text, " ", 2)
	command := strings.TrimPrefix(parts[0], "/")
	command = strings.Split(command, "@")[0]
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	h, ok := Commands[command]
	if !ok {
		return nil // let fallback handle
	}

	slog.Debug("dispatching command", "command", command, "chat_id", msg.Chat.ID)
	return h.Handle(ctx, b, msg, args, deps)
}
