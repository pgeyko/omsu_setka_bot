package forwarder

import (
	"context"
	"fmt"

	"omsu_bot/internal/util"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type PersonaProvider interface {
	Name() string
}

type Forwarder struct {
	b       *tgbot.Bot
	persona PersonaProvider
}

func New(b *tgbot.Bot, persona PersonaProvider) *Forwarder {
	return &Forwarder{b: b, persona: persona}
}

func (f *Forwarder) Duplicate(ctx context.Context, fromChatID int64, fromThreadID int, targetThreadID int, username, fromTopicName string, hashtags []string, messageID int) (*models.MessageID, error) {
	header := f.buildHeader(fromTopicName, username, hashtags)

	copied, err := f.b.CopyMessage(ctx, &tgbot.CopyMessageParams{
		ChatID:          fromChatID,
		FromChatID:      fmt.Sprintf("%d", fromChatID),
		MessageID:       messageID,
		MessageThreadID: targetThreadID,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "📌 Перейти", URL: util.ChatLink(fromChatID, messageID)},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("copy message failed: %w", err)
	}

	var signature string
	if f.persona.Name() != "" {
		signature = "\n— " + f.persona.Name()
	}

	headerMsg := header + signature
	if _, err := f.b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          fromChatID,
		MessageThreadID: targetThreadID,
		Text:            headerMsg,
		ParseMode:       models.ParseModeHTML,
	}); err != nil {
		return nil, fmt.Errorf("send header failed: %w", err)
	}

	return copied, nil
}

func (f *Forwarder) ReplyWithLink(ctx context.Context, chatID int64, threadID int, replyToID int, topicName string) error {
	link := util.ChatLink(chatID, replyToID)
	text := fmt.Sprintf("↗️ Продублировал в «%s» → <a href=\"%s\">link</a>", topicName, link)

	_, err := f.b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          chatID,
		MessageThreadID: threadID,
		Text:            text,
		ParseMode:       models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: replyToID,
		},
	})
	return err
}

func (f *Forwarder) buildHeader(fromTopic, username string, hashtags []string) string {
	var header string
	for i, tag := range hashtags {
		if i == 0 {
			header = "#" + tag
		} else {
			header += " #" + tag
		}
	}
	return header
}
