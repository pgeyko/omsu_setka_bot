package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	omsudb "omsu_bot/internal/db"
	"omsu_bot/internal/util"
)

func handleInit(ctx context.Context, b *tgbot.Bot, msg *models.Message, args string, deps *CommandDeps) error {
	if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, Text: deps.BotMsgs.InitGroupOnly})
		return nil
	}

	member, err := b.GetChatMember(ctx, &tgbot.GetChatMemberParams{ChatID: msg.Chat.ID, UserID: msg.From.ID})
	if err != nil {
		return nil
	}
	if member.Type != models.ChatMemberTypeAdministrator && member.Type != models.ChatMemberTypeOwner {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, Text: deps.BotMsgs.InitAdminOnly})
		return nil
	}

	omsuID := 0
	if args != "" {
		omsuID, err = strconv.Atoi(args)
		if err != nil {
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, Text: deps.BotMsgs.InitInvalidID})
			return nil
		}
	}

	existingGroup, _ := deps.DB.GetGroup(ctx, msg.Chat.ID)

	var restrictedStr string
	deps.DB.QueryRowContext(ctx, `SELECT value FROM bot_config WHERE key = 'group_registration_restricted'`).Scan(&restrictedStr)
	if restrictedStr == "true" && existingGroup == nil {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, Text: "❌ Регистрация групп через /init отключена. Обратитесь к суперадмину."})
		return nil
	}

	if existingGroup == nil {
		g := omsudb.Group{
			ChatID: msg.Chat.ID, Title: msg.Chat.Title, APIToken: generateAPIToken(),
			OmsuGroupID: omsuID, IsActive: true, IsVIP: false,
		}
		err = deps.DB.CreateGroup(ctx, &g)
	} else {
		if omsuID != 0 {
			existingGroup.OmsuGroupID = omsuID
		}
		existingGroup.IsActive = true
		if existingGroup.Title == "" || strings.HasSuffix(existingGroup.Title, "[DELETED]") {
			existingGroup.Title = msg.Chat.Title
		}
		err = deps.DB.UpdateGroup(ctx, existingGroup)
	}
	if err != nil {
		b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, Text: deps.BotMsgs.InitDBError})
		return nil
	}

	b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, Text: deps.BotMsgs.InitSuccess})
	return nil
}

func init() {
	registerCommand(&CommandHandler{
		Names: []string{"init"}, AdminOnly: true,
		Handle: handleInit,
	})
}

func handleHelp(ctx context.Context, b *tgbot.Bot, msg *models.Message, args string, deps *CommandDeps) error {
	b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
		Text: deps.HelpText, ParseMode: models.ParseModeHTML,
	})
	return nil
}

func handleStart(ctx context.Context, b *tgbot.Bot, msg *models.Message, args string, deps *CommandDeps) error {
	return handleHelp(ctx, b, msg, args, deps)
}

func init() {
	registerCommand(&CommandHandler{
		Names: []string{"help"}, Handle: handleHelp,
	})
	registerCommand(&CommandHandler{
		Names: []string{"start"}, Handle: handleHelp,
	})
}

func handleStatus(ctx context.Context, b *tgbot.Bot, msg *models.Message, args string, deps *CommandDeps) error {
	uptime := time.Since(botStartTime).Round(time.Second)
	var msgCount, fwdCount, llmToday int
	deps.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM processed_messages WHERE chat_id = ?`, msg.Chat.ID).Scan(&msgCount)
	deps.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM processed_messages WHERE chat_id = ? AND action = 'forwarded'`, msg.Chat.ID).Scan(&fwdCount)
	deps.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM llm_requests WHERE group_id = ? AND date(created_at) = date('now')`, msg.Chat.ID).Scan(&llmToday)

	statsStr := fmt.Sprintf("Аптайм: %s\nОбработано сообщений: %d\nПереслано: %d\nLLM запросов сегодня: %d\nПровайдеров: %d\nБот: @%s",
		uptime, msgCount, fwdCount, llmToday, providerCount, botUsername)

	if globalLLM != nil {
		statusPrompt := strings.ReplaceAll(deps.PromptRegistry.Get("status_report"), "{stats}", statsStr)
		resp, err := globalLLM.Call(ctx, "diagnostic", "", statusPrompt, false)
		if err == nil {
			resp.Content = util.StripMarkdown(resp.Content)
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: resp.Content, ParseMode: models.ParseModeHTML})
			return nil
		}
	}
	b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: statsStr, ParseMode: models.ParseModeHTML})
	return nil
}

func init() {
	registerCommand(&CommandHandler{
		Names: []string{"status"}, Handle: handleStatus,
	})
}
