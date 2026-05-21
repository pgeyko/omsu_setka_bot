package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"omsu_bot/internal/buffer"
	"omsu_bot/internal/llm"
	"omsu_bot/internal/telegram"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type TopicCRUD struct {
	llmClient   *llm.Client
	prompts     *llm.PromptRegistry
	db          *sql.DB
	bot         *tgbot.Bot
	groupID     int64
	botUsername string
	buffer      *buffer.SummaryBuffer

	mu         sync.RWMutex
	adminCache *telegram.AdminCache
}

func NewTopicCRUD(llmClient *llm.Client, prompts *llm.PromptRegistry, db *sql.DB, bot *tgbot.Bot, groupID int64, botUsername string, buffer *buffer.SummaryBuffer, adminCache *telegram.AdminCache) *TopicCRUD {
	return &TopicCRUD{
		llmClient:   llmClient,
		prompts:     prompts,
		db:          db,
		bot:         bot,
		groupID:     groupID,
		botUsername: botUsername,
		buffer:      buffer,
		adminCache:  adminCache,
	}
}

type TopicIntent struct {
	Intent     string  `json:"intent"`
	TopicName  string  `json:"topic_name"`
	NewName    string  `json:"new_name,omitempty"`
	Confidence float64 `json:"confidence"`
}

func (t *TopicCRUD) Handle(ctx context.Context, update *models.Update) {
	if update.Message == nil {
		return
	}

	var active int
	if err := t.db.QueryRowContext(ctx, "SELECT is_active FROM groups WHERE chat_id = ?", update.Message.Chat.ID).Scan(&active); err != nil || active != 1 {
		return
	}

	msg := update.Message
	userID := msg.From.ID

	if !t.isAdmin(ctx, msg.Chat.ID, userID) {
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"⛔ Только администраторы могут управлять топиками.")
		return
	}

	text := msg.Text
	if text == "" {
		text = msg.Caption
	}
	if text == "" {
		return
	}

	intent, err := t.parseIntent(ctx, text)
	if err != nil {
		slog.Error("topic command parsing failed", "error", err)
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"Не удалось распознать команду. Попробуй: создай топик «Название»")
		return
	}

	if intent == nil || intent.Confidence < 0.75 {
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"Не уверен в команде. Повтори точнее.")
		return
	}

	switch intent.Intent {
	case "create":
		t.handleCreate(ctx, msg, *intent)
	case "register":
		t.handleRegister(ctx, msg, *intent)
	case "close":
		t.handleClose(ctx, msg, *intent)
	case "rename":
		t.handleRename(ctx, msg, *intent)
	default:
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"Неизвестная команда. Используй: создай, закрой, переименуй")
	}
}

func (t *TopicCRUD) handleCreate(ctx context.Context, msg *models.Message, intent TopicIntent) {
	if intent.TopicName == "" {
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"Укажи название топика: создай топик «Название»")
		return
	}

	slug := makeSlug(intent.TopicName)

	forum, err := t.bot.CreateForumTopic(ctx, &tgbot.CreateForumTopicParams{
		ChatID: msg.Chat.ID,
		Name:   intent.TopicName,
	})
	if err != nil {
		slog.Error("create forum topic failed", "error", err)
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"Ошибка при создании топика в Telegram.")
		return
	}

	_, err = t.db.ExecContext(ctx,
		`INSERT INTO topics (group_id, tg_thread_id, name, slug, aliases, description, hashtags, is_active, created_at)
		 VALUES (?, ?, ?, ?, '[]', '', '[]', 1, CURRENT_TIMESTAMP)`,
		msg.Chat.ID, forum.MessageThreadID, intent.TopicName, slug)
	if err != nil {
		slog.Error("failed to save topic to DB", "error", err)
	}

	t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
		fmt.Sprintf("✅ Топик «%s» создан!", intent.TopicName))

	if t.buffer != nil && t.llmClient != nil {
		msgs := t.buffer.GetMessages(msg.Chat.ID, 0)
		if msgs != "" {
			summaryPrompt := fmt.Sprintf(
				"Сделай краткое саммари последних сообщений (2-3 предложения). Выдели главные темы.\n\nСообщения:\n%s", msgs)
			resp, err := t.llmClient.Call(ctx, "summary", "", summaryPrompt, false)
			if err == nil {
				desc := resp.Content
				if len(desc) > 300 {
					desc = desc[:300]
				}
				t.db.ExecContext(ctx, `UPDATE topics SET description = ? WHERE tg_thread_id = ?`,
					desc, forum.MessageThreadID)
				t.bot.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:          msg.Chat.ID,
					MessageThreadID: forum.MessageThreadID,
					Text:            "📋 <b>Краткое саммари</b>\n\n" + desc,
					ParseMode:       models.ParseModeHTML,
				})
			}
		}
	}
}

func (t *TopicCRUD) handleRegister(ctx context.Context, msg *models.Message, intent TopicIntent) {
	if msg.MessageThreadID == 0 {
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"❌ Это общий чат, а не топик. Напиши команду в нужном топике.")
		return
	}

	var existing int
	t.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM topics WHERE group_id = ? AND tg_thread_id = ?`, msg.Chat.ID, msg.MessageThreadID,
	).Scan(&existing)
	if existing > 0 {
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"⚠️ Этот топик уже зарегистрирован.")
		return
	}

	topicName := intent.TopicName
	if topicName == "" {
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"Не удалось определить название. Пример: зарегистрируй этот топик как Сессия")
		return
	}

	slug := makeSlug(topicName)
	var exists string
	t.db.QueryRowContext(ctx, `SELECT name FROM topics WHERE group_id = ? AND slug = ?`, msg.Chat.ID, slug).Scan(&exists)
	if exists != "" {
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			fmt.Sprintf("❌ Топик со slug «%s» уже существует.", slug))
		return
	}

	_, err := t.db.ExecContext(ctx,
		`INSERT INTO topics (group_id, tg_thread_id, name, slug, aliases, description, hashtags, is_active, created_at)
		 VALUES (?, ?, ?, ?, '[]', '', '[]', 1, CURRENT_TIMESTAMP)`,
		msg.Chat.ID, msg.MessageThreadID, topicName, slug)
	if err != nil {
		slog.Error("failed to register topic", "error", err)
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID, "❌ Ошибка при регистрации топика.")
		return
	}

	t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
		fmt.Sprintf("✅ Топик «%s» зарегистрирован (ID: %d)", topicName, msg.MessageThreadID))
}

func (t *TopicCRUD) parseIntent(ctx context.Context, text string) (*TopicIntent, error) {
	prompt := t.prompts.Get("topic_command")
	prompt = strings.ReplaceAll(prompt, "{text}", text)

	resp, err := t.llmClient.Call(ctx, "topic_command", "", prompt, false)
	if err != nil {
		return nil, err
	}

	var intent TopicIntent
	if err := json.Unmarshal([]byte(llm.ExtractJSON(resp.Content)), &intent); err != nil {
		return nil, err
	}
	return &intent, nil
}

func (t *TopicCRUD) handleClose(ctx context.Context, msg *models.Message, intent TopicIntent) {
	topic := t.findTopic(ctx, msg.Chat.ID, intent.TopicName)
	if topic == nil {
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			fmt.Sprintf("Топик «%s» не найден.", intent.TopicName))
		return
	}

	_, err := t.bot.CloseForumTopic(ctx, &tgbot.CloseForumTopicParams{
		ChatID:          msg.Chat.ID,
		MessageThreadID: topic.TgThreadID,
	})
	if err != nil {
		slog.Error("close forum topic failed", "error", err)
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"Ошибка при закрытии топика.")
		return
	}

	t.db.ExecContext(ctx, `UPDATE topics SET is_active = 0 WHERE id = ?`, topic.ID)

	t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
		fmt.Sprintf("🔒 Топик «%s» закрыт.", topic.Name))
}

func (t *TopicCRUD) handleRename(ctx context.Context, msg *models.Message, intent TopicIntent) {
	if intent.NewName == "" {
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"Укажи новое название: переименуй топик «Старое» в «Новое»")
		return
	}

	topic := t.findTopic(ctx, msg.Chat.ID, intent.TopicName)
	if topic == nil {
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			fmt.Sprintf("Топик «%s» не найден.", intent.TopicName))
		return
	}

	_, err := t.bot.EditForumTopic(ctx, &tgbot.EditForumTopicParams{
		ChatID:          msg.Chat.ID,
		MessageThreadID: topic.TgThreadID,
		Name:            intent.NewName,
	})
	if err != nil {
		slog.Error("edit forum topic failed", "error", err)
		t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
			"Ошибка при переименовании топика.")
		return
	}

	newSlug := makeSlug(intent.NewName)
	t.db.ExecContext(ctx,
		`UPDATE topics SET name = ?, slug = ? WHERE id = ?`,
		intent.NewName, newSlug, topic.ID)

	t.reply(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID,
		fmt.Sprintf("✏️ Топик переименован в «%s»", intent.NewName))
}

type topicInfo struct {
	ID         int
	TgThreadID int
	Name       string
	Slug       string
}

func (t *TopicCRUD) findTopic(ctx context.Context, chatID int64, nameOrSlug string) *topicInfo {
	row := t.db.QueryRowContext(ctx,
		`SELECT id, tg_thread_id, name, slug FROM topics
		 WHERE group_id = ? AND (name = ? OR slug = ?) AND is_active = 1
		 LIMIT 1`, chatID, nameOrSlug, nameOrSlug)

	var ti topicInfo
	err := row.Scan(&ti.ID, &ti.TgThreadID, &ti.Name, &ti.Slug)
	if err != nil {
		return nil
	}
	return &ti
}

func (t *TopicCRUD) isAdmin(ctx context.Context, chatID int64, userID int64) bool {
	return t.adminCache.IsAdmin(ctx, chatID, userID)
}

func (t *TopicCRUD) reply(ctx context.Context, chatID int64, threadID int, replyToID int, text string) {
	t.bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          chatID,
		MessageThreadID: threadID,
		Text:            text,
		ReplyParameters: &models.ReplyParameters{
			MessageID: replyToID,
		},
	})
}

var cyrToLat = strings.NewReplacer(
	"а", "a", "б", "b", "в", "v", "г", "g", "д", "d", "е", "e", "ё", "e",
	"ж", "zh", "з", "z", "и", "i", "й", "y", "к", "k", "л", "l", "м", "m",
	"н", "n", "о", "o", "п", "p", "р", "r", "с", "s", "т", "t", "у", "u",
	"ф", "f", "х", "kh", "ц", "ts", "ч", "ch", "ш", "sh", "щ", "shch",
	"ы", "y", "э", "e", "ю", "yu", "я", "ya",
)

func makeSlug(name string) string {
	slug := strings.ToLower(name)
	slug = cyrToLat.Replace(slug)
	slug = strings.ReplaceAll(slug, " ", "_")
	slug = strings.ReplaceAll(slug, ".", "")
	slug = strings.ReplaceAll(slug, "-", "_")
	slug = strings.ReplaceAll(slug, "'", "")
	slug = strings.ReplaceAll(slug, "`", "")
	slug = strings.ReplaceAll(slug, "\"", "")
	return slug
}
