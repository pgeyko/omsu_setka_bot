package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"os"
	"strings"
	"unicode/utf16"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"omsu_bot/internal/classifier"
	"omsu_bot/internal/config"
	"omsu_bot/internal/telegram"
)

type dbTopicsProvider struct {
	db *sql.DB
}

func (p *dbTopicsProvider) GetTopics(ctx context.Context, chatID int64) ([]classifier.TopicInfo, error) {
	rows, err := p.db.QueryContext(ctx, "SELECT slug, name, description FROM topics WHERE group_id = ? AND is_active = 1", chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []classifier.TopicInfo
	for rows.Next() {
		var t classifier.TopicInfo
		if err := rows.Scan(&t.Slug, &t.Name, &t.Description); err != nil {
			return nil, err
		}
		topics = append(topics, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return topics, nil
}

func setupLogger(cfg *config.Config) {
	level := slog.LevelInfo
	if cfg.Logging.Level == "debug" {
		level = slog.LevelDebug
	} else if cfg.Logging.Level == "warn" {
		level = slog.LevelWarn
	} else if cfg.Logging.Level == "error" {
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if cfg.Logging.Format == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(h))
}

func isBotCommand(msg *models.Message) bool {
	if msg.Entities == nil {
		return false
	}
	for _, e := range msg.Entities {
		if e.Type == models.MessageEntityTypeBotCommand {
			return true
		}
	}
	return false
}

func isBotMention(msg *models.Message) bool {
	checkEntities := func(entities []models.MessageEntity, text string) bool {
		if len(entities) == 0 || text == "" {
			return false
		}
		u16 := utf16.Encode([]rune(text))
		for _, e := range entities {
			if e.Type == models.MessageEntityTypeMention {
				if e.Offset >= 0 && e.Offset+e.Length <= len(u16) {
					mention := string(utf16.Decode(u16[e.Offset : e.Offset+e.Length]))
					if strings.EqualFold(mention, "@"+app.BotUsername) {
						return true
					}
				}
			}
		}
		return false
	}

	if checkEntities(msg.Entities, msg.Text) {
		return true
	}
	if checkEntities(msg.CaptionEntities, msg.Caption) {
		return true
	}

	atBot := "@" + app.BotUsername
	if msg.Text != "" && strings.Contains(strings.ToLower(msg.Text), strings.ToLower(atBot)) {
		return true
	}
	if msg.Caption != "" && strings.Contains(strings.ToLower(msg.Caption), strings.ToLower(atBot)) {
		return true
	}

	return false
}

func handleRollCall(ctx context.Context, b *tgbot.Bot, msg *models.Message, usernameCache *telegram.UsernameCache) {
	admins, err := b.GetChatAdministrators(ctx, &tgbot.GetChatAdministratorsParams{ChatID: msg.Chat.ID})
	if err != nil {
		slog.Error("roll call: failed to get admins", "error", err)
	}

	seen := make(map[string]bool)
	var mentions []string

	if admins != nil {
		for _, a := range admins {
			var username string
			switch {
			case a.Owner != nil:
				username = a.Owner.User.Username
			case a.Administrator != nil:
				username = a.Administrator.User.Username
			}
			if username != "" && !seen[username] {
				seen[username] = true
				mentions = append(mentions, "@"+username)
			}
		}
	}

	if usernameCache != nil {
		for _, u := range usernameCache.All() {
			if u != "" && !seen[u] {
				seen[u] = true
				mentions = append(mentions, "@"+u)
			}
		}
	}

	if len(mentions) == 0 {
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          msg.Chat.ID,
			MessageThreadID: msg.MessageThreadID,
			Text:            "📢 ПЕРЕКЛИЧКА! Нет данных об участниках.",
		})
		return
	}

	text := "📢 <b>ПЕРЕКЛИЧКА!</b>\n⚠️ <i>(показаны только администраторы и недавно активные участники)</i>\n"
	for i, m := range mentions {
		if i > 0 && i%5 == 0 {
			text += "\n"
		}
		text += m + " "
	}

	b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          msg.Chat.ID,
		MessageThreadID: msg.MessageThreadID,
		Text:            strings.TrimSpace(text),
		ParseMode:       models.ParseModeHTML,
	})
}

func generateAPIToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand.Read failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
