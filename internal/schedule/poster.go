package schedule

import (
	"context"

	tgbot "github.com/go-telegram/bot"
)

// BotPoster implements TelegramPoster using a real Telegram bot client.
type BotPoster struct {
	b *tgbot.Bot
}

func NewBotPoster(b *tgbot.Bot) *BotPoster {
	return &BotPoster{b: b}
}

func (p *BotPoster) PostToThread(ctx context.Context, chatID int64, threadID int, text string) error {
	_, err := p.b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          chatID,
		MessageThreadID: threadID,
		Text:            text,
	})
	return err
}

func (p *BotPoster) Send(ctx context.Context, chatID int64, text string) error {
	_, err := p.b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	return err
}

// Ensure BotPoster satisfies the BotSender interface expected by the API layer.
func (p *BotPoster) SendMessage(ctx context.Context, chatID int64, text string) error {
	return p.Send(ctx, chatID, text)
}

