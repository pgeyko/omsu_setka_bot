package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Antispam struct {
	mu           sync.Mutex
	joinedUsers  map[string]time.Time
	messageTimes map[string][]time.Time
	loadFeatures func(chatID int64) map[string]bool
	captchaStore sync.Map // key=chatID:memberID, value=correct answer
}

func NewAntispam(loaders ...func(chatID int64) map[string]bool) *Antispam {
	var loader func(chatID int64) map[string]bool
	if len(loaders) > 0 {
		loader = loaders[0]
	}
	return &Antispam{
		joinedUsers:  make(map[string]time.Time),
		messageTimes: make(map[string][]time.Time),
		loadFeatures: loader,
	}
}

func (a *Antispam) getFeatures(chatID int64) map[string]bool {
	if a.loadFeatures != nil {
		return a.loadFeatures(chatID)
	}
	return map[string]bool{
		"enable_moderation":    true,
		"enable_captcha":       true,
		"enable_link_filter":   true,
		"enable_flood_control": true,
	}
}

func (a *Antispam) HandleNewChatMembers(ctx context.Context, b *tgbot.Bot, chatID int64, newMembers []models.User) {
	features := a.getFeatures(chatID)
	if !features["enable_moderation"] || !features["enable_captcha"] {
		return
	}
	for _, member := range newMembers {
		if member.IsBot {
			continue
		}

		key := fmt.Sprintf("%d:%d", chatID, member.ID)
		a.mu.Lock()
		a.joinedUsers[key] = time.Now()
		// Clean up entries older than 24 hours
		for k, t := range a.joinedUsers {
			if time.Since(t) > 24*time.Hour {
				delete(a.joinedUsers, k)
			}
		}
		a.mu.Unlock()

		if b != nil {
			_, err := b.RestrictChatMember(ctx, &tgbot.RestrictChatMemberParams{
				ChatID: chatID,
				UserID: member.ID,
				Permissions: &models.ChatPermissions{
					CanSendMessages:       false,
					CanSendAudios:         false,
					CanSendDocuments:      false,
					CanSendPhotos:         false,
					CanSendVideos:         false,
					CanSendVideoNotes:     false,
					CanSendVoiceNotes:     false,
					CanSendPolls:          false,
					CanSendOtherMessages:  false,
					CanAddWebPagePreviews: false,
				},
			})
			if err != nil {
				slog.Error("failed to restrict new member", "userID", member.ID, "error", err)
			}
		}

		x := rand.Intn(9) + 1
		y := rand.Intn(9) + 1
		correct := x + y

		name := member.Username
		if name == "" {
			name = member.FirstName
		} else {
			name = "@" + name
		}

		text := fmt.Sprintf("👋 Привет, %s! Реши пример, чтобы получить возможность писать в группу:\n\n<b>%d + %d = ?</b>", name, x, y)

		opts := []int{correct, correct + 1, correct - 2, correct + 3}
		uniqueOpts := make(map[int]bool)
		var shuffled []int
		for _, o := range opts {
			if o <= 0 {
				o = correct + rand.Intn(5) + 4
			}
			if uniqueOpts[o] {
				continue
			}
			uniqueOpts[o] = true
			shuffled = append(shuffled, o)
		}

		rand.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})

		captchaKey := fmt.Sprintf("%d:%d", chatID, member.ID)
		a.captchaStore.Store(captchaKey, correct)

		var keyboard [][]models.InlineKeyboardButton
		var row []models.InlineKeyboardButton
		for _, o := range shuffled {
			row = append(row, models.InlineKeyboardButton{
				Text:         strconv.Itoa(o),
				CallbackData: fmt.Sprintf("captcha:%d:%d", member.ID, o),
			})
		}
		keyboard = append(keyboard, row)

		if b != nil {
			_, err := b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:    chatID,
				Text:      text,
				ParseMode: models.ParseModeHTML,
				ReplyMarkup: &models.InlineKeyboardMarkup{
					InlineKeyboard: keyboard,
				},
			})
			if err != nil {
				slog.Error("failed to send captcha message", "error", err)
			}
		}
	}
}

func (a *Antispam) CheckFloodAndLinks(ctx context.Context, b *tgbot.Bot, msg *models.Message) bool {
	features := a.getFeatures(msg.Chat.ID)
	if !features["enable_moderation"] {
		return false
	}

	chatID := msg.Chat.ID
	userID := msg.From.ID
	username := msg.From.Username
	if username == "" {
		username = msg.From.FirstName
	}
	key := fmt.Sprintf("%d:%d", chatID, userID)

	now := time.Now()

	// 1. Link filtering
	if features["enable_link_filter"] {
		hasLink := false
		entities := msg.Entities
		if len(entities) == 0 {
			entities = msg.CaptionEntities
		}
		for _, ent := range entities {
			if ent.Type == models.MessageEntityTypeURL || ent.Type == models.MessageEntityTypeTextLink {
				hasLink = true
				break
			}
		}

		if hasLink {
			a.mu.Lock()
			joinTime, exists := a.joinedUsers[key]
			if exists && now.Sub(joinTime) >= 24*time.Hour {
				delete(a.joinedUsers, key)
				exists = false
			}
			a.mu.Unlock()

			if exists && now.Sub(joinTime) < 24*time.Hour {
				slog.Info("Deleting link message from new user", "userID", userID, "chatID", chatID)
				if b != nil {
					b.DeleteMessage(ctx, &tgbot.DeleteMessageParams{
						ChatID:    chatID,
						MessageID: msg.ID,
					})
				}
				return true
			}
		}
	}

	// 2. Flood Control
	if features["enable_flood_control"] {
		a.mu.Lock()
		times := a.messageTimes[key]
		var validTimes []time.Time
		for _, t := range times {
			if now.Sub(t) <= 10*time.Second {
				validTimes = append(validTimes, t)
			}
		}
		validTimes = append(validTimes, now)
		a.messageTimes[key] = validTimes
		floodDetected := len(validTimes) > 5
		a.mu.Unlock()

		if floodDetected {
			slog.Warn("Flood detected", "userID", userID, "chatID", chatID)
			untilDate := now.Add(5 * time.Minute).Unix()
			if b != nil {
				_, err := b.RestrictChatMember(ctx, &tgbot.RestrictChatMemberParams{
					ChatID: chatID,
					UserID: userID,
					Permissions: &models.ChatPermissions{
						CanSendMessages:       false,
						CanSendAudios:         false,
						CanSendDocuments:      false,
						CanSendPhotos:         false,
						CanSendVideos:         false,
						CanSendVideoNotes:     false,
						CanSendVoiceNotes:     false,
						CanSendPolls:          false,
						CanSendOtherMessages:  false,
						CanAddWebPagePreviews: false,
					},
					UntilDate: int(untilDate),
				})
				if err != nil {
					slog.Error("failed to restrict user after flood", "userID", userID, "error", err)
				}

				b.DeleteMessage(ctx, &tgbot.DeleteMessageParams{
					ChatID:    chatID,
					MessageID: msg.ID,
				})

				warnText := fmt.Sprintf("⚠️ @%s временно заблокирован на 5 минут за спам.", username)
				b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text:   warnText,
				})
			}
			return true
		}
	}

	return false
}

func (a *Antispam) HandleCallbackQuery(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}
	cb := update.CallbackQuery
	data := cb.Data

	if b == nil {
		return
	}

	if !strings.HasPrefix(data, "captcha:") {
		return // not our callback, let other handlers process it
	}

	parts := strings.Split(data, ":")
	if len(parts) != 3 {
		b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "Ошибка: неверный формат капчи.",
		})
		slog.Warn("captcha: invalid callback data", "data", data)
		return
	}

	targetUserID, _ := strconv.ParseInt(parts[1], 10, 64)
	chosenAnswer, _ := strconv.Atoi(parts[2])

	chatID, msgID := getChatAndMsgID(cb.Message)

	captchaKey := fmt.Sprintf("%d:%d", chatID, targetUserID)
	stored, ok := a.captchaStore.LoadAndDelete(captchaKey)
	if !ok {
		b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "❌ Капча устарела. Попросите администратора снова.",
			ShowAlert:       true,
		})
		return
	}
	correctAnswer := stored.(int)

	if b == nil {
		return
	}

	// Wrong user pressed the button — notify with alert
	if cb.From.ID != targetUserID {
		b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "❌ Эта капча не для вас!",
			ShowAlert:       true,
		})
		slog.Info("captcha: wrong user pressed", "target", targetUserID, "pressed", cb.From.ID)
		return
	}

	features := a.getFeatures(chatID)
	if !features["enable_moderation"] || !features["enable_captcha"] {
		b.RestrictChatMember(ctx, &tgbot.RestrictChatMemberParams{
			ChatID: chatID,
			UserID: targetUserID,
			Permissions: &models.ChatPermissions{
				CanSendMessages:       true,
				CanSendAudios:         true,
				CanSendDocuments:      true,
				CanSendPhotos:         true,
				CanSendVideos:         true,
				CanSendVideoNotes:     true,
				CanSendVoiceNotes:     true,
				CanSendPolls:          true,
				CanSendOtherMessages:  true,
				CanAddWebPagePreviews: true,
			},
		})
		b.DeleteMessage(ctx, &tgbot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: msgID,
		})
		b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "Капча отключена.",
			ShowAlert:       true,
		})
		return
	}

	if chosenAnswer == correctAnswer {
		_, err := b.RestrictChatMember(ctx, &tgbot.RestrictChatMemberParams{
			ChatID: chatID,
			UserID: targetUserID,
			Permissions: &models.ChatPermissions{
				CanSendMessages:       true,
				CanSendAudios:         true,
				CanSendDocuments:      true,
				CanSendPhotos:         true,
				CanSendVideos:         true,
				CanSendVideoNotes:     true,
				CanSendVoiceNotes:     true,
				CanSendPolls:          true,
				CanSendOtherMessages:  true,
				CanAddWebPagePreviews: true,
			},
		})
		if err != nil {
			slog.Error("failed to unmute user after captcha", "userID", targetUserID, "error", err)
		}

		b.DeleteMessage(ctx, &tgbot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: msgID,
		})
		b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "✅ Капча решена! Вы можете общаться.",
			ShowAlert:       true,
		})
	} else {
		b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "❌ Неверный ответ! Попробуйте еще раз.",
			ShowAlert:       true,
		})
	}
}

func getChatAndMsgID(mim models.MaybeInaccessibleMessage) (int64, int) {
	if mim.Message != nil {
		return mim.Message.Chat.ID, mim.Message.ID
	}
	if mim.InaccessibleMessage != nil {
		return mim.InaccessibleMessage.Chat.ID, mim.InaccessibleMessage.MessageID
	}
	return 0, 0
}
