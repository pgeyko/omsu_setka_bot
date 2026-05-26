package forwarder

import (
	"context"
	"fmt"
	"strings"

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
	})
	if err != nil {
		return nil, fmt.Errorf("copy message failed: %w", err)
	}

	headerMsg := header
	if headerMsg != "" {
		if _, err := f.b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          fromChatID,
			MessageThreadID: targetThreadID,
			Text:            headerMsg,
			ParseMode:       models.ParseModeHTML,
		}); err != nil {
			return nil, fmt.Errorf("send header failed: %w", err)
		}
	}

	return copied, nil
}

func (f *Forwarder) ReplyWithLink(ctx context.Context, chatID int64, threadID int, replyToSourceID int, forwardedMsgID int, topicName string) error {
	link := util.ChatLink(chatID, forwardedMsgID)
	text := fmt.Sprintf("↗️ Продублировала в «%s» → <a href=\"%s\">link</a>", topicName, link)

	_, err := f.b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          chatID,
		MessageThreadID: threadID,
		Text:            text,
		ParseMode:       models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: replyToSourceID,
		},
	})
	return err
}

func (f *Forwarder) buildHeader(fromTopic, username string, hashtags []string) string {
	if len(hashtags) == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(len(hashtags) * 20)
	for i, tag := range hashtags {
		if i > 0 {
			b.WriteString(" #")
		} else {
			b.WriteByte('#')
		}
		b.WriteString(tag)
	}
	return b.String()
}
