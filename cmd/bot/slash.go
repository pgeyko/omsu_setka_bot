package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	omsudb "omsu_bot/internal/db"
	handlers "omsu_bot/internal/handler"
	"omsu_bot/internal/llm"
	"omsu_bot/internal/messages"
	"omsu_bot/internal/util"
)

func handleSlashCommand(ctx context.Context, b *tgbot.Bot, update *models.Update, db *sql.DB, groupID int64, mh *handlers.MentionHandler, helpText string, cmd *handlers.CommandRegistry, settingsHandler *handlers.SettingsHandler, botMsgs *messages.Messages, promptRegistry *llm.PromptRegistry) {
	msg := update.Message
	if msg == nil || msg.From == nil {
		return
	}
	text := msg.Text

	parts := strings.SplitN(text, " ", 2)
	command := strings.TrimPrefix(parts[0], "/")
	command = strings.Split(command, "@")[0]
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	switch command {
	case "init":
		if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: msg.Chat.ID,
				Text:   botMsgs.InitGroupOnly,
			})
			return
		}

		member, err := b.GetChatMember(ctx, &tgbot.GetChatMemberParams{
			ChatID: msg.Chat.ID,
			UserID: msg.From.ID,
		})
		if err != nil {
			slog.Error("failed to get chat member", "error", err)
			return
		}
		if member.Type != models.ChatMemberTypeAdministrator && member.Type != models.ChatMemberTypeOwner {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: msg.Chat.ID,
				Text:   botMsgs.InitAdminOnly,
			})
			return
		}

		omsuID := 0
		if args != "" {
			omsuID, err = strconv.Atoi(args)
			if err != nil {
				b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: msg.Chat.ID,
					Text:   botMsgs.InitInvalidID,
				})
				return
			}
		}

		dDB := &omsudb.DB{DB: db}
		existingGroup, _ := dDB.GetGroup(ctx, msg.Chat.ID)

		var restrictedStr string
		db.QueryRowContext(ctx, `SELECT value FROM bot_config WHERE key = 'group_registration_restricted'`).Scan(&restrictedStr)
		if restrictedStr == "true" && existingGroup == nil {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: msg.Chat.ID,
				Text:   "❌ Регистрация групп через /init отключена. Обратитесь к суперадмину.",
			})
			return
		}

		var g omsudb.Group
		if existingGroup == nil {
			g = omsudb.Group{
				ChatID:      msg.Chat.ID,
				Title:       msg.Chat.Title,
				APIToken:    generateAPIToken(),
				OmsuGroupID: omsuID,
				IsActive:    true,
				IsVIP:       false,
			}
			err = dDB.CreateGroup(ctx, &g)
		} else {
			g = *existingGroup
			if omsuID != 0 {
				g.OmsuGroupID = omsuID
			}
			g.IsActive = true
			if g.Title == "" || strings.HasSuffix(g.Title, "[DELETED]") {
				g.Title = msg.Chat.Title
			}
			err = dDB.UpdateGroup(ctx, &g)
		}

		if err != nil {
			slog.Error("failed to upsert group on /init", "error", err)
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: msg.Chat.ID,
				Text:   botMsgs.InitDBError,
			})
			return
		}

		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: msg.Chat.ID,
			Text:   botMsgs.InitSuccess,
		})
		return

	case "settings", "настройки":
		if settingsHandler != nil {
			settingsHandler.HandleSettingsCommand(ctx, b, update)
		}

	case "help":
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:          msg.Chat.ID,
			MessageThreadID: msg.MessageThreadID,
			Text:            helpText,
			ParseMode:       models.ParseModeHTML,
		})

	case "resend", "перешли":
		if args == "" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: botMsgs.ResendTopicNotFound})
			return
		}

		var tgThreadID int
		err := db.QueryRowContext(ctx,
			`SELECT tg_thread_id FROM topics WHERE group_id = ? AND (slug = ? OR name = ?) AND is_active = 1 LIMIT 1`,
			msg.Chat.ID, args, args,
		).Scan(&tgThreadID)
		if err != nil {
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: botMsgs.Format(botMsgs.ResendTopicNotFound, map[string]string{"topic": args})})
			return
		}

		forwardMsgID := msg.ID
		if msg.ReplyToMessage != nil {
			forwardMsgID = msg.ReplyToMessage.ID
		} else {
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: botMsgs.ResendNoReply})
			return
		}

		fromChatID := fmt.Sprintf("%d", msg.Chat.ID)
		_, err = b.CopyMessage(ctx, &tgbot.CopyMessageParams{
			ChatID: msg.Chat.ID, FromChatID: fromChatID, MessageID: forwardMsgID, MessageThreadID: tgThreadID,
		})
		if err != nil {
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: botMsgs.ResendError})
			return
		}

	case "summary", "саммари":
		update.Message.Text = "саммари"
		mh.Handle(ctx, b, update)

	case "start":
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
			Text:      helpText,
			ParseMode: models.ParseModeHTML,
		})

	case "status":
		uptime := time.Since(botStartTime).Round(time.Second)
		var msgCount, fwdCount, llmToday int
		db.QueryRowContext(ctx, `SELECT COUNT(*) FROM processed_messages WHERE chat_id = ?`, msg.Chat.ID).Scan(&msgCount)
		db.QueryRowContext(ctx, `SELECT COUNT(*) FROM processed_messages WHERE chat_id = ? AND action = 'forwarded'`, msg.Chat.ID).Scan(&fwdCount)
		db.QueryRowContext(ctx, `SELECT COUNT(*) FROM llm_requests WHERE group_id = ? AND date(created_at) = date('now')`, msg.Chat.ID).Scan(&llmToday)

		statsStr := fmt.Sprintf("Аптайм: %s\nОбработано сообщений: %d\nПереслано: %d\nLLM запросов сегодня: %d\nПровайдеров: %d\nБот: @%s",
			uptime, msgCount, fwdCount, llmToday, providerCount, botUsername)

		if globalLLM != nil {
			statusPrompt := strings.ReplaceAll(promptRegistry.Get("status_report"), "{stats}", statsStr)
			resp, err := globalLLM.Call(ctx, "diagnostic", "", statusPrompt, false)
			if err == nil {
				resp.Content = util.StripMarkdown(resp.Content)
				b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
					Text: resp.Content, ParseMode: models.ParseModeHTML,
				})
				break
			}
		}
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
			Text: statsStr, ParseMode: models.ParseModeHTML,
		})

	case "register", "зарегистрируй":
		if args == "" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: botMsgs.RegisterUsage})
			return
		}
		if msg.MessageThreadID == 0 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, Text: botMsgs.RegisterGeneral})
			return
		}
		var existing int
		db.QueryRowContext(ctx, `SELECT COUNT(*) FROM topics WHERE group_id = ? AND tg_thread_id = ?`, msg.Chat.ID, msg.MessageThreadID).Scan(&existing)
		if existing > 0 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: botMsgs.RegisterExists})
			return
		}
		slug := util.MakeSlug(args)
		_, err := db.ExecContext(ctx,
			`INSERT INTO topics (group_id, tg_thread_id, name, slug, aliases, description, hashtags, is_active, created_at)
			 VALUES (?, ?, ?, ?, '[]', '', '[]', 1, CURRENT_TIMESTAMP)`,
			msg.Chat.ID, msg.MessageThreadID, args, slug)
		if err != nil {
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: botMsgs.RegisterError})
			return
		}
		reply := botMsgs.Format(botMsgs.RegisterOK, map[string]string{"name": args, "id": fmt.Sprintf("%d", msg.MessageThreadID)})
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: reply})

	case "topics", "топики":
		rows, err := db.QueryContext(ctx, `SELECT name, COALESCE(slug, ''), is_active FROM topics WHERE group_id = ? ORDER BY name`, msg.Chat.ID)
		if err != nil {
			return
		}
		defer rows.Close()
		var list string
		for rows.Next() {
			var name, slug string
			var active int
			rows.Scan(&name, &slug, &active)
			if active == 1 {
				list += fmt.Sprintf("• %s (%s)\n", name, slug)
			} else {
				list += fmt.Sprintf("• %s (%s) 🔒\n", name, slug)
			}
		}
		if list == "" {
			list = botMsgs.TopicsEmpty
		}
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
			Text: botMsgs.TopicsHeader + "\n" + list, ParseMode: models.ParseModeHTML,
		})

	case "tag", "тег":
		tagName := strings.TrimPrefix(args, "#")
		if tagName == "" {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
				Text: botMsgs.TagUsage,
			})
			return
		}
		dDB := &omsudb.DB{DB: db}
		messages, err := dDB.GetMessagesByTag(ctx, msg.Chat.ID, tagName, 20)
		if err != nil {
			slog.Error("failed to search tags", "error", err)
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
				Text: botMsgs.TagError,
			})
			return
		}
		if len(messages) == 0 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
				Text: botMsgs.Format(botMsgs.TagEmpty, map[string]string{"tag": tagName}),
			})
			return
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📌 <b>#%s</b> — найдено %d:\n\n", tagName, len(messages)))
		chatIDPos := msg.Chat.ID
		if chatIDPos < 0 {
			chatIDPos = -chatIDPos
		}
		for i, m := range messages {
			if i >= 10 {
				sb.WriteString(fmt.Sprintf("\n... и ещё %d", len(messages)-10))
				break
			}
			link := fmt.Sprintf("https://t.me/c/%d/%d", chatIDPos, m.MessageID)
			preview := m.Text
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			timeStr := m.CreatedAt.Format("02.01 15:04")
			sb.WriteString(fmt.Sprintf("<a href=\"%s\">🔗</a> %s @%s\n<code>%s</code>\n\n", link, timeStr, m.Username, preview))
		}
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
			Text: sb.String(), ParseMode: models.ParseModeHTML,
		})

	case "id", "topic_id":
		if msg.MessageThreadID != 0 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
				Text: botMsgs.Format(botMsgs.IDTopic, map[string]string{"id": fmt.Sprintf("%d", msg.MessageThreadID)}),
			})
		} else {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: msg.Chat.ID, Text: botMsgs.IDGeneral,
			})
		}

	default:
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: botMsgs.UnknownCommand,
		})
	}
}
