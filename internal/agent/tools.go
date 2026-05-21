package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"omsu_bot/internal/buffer"
	"omsu_bot/internal/llm"
	"omsu_bot/internal/telegram"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Define the schemas for tools
var AvailableTools = []llm.Tool{
	{
		Name:        "get_schedule",
		Description: "Получить расписание занятий группы на определенный день.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"date": map[string]interface{}{
					"type":        "string",
					"description": "Дата в формате YYYY-MM-DD",
				},
				"relative_day": map[string]interface{}{
					"type":        "string",
					"description": "Относительный день недели (today, tomorrow, monday, tuesday, wednesday, thursday, friday, saturday)",
				},
				"subgroup": map[string]interface{}{
					"type":        "integer",
					"description": "Номер подгруппы (1 или 2)",
				},
			},
		},
	},
	{
		Name:        "generate_summary",
		Description: "Собрать последние сообщения из указанного топика и предоставить их для суммаризации.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"topic_slug": map[string]interface{}{
					"type":        "string",
					"description": "Слаг или имя топика (например, 'sessiya', 'general')",
				},
			},
			"required": []string{"topic_slug"},
		},
	},
	{
		Name:        "manage_topic",
		Description: "Управление топиками в группе: создание, закрытие, переименование.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Действие (create, close, rename, reopen)",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Название топика (для создания, переименования или закрытия)",
				},
				"new_name": map[string]interface{}{
					"type":        "string",
					"description": "Новое название топика (только при rename)",
				},
			},
			"required": []string{"action", "name"},
		},
	},
	{
		Name:        "moderate_user",
		Description: "Модерация участников группы: мут (mute), бан (ban), размут (unmute).",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Действие модерации (mute, ban, unmute)",
				},
				"username": map[string]interface{}{
					"type":        "string",
					"description": "Имя пользователя в Telegram (например, '@username' или 'username')",
				},
				"duration_minutes": map[string]interface{}{
					"type":        "integer",
					"description": "Длительность ограничения в минутах (по умолчанию 10 минут)",
				},
			},
			"required": []string{"action", "username"},
		},
	},
}

type ToolExecutor struct {
	db             *sql.DB
	bot            *tgbot.Bot
	buffer         *buffer.SummaryBuffer
	usernameCache  *telegram.UsernameCache
	setkaBaseURL   string
	setkaPublicURL string
}

func NewToolExecutor(db *sql.DB, bot *tgbot.Bot, buf *buffer.SummaryBuffer, uc *telegram.UsernameCache, setkaBase, setkaPublic string) *ToolExecutor {
	return &ToolExecutor{
		db:             db,
		bot:            bot,
		buffer:         buf,
		usernameCache:  uc,
		setkaBaseURL:   setkaBase,
		setkaPublicURL: setkaPublic,
	}
}

func (e *ToolExecutor) Execute(ctx context.Context, chatID int64, name string, arguments string) (string, error) {
	slog.Info("Executing tool", "name", name, "chat_id", chatID, "arguments", arguments)
	switch name {
	case "get_schedule":
		return e.getSchedule(ctx, chatID, arguments)
	case "generate_summary":
		return e.generateSummary(ctx, chatID, arguments)
	case "manage_topic":
		return e.manageTopic(ctx, chatID, arguments)
	case "moderate_user":
		return e.moderateUser(ctx, chatID, arguments)
	default:
		return "", fmt.Errorf("unknown tool name: %s", name)
	}
}

func (e *ToolExecutor) getSchedule(ctx context.Context, chatID int64, argsJSON string) (string, error) {
	var args struct {
		Date        string `json:"date"`
		RelativeDay string `json:"relative_day"`
		Subgroup    int    `json:"subgroup"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	var omsuGroupID int
	err := e.db.QueryRowContext(ctx, "SELECT omsu_group_id FROM groups WHERE chat_id = ?", chatID).Scan(&omsuGroupID)
	if err != nil {
		return "", fmt.Errorf("failed to find omsu group id for this chat: %w", err)
	}

	date := resolveDate(args.Date, args.RelativeDay)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	url := fmt.Sprintf("%s/api/v1/schedule/group/%d/day?date=%s", e.setkaBaseURL, omsuGroupID, date)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to query setka schedule: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var wrapper struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil || !wrapper.Success {
		return fmt.Sprintf("Ошибка получения расписания с сервера: %s", string(body)), nil
	}

	return string(wrapper.Data), nil
}

func (e *ToolExecutor) generateSummary(ctx context.Context, chatID int64, argsJSON string) (string, error) {
	var args struct {
		TopicSlug string `json:"topic_slug"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	slug := makeSlug(args.TopicSlug)

	var threadID int
	err := e.db.QueryRowContext(ctx, "SELECT tg_thread_id FROM topics WHERE slug = ? OR name = ?", slug, args.TopicSlug).Scan(&threadID)
	if err != nil {
		return "Ошибка: топик не найден в базе данных.", nil
	}

	messages := e.buffer.GetMessages(chatID, threadID)
	if messages == "" && threadID != 0 {
		messages = e.buffer.GetMessages(chatID, 0)
	}
	if messages == "" {
		return "Нет сообщений в буфере для этого топика.", nil
	}

	return messages, nil
}

func (e *ToolExecutor) manageTopic(ctx context.Context, chatID int64, argsJSON string) (string, error) {
	var args struct {
		Action  string `json:"action"`
		Name    string `json:"name"`
		NewName string `json:"new_name"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	switch args.Action {
	case "create":
		slug := makeSlug(args.Name)
		forum, err := e.bot.CreateForumTopic(ctx, &tgbot.CreateForumTopicParams{
			ChatID: chatID,
			Name:   args.Name,
		})
		if err != nil {
			return fmt.Sprintf("Ошибка при создании топика в Telegram: %v", err), nil
		}

		_, err = e.db.ExecContext(ctx,
			`INSERT INTO topics (group_id, tg_thread_id, name, slug, aliases, description, hashtags, is_active, created_at)
			 VALUES (?, ?, ?, ?, '[]', '', '[]', 1, CURRENT_TIMESTAMP)`,
			chatID, forum.MessageThreadID, args.Name, slug)
		if err != nil {
			slog.Error("failed to save topic to DB", "error", err)
		}
		return fmt.Sprintf("Топик «%s» успешно создан с ID %d", args.Name, forum.MessageThreadID), nil

	case "close":
		var id, tgThreadID int
		var name string
		err := e.db.QueryRowContext(ctx,
			`SELECT id, tg_thread_id, name FROM topics WHERE group_id = ? AND (name = ? OR slug = ?)`, chatID, args.Name, makeSlug(args.Name),
		).Scan(&id, &tgThreadID, &name)
		if err != nil {
			return fmt.Sprintf("Топик «%s» не найден.", args.Name), nil
		}

		_, err = e.bot.CloseForumTopic(ctx, &tgbot.CloseForumTopicParams{
			ChatID:          chatID,
			MessageThreadID: tgThreadID,
		})
		if err != nil {
			return fmt.Sprintf("Ошибка при закрытии топика: %v", err), nil
		}

		e.db.ExecContext(ctx, `UPDATE topics SET is_active = 0 WHERE id = ?`, id)
		return fmt.Sprintf("Топик «%s» успешно закрыт.", name), nil

	case "rename":
		if args.NewName == "" {
			return "Ошибка: не указано новое имя для переименования.", nil
		}
		var id, tgThreadID int
		err := e.db.QueryRowContext(ctx,
			`SELECT id, tg_thread_id FROM topics WHERE group_id = ? AND (name = ? OR slug = ?)`, chatID, args.Name, makeSlug(args.Name),
		).Scan(&id, &tgThreadID)
		if err != nil {
			return fmt.Sprintf("Топик «%s» не найден.", args.Name), nil
		}

		_, err = e.bot.EditForumTopic(ctx, &tgbot.EditForumTopicParams{
			ChatID:          chatID,
			MessageThreadID: tgThreadID,
			Name:            args.NewName,
		})
		if err != nil {
			return fmt.Sprintf("Ошибка при переименовании топика: %v", err), nil
		}

		newSlug := makeSlug(args.NewName)
		e.db.ExecContext(ctx, `UPDATE topics SET name = ?, slug = ? WHERE id = ?`, args.NewName, newSlug, id)
		return fmt.Sprintf("Топик «%s» переименован в «%s».", args.Name, args.NewName), nil

	case "reopen":
		var id, tgThreadID int
		var name string
		err := e.db.QueryRowContext(ctx,
			`SELECT id, tg_thread_id, name FROM topics WHERE group_id = ? AND (name = ? OR slug = ?)`, chatID, args.Name, makeSlug(args.Name),
		).Scan(&id, &tgThreadID, &name)
		if err != nil {
			return fmt.Sprintf("Топик «%s» не найден.", args.Name), nil
		}

		_, err = e.bot.ReopenForumTopic(ctx, &tgbot.ReopenForumTopicParams{
			ChatID:          chatID,
			MessageThreadID: tgThreadID,
		})
		if err != nil {
			return fmt.Sprintf("Ошибка при открытии топика: %v", err), nil
		}

		e.db.ExecContext(ctx, `UPDATE topics SET is_active = 1 WHERE id = ?`, id)
		return fmt.Sprintf("Топик «%s» успешно открыт заново.", name), nil

	default:
		return fmt.Sprintf("Неподдерживаемое действие: %s", args.Action), nil
	}
}

func (e *ToolExecutor) moderateUser(ctx context.Context, chatID int64, argsJSON string) (string, error) {
	var args struct {
		Action          string `json:"action"`
		Username        string `json:"username"`
		DurationMinutes int    `json:"duration_minutes"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	username := strings.TrimPrefix(args.Username, "@")
	userID, ok := e.usernameCache.Get(username)
	if !ok {
		return fmt.Sprintf("Пользователь @%s не найден в кэше юзеров бота. Бот должен увидеть хотя бы одно сообщение от пользователя, чтобы узнать его ID.", username), nil
	}

	if args.DurationMinutes <= 0 {
		args.DurationMinutes = 10
	}

	switch args.Action {
	case "mute":
		untilDate := time.Now().Add(time.Duration(args.DurationMinutes) * time.Minute).Unix()
		_, err := e.bot.RestrictChatMember(ctx, &tgbot.RestrictChatMemberParams{
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
			return fmt.Sprintf("Не удалось ограничить права пользователя в Telegram: %v", err), nil
		}
		return fmt.Sprintf("Пользователь @%s замучен на %d минут.", username, args.DurationMinutes), nil

	case "ban":
		_, err := e.bot.BanChatMember(ctx, &tgbot.BanChatMemberParams{
			ChatID: chatID,
			UserID: userID,
		})
		if err != nil {
			return fmt.Sprintf("Не удалось заблокировать пользователя в Telegram: %v", err), nil
		}
		return fmt.Sprintf("Пользователь @%s успешно заблокирован.", username), nil

	case "unmute":
		_, err := e.bot.RestrictChatMember(ctx, &tgbot.RestrictChatMemberParams{
			ChatID: chatID,
			UserID: userID,
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
			return fmt.Sprintf("Не удалось размутить пользователя в Telegram: %v", err), nil
		}
		return fmt.Sprintf("Пользователь @%s размучен.", username), nil

	default:
		return fmt.Sprintf("Неподдерживаемое действие модерации: %s", args.Action), nil
	}
}

var weekdayMap = map[string]int{
	"sunday": 0, "saturday": 6, "friday": 5, "thursday": 4,
	"wednesday": 3, "tuesday": 2, "monday": 1,
	"воскресенье": 0, "суббота": 6, "пятница": 5, "четверг": 4,
	"среда": 3, "вторник": 2, "понедельник": 1,
}

func resolveDate(date, relativeDate string) string {
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err == nil {
			return date
		}
	}
	now := time.Now()
	switch relativeDate {
	case "today":
		return now.Format("2006-01-02")
	case "tomorrow":
		return now.AddDate(0, 0, 1).Format("2006-01-02")
	case "":
		return ""
	}
	if wd, ok := weekdayMap[relativeDate]; ok {
		diff := (wd - int(now.Weekday()) + 7) % 7
		if diff == 0 {
			diff = 7
		}
		return now.AddDate(0, 0, diff).Format("2006-01-02")
	}
	return now.Format("2006-01-02")
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
