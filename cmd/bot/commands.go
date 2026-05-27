package main

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	omsudb "omsu_bot/internal/db"
	"omsu_bot/internal/handler"
	"omsu_bot/internal/llm"
	"omsu_bot/internal/messages"
)

// CommandHandler defines a single bot slash command.
type CommandHandler struct {
	Names     []string
	AdminOnly bool
	Handle    func(ctx context.Context, b *tgbot.Bot, msg *models.Message, args string, deps *CommandDeps) error
}

// CommandDeps groups all dependencies for command handlers.
type CommandDeps struct {
	DB              *omsudb.DB
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
