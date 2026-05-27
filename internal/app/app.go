// Package app provides the App struct and shared initialization helpers.
package app

import (
	"context"
	"log/slog"
	"sync"
	"time"

	omsudb "omsu_bot/internal/db"
	"omsu_bot/internal/llm"
	"omsu_bot/internal/persona"
	"omsu_bot/internal/config"
	"omsu_bot/internal/buffer"
	"omsu_bot/internal/telegram"

	tgbot "github.com/go-telegram/bot"
	"github.com/gofiber/fiber/v2"
)

// App holds all top-level application state.
type App struct {
	Config             *config.Config
	DB                 *omsudb.DB
	Bot                *tgbot.Bot
	API                *fiber.App
	LLMClient          *llm.Client
	Prompts            *llm.PromptRegistry
	PersonaStore       *persona.Store
	Buffer             *buffer.SummaryBuffer
	UsernameCache      *telegram.UsernameCache

	GlobalLLM           llm.LLMClient

	ProcessedMediaGroups sync.Map
	MediaGroupMessages   sync.Map
	BotStartTime         time.Time
	BotUsername          string
	ProviderCount        int
}

// New creates an App with core initialization.
func New(cfg *config.Config) *App {
	return &App{
		Config:      cfg,
		BotStartTime: time.Now(),
	}
}

// Shutdown stops background goroutines and closes resources.
func (a *App) Shutdown(ctx context.Context) {
	if a.Buffer != nil {
		a.Buffer.Close()
	}
	if a.UsernameCache != nil {
		a.UsernameCache.Close()
	}
	if a.DB != nil {
		a.DB.Close()
	}
	slog.Info("app: shutdown complete")
}
