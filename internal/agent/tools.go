package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"omsu_bot/internal/buffer"
	"omsu_bot/internal/classifier"
	"omsu_bot/internal/llm"
	"omsu_bot/internal/telegram"
	"omsu_bot/internal/util"

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
					"type":        "string",
					"description": "Номер подгруппы (1 или 2)",
				},
			},
		},
	},
	{
		Name:        "generate_summary",
		Description: "Собрать последние сообщения из указанного топика (темы, ветки, раздела) и предоставить их для суммаризации. Пользователи могут сказать: 'саммари', 'что тут было', 'о чём говорили'.",
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
		Description: "Управление топиками (темами, ветками, разделами) форума: создание (create), закрытие (close), переименование (rename), открытие (reopen). Пользователи могут сказать: 'создай тему X', 'закрой топик Y', 'переименуй ветку Z в W', 'открой раздел Q'.",
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
		Description: "Модерация участников: мут/замутить/заглушить (mute), бан/забанить/заблокировать (ban), размут/разбан/разблокировать (unmute). Пользователи могут сказать: 'замуть @user', 'забань @user', 'заблокируй @user', 'размуть @user', 'разбань @user', 'разблокируй @user'.",
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
					"type":        "string",
					"description": "Длительность ограничения в минутах (по умолчанию 10 минут)",
				},
			},
			"required": []string{"action", "username"},
		},
	},
	{
		Name:        "run_protocol",
		Description: "Запустить предопределенный сценарий (протокол действий) для администрирования или модерации. Доступные протоколы прописаны в protocols.json.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"protocol_name": map[string]interface{}{
					"type":        "string",
					"description": "Имя запускаемого протокола (например, 'зачистка', 'зачисти_30')",
				},
				"thread_id": map[string]interface{}{
					"type":        "string",
					"description": "ID топика (thread_id) в Telegram, к которому применяется протокол. Для General (основного) топика это 0 или 1.",
				},
				"username": map[string]interface{}{
					"type":        "string",
					"description": "Имя пользователя Telegram (например, '@username' или 'username'), если протокол требует модерации конкретного пользователя.",
				},
			},
			"required": []string{"protocol_name"},
		},
	},
	{
		Name:        "forward_message",
		Description: "Переслать (скопировать) сообщение в другой топик (тему, ветку, раздел). Пользователи могут сказать: 'перешли это в топик X', 'скинь в тему Y', 'отправь в ветку Z', 'перешли в сессию', 'в топик расписание'.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"target_topic": map[string]interface{}{
					"type":        "string",
					"description": "Название или slug целевого топика (например, 'сессия', 'sessiya', 'лекции', 'lab_works')",
				},
			},
			"required": []string{"target_topic"},
		},
	},
}

type protocolAction struct {
	Type            string `json:"type"`
	Count           int    `json:"count,omitempty"`
	Text            string `json:"text,omitempty"`
	DurationMinutes int    `json:"duration_minutes,omitempty"`
}

type protocolDef struct {
	Name    string           `json:"name"`
	Actions []protocolAction `json:"actions"`
}

type protocolsConfig struct {
	Protocols []protocolDef `json:"protocols"`
}

type MediaGroupItem struct {
	MessageID int
	FileID    string
	Caption   string
	MediaType string // "photo", "document", "video", "audio"
}

type MediaGroupEntry struct {
	Mu        sync.Mutex
	Items     []MediaGroupItem
	CreatedAt time.Time
}

type ToolExecutor struct {
	db                  *sql.DB
	bot                 *tgbot.Bot
	buffer              *buffer.SummaryBuffer
	usernameCache       *telegram.UsernameCache
	setkaBaseURL        string
	setkaPublicURL      string
	ProtocolsConfigPath string
	adminChecker        AdminChecker
	mediaGroupMessages  *sync.Map // media_group_id → []MediaGroupItem
	classifier          *classifier.Classifier

	sourceMessageID  int
	replyToMessageID int

	protocolsOnce sync.Once
	protocolsData *protocolsConfig
}

func NewToolExecutor(db *sql.DB, bot *tgbot.Bot, buf *buffer.SummaryBuffer, uc *telegram.UsernameCache, setkaBase, setkaPublic string, adminChecker AdminChecker, mediaGroupMessages *sync.Map, classif *classifier.Classifier) *ToolExecutor {
	return &ToolExecutor{
		db:                 db,
		bot:                bot,
		buffer:             buf,
		usernameCache:      uc,
		setkaBaseURL:       setkaBase,
		setkaPublicURL:     setkaPublic,
		adminChecker:       adminChecker,
		mediaGroupMessages: mediaGroupMessages,
		classifier:         classif,
	}
}

func (e *ToolExecutor) SetMessageContext(sourceMessageID, replyToMessageID int) {
	e.sourceMessageID = sourceMessageID
	e.replyToMessageID = replyToMessageID
}

func (e *ToolExecutor) Execute(ctx context.Context, chatID int64, name string, arguments string) (string, error) {
	slog.Info("Executing tool", "name", name, "chat_id", chatID)
	switch name {
	case "get_schedule":
		return e.getSchedule(ctx, chatID, arguments)
	case "generate_summary":
		return e.generateSummary(ctx, chatID, arguments)
	case "manage_topic":
		return e.manageTopic(ctx, chatID, arguments)
	case "moderate_user":
		return e.moderateUser(ctx, chatID, arguments)
	case "run_protocol":
		return e.runProtocol(ctx, chatID, arguments)
	case "forward_message":
		return e.forwardMessage(ctx, chatID, arguments)
	default:
		return "", fmt.Errorf("unknown tool name: %s", name)
	}
}

func (e *ToolExecutor) getSchedule(ctx context.Context, chatID int64, argsJSON string) (string, error) {
	var args struct {
		Date        string `json:"date"`
		RelativeDay string `json:"relative_day"`
		Subgroup    string `json:"subgroup"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	var omsuGroupID int
	err := e.db.QueryRowContext(ctx, "SELECT omsu_group_id FROM groups WHERE chat_id = ?", chatID).Scan(&omsuGroupID)
	if err != nil || omsuGroupID == 0 {
		return "Группа не привязана к Setka. Администратор может настроить через /settings → Поиск группы в Setka.", nil
	}

	date := util.ResolveDate(args.Date, args.RelativeDay)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	url := fmt.Sprintf("%s/api/v1/schedule/group/%d/day?date=%s", e.setkaBaseURL, omsuGroupID, date)
	if sg, err := strconv.Atoi(args.Subgroup); err == nil && sg > 0 {
		url = fmt.Sprintf("%s&subgroup=%d", url, sg)
	}
	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Get(url)
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

	result := string(wrapper.Data)
	if e.setkaPublicURL != "" {
		base := strings.TrimRight(e.setkaPublicURL, "/")
		var link string
		if omsuGroupID != 0 {
			link = fmt.Sprintf("%s/schedule/group/%d?date=%s", base, omsuGroupID, date)
		} else {
			link = base
		}
		result += fmt.Sprintf("\n\n<a href=\"%s\">[расписание]</a>", link)
	}
	return result, nil
}

func (e *ToolExecutor) generateSummary(ctx context.Context, chatID int64, argsJSON string) (string, error) {
	var args struct {
		TopicSlug string `json:"topic_slug"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	slug := util.MakeSlug(args.TopicSlug)

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
		slug := util.MakeSlug(args.Name)
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
			`SELECT id, tg_thread_id, name FROM topics WHERE group_id = ? AND (name = ? OR slug = ?)`, chatID, args.Name, util.MakeSlug(args.Name),
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
			`SELECT id, tg_thread_id FROM topics WHERE group_id = ? AND (name = ? OR slug = ?)`, chatID, args.Name, util.MakeSlug(args.Name),
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

		newSlug := util.MakeSlug(args.NewName)
		e.db.ExecContext(ctx, `UPDATE topics SET name = ?, slug = ? WHERE id = ?`, args.NewName, newSlug, id)
		return fmt.Sprintf("Топик «%s» переименован в «%s».", args.Name, args.NewName), nil

	case "reopen":
		var id, tgThreadID int
		var name string
		err := e.db.QueryRowContext(ctx,
			`SELECT id, tg_thread_id, name FROM topics WHERE group_id = ? AND (name = ? OR slug = ?)`, chatID, args.Name, util.MakeSlug(args.Name),
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
		DurationMinutes string `json:"duration_minutes"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	username := strings.TrimPrefix(args.Username, "@")
	userID, ok := e.usernameCache.Get(username)
	if !ok {
		return fmt.Sprintf("Пользователь @%s не найден в кэше юзеров бота. Бот должен увидеть хотя бы одно сообщение от пользователя, чтобы узнать его ID.", username), nil
	}

	durationMinutes := 10
	if d, err := strconv.Atoi(args.DurationMinutes); err == nil && d > 0 {
		durationMinutes = d
	}

	if args.Action == "ban" || args.Action == "mute" {
		if e.adminChecker != nil && e.adminChecker.IsOwner(ctx, chatID, userID) {
			return "⛔ Нельзя забанить или замутить владельца группы.", nil
		}
	}

	switch args.Action {
	case "mute":
		untilDate := time.Now().Add(time.Duration(durationMinutes) * time.Minute).Unix()
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
		return fmt.Sprintf("Пользователь @%s замучен на %d минут.", username, durationMinutes), nil

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

func (e *ToolExecutor) runProtocol(ctx context.Context, chatID int64, argsJSON string) (string, error) {
	var args struct {
		ProtocolName string `json:"protocol_name"`
		ThreadID     string `json:"thread_id"`
		Username     string `json:"username"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	threadID := 0
	if t, err := strconv.Atoi(args.ThreadID); err == nil {
		threadID = t
	}

	if e.protocolsData == nil {
		e.protocolsOnce.Do(func() {
			path := e.ProtocolsConfigPath
			if path == "" {
				path = "protocols.json"
			}
			content, err := os.ReadFile(path)
			if err != nil {
				slog.Error("failed to read protocols file", "error", err)
				return
			}
			var cfg protocolsConfig
			if err := json.Unmarshal(content, &cfg); err != nil {
				slog.Error("failed to parse protocols config", "error", err)
				return
			}
			e.protocolsData = &cfg
			slog.Info("protocols config loaded", "count", len(cfg.Protocols))
		})
	}
	if e.protocolsData == nil {
		return "", fmt.Errorf("failed to load protocols configuration")
	}

	var foundProto *protocolDef
	for i := range e.protocolsData.Protocols {
		if e.protocolsData.Protocols[i].Name == args.ProtocolName {
			foundProto = &e.protocolsData.Protocols[i]
			break
		}
	}
	if foundProto == nil {
		return fmt.Sprintf("Протокол '%s' не найден в конфигурации.", args.ProtocolName), nil
	}

	var logMsg []string
	for _, action := range foundProto.Actions {
		switch action.Type {
		case "delete_messages":
			rows, err := e.db.QueryContext(ctx, "SELECT message_id FROM message_buffer WHERE chat_id = ? AND thread_id = ? ORDER BY created_at DESC LIMIT ?", chatID, threadID, action.Count)
			if err != nil {
				logMsg = append(logMsg, fmt.Sprintf("Ошибка получения сообщений для удаления: %v", err))
				continue
			}
			var messageIDs []int
			for rows.Next() {
				var msgID int
				if err := rows.Scan(&msgID); err == nil && msgID > 0 {
					messageIDs = append(messageIDs, msgID)
				}
			}
			rows.Close()

			var deletedCount int
			for _, msgID := range messageIDs {
				var err error
				if e.bot != nil {
					_, err = e.bot.DeleteMessage(ctx, &tgbot.DeleteMessageParams{
						ChatID:    chatID,
						MessageID: msgID,
					})
				}
				if err == nil {
					deletedCount++
				} else {
					slog.Error("failed to delete message", "chat_id", chatID, "message_id", msgID, "error", err)
				}
			}

			if len(messageIDs) > 0 {
				query := "DELETE FROM message_buffer WHERE chat_id = ? AND thread_id = ? AND message_id IN ("
				sqlArgs := []interface{}{chatID, threadID}
				for i, id := range messageIDs {
					if i > 0 {
						query += ","
					}
					query += "?"
					sqlArgs = append(sqlArgs, id)
				}
				query += ")"
				_, err = e.db.ExecContext(ctx, query, sqlArgs...)
				if err != nil {
					slog.Error("failed to delete messages from database", "error", err)
				}
				e.buffer.RemoveMessages(chatID, threadID, messageIDs)
			}
			logMsg = append(logMsg, fmt.Sprintf("Удалено сообщений из Telegram и базы данных: %d", deletedCount))

		case "close_topic":
			var err error
			if e.bot != nil {
				_, err = e.bot.CloseForumTopic(ctx, &tgbot.CloseForumTopicParams{
					ChatID:          chatID,
					MessageThreadID: threadID,
				})
			}
			if err != nil {
				logMsg = append(logMsg, fmt.Sprintf("Ошибка закрытия топика: %v", err))
			} else {
				e.db.ExecContext(ctx, "UPDATE topics SET is_active = 0 WHERE group_id = ? AND tg_thread_id = ?", chatID, threadID)
				logMsg = append(logMsg, "Топик успешно закрыт.")
			}

		case "open_topic":
			var err error
			if e.bot != nil {
				_, err = e.bot.ReopenForumTopic(ctx, &tgbot.ReopenForumTopicParams{
					ChatID:          chatID,
					MessageThreadID: threadID,
				})
			}
			if err != nil {
				logMsg = append(logMsg, fmt.Sprintf("Ошибка открытия топика: %v", err))
			} else {
				e.db.ExecContext(ctx, "UPDATE topics SET is_active = 1 WHERE group_id = ? AND tg_thread_id = ?", chatID, threadID)
				logMsg = append(logMsg, "Топик успешно открыт.")
			}

		case "send_message":
			var err error
			if e.bot != nil {
				_, err = e.bot.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:          chatID,
					MessageThreadID: threadID,
					Text:            util.StripMarkdown(action.Text),
					ParseMode:       models.ParseModeHTML,
				})
			}
			if err != nil {
				logMsg = append(logMsg, fmt.Sprintf("Ошибка отправки сообщения: %v", err))
			} else {
				logMsg = append(logMsg, "Сообщение отправлено.")
			}

		case "mute_user":
			usernameClean := strings.TrimPrefix(args.Username, "@")
			if usernameClean == "" {
				logMsg = append(logMsg, "Ошибка: не указано имя пользователя для мута.")
				continue
			}
			userID, ok := e.usernameCache.Get(usernameClean)
			if !ok {
				logMsg = append(logMsg, fmt.Sprintf("Ошибка: пользователь @%s не найден в кэше.", usernameClean))
				continue
			}
			duration := action.DurationMinutes
			if duration <= 0 {
				duration = 10
			}
			untilDate := time.Now().Add(time.Duration(duration) * time.Minute).Unix()
			var err error
			if e.bot != nil {
				_, err = e.bot.RestrictChatMember(ctx, &tgbot.RestrictChatMemberParams{
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
			}
			if err != nil {
				logMsg = append(logMsg, fmt.Sprintf("Ошибка ограничения пользователя @%s: %v", usernameClean, err))
			} else {
				logMsg = append(logMsg, fmt.Sprintf("Пользователь @%s замучен на %d минут.", usernameClean, duration))
			}

		case "ban_user":
			usernameClean := strings.TrimPrefix(args.Username, "@")
			if usernameClean == "" {
				logMsg = append(logMsg, "Ошибка: не указано имя пользователя для бана.")
				continue
			}
			userID, ok := e.usernameCache.Get(usernameClean)
			if !ok {
				logMsg = append(logMsg, fmt.Sprintf("Ошибка: пользователь @%s не найден в кэше.", usernameClean))
				continue
			}
			var err error
			if e.bot != nil {
				_, err = e.bot.BanChatMember(ctx, &tgbot.BanChatMemberParams{
					ChatID: chatID,
					UserID: userID,
				})
			}
			if err != nil {
				logMsg = append(logMsg, fmt.Sprintf("Ошибка бана пользователя @%s: %v", usernameClean, err))
			} else {
				logMsg = append(logMsg, fmt.Sprintf("Пользователь @%s заблокирован.", usernameClean))
			}
		}
	}

	return fmt.Sprintf("Протокол '%s' выполнен. Результаты:\n%s", args.ProtocolName, strings.Join(logMsg, "\n")), nil
}

func (e *ToolExecutor) forwardMessage(ctx context.Context, chatID int64, argsJSON string) (string, error) {
	var args struct {
		TargetTopic string `json:"target_topic"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	slug := util.MakeSlug(args.TargetTopic)
	var tgThreadID int
	var topicName string
	err := e.db.QueryRowContext(ctx,
		`SELECT tg_thread_id, name FROM topics WHERE group_id = ? AND (slug = ? OR name = ?) AND is_active = 1 LIMIT 1`,
		chatID, slug, args.TargetTopic,
	).Scan(&tgThreadID, &topicName)
	if err != nil {
		// Fuzzy fallback: try matching by partial name or slug
		fuzzySlug := strings.ReplaceAll(strings.ToLower(args.TargetTopic), " ", "_")
		err = e.db.QueryRowContext(ctx,
			`SELECT tg_thread_id, name FROM topics WHERE group_id = ? AND is_active = 1
			 AND (LOWER(name) LIKE '%' || ? || '%' OR slug LIKE '%' || ? || '%')
			 ORDER BY LENGTH(name) LIMIT 1`,
			chatID, fuzzySlug, fuzzySlug,
		).Scan(&tgThreadID, &topicName)
		if err != nil {
			return fmt.Sprintf("Топик «%s» не найден. Проверь название через /topics.", args.TargetTopic), nil
		}
	}

	forwardMsgID := e.replyToMessageID
	if forwardMsgID == 0 {
		forwardMsgID = e.sourceMessageID
	}
	if forwardMsgID == 0 {
		return "Не удалось определить сообщение для пересылки. Ответь на сообщение, которое нужно переслать.", nil
	}

	if e.bot == nil {
		return "Ошибка: бот не инициализирован.", nil
	}

	slog.Debug("forward_message start",
		"target_topic", args.TargetTopic,
		"forward_msg_id", forwardMsgID,
		"source_msg_id", e.sourceMessageID,
		"reply_to_msg_id", e.replyToMessageID,
		"chat_id", chatID,
	)

	// Collect media group items if the replied-to message was part of an album
	var groupItems []MediaGroupItem
	if e.mediaGroupMessages != nil {
		e.mediaGroupMessages.Range(func(key, value interface{}) bool {
			if entry, ok := value.(*MediaGroupEntry); ok {
				for _, item := range entry.Items {
					if item.MessageID == forwardMsgID {
						groupItems = entry.Items
						return false
					}
				}
			}
			return true
		})
	}
	slog.Debug("forward_message in-memory lookup", "group_items", len(groupItems))

	// Fallback: query DB when in-memory map is cold (after restart)
	if len(groupItems) == 0 {
		rows, err := e.db.QueryContext(ctx,
				`SELECT mg.media_group_id, mg.message_id, mg.file_id, mg.caption, mg.media_type
			 FROM media_group_items mg
			 WHERE mg.media_group_id = (
				SELECT media_group_id FROM media_group_items WHERE message_id = ?
			 ) AND mg.chat_id = ?
			 ORDER BY mg.message_id`,
			forwardMsgID, chatID,
		)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var item MediaGroupItem
				var groupID string
				rows.Scan(&groupID, &item.MessageID, &item.FileID, &item.Caption, &item.MediaType)
				groupItems = append(groupItems, item)
			}
			slog.Debug("forward_message DB fallback", "forward_msg_id", forwardMsgID, "group_items", len(groupItems))
		} else {
			slog.Debug("forward_message DB query failed", "error", err)
		}
	}

	slog.Debug("forward_message media group lookup", "forward_msg_id", forwardMsgID, "group_items", len(groupItems))

	// Generate hashtags from caption text
	captionText := ""
	for _, item := range groupItems {
		if item.Caption != "" {
			captionText = item.Caption
			break
		}
	}
	var hashtagStr string
	if e.classifier != nil && captionText != "" {
		hashtags, err := e.generateHashtags(ctx, chatID, captionText)
		if err == nil && len(hashtags) > 0 {
			hashtagStr = "\n" + hashtagsToText(hashtags)
		}
	}

	var lastLink string

	if len(groupItems) > 1 {
		slog.Debug("forward_message sending album", "count", len(groupItems), "first_caption", captionText[:min(len(captionText), 100)])
		var media []models.InputMedia
		for i, item := range groupItems {
			cap := ""
			if i == 0 {
				cap = item.Caption + hashtagStr
			}
			mediaType := item.MediaType
			if mediaType == "" {
				mediaType = "photo"
			}
			switch mediaType {
			case "document":
				media = append(media, &models.InputMediaDocument{
					Media:   item.FileID,
					Caption: cap,
				})
			case "video":
				media = append(media, &models.InputMediaVideo{
					Media:                 item.FileID,
					Caption:               cap,
					ShowCaptionAboveMedia: true,
				})
			case "audio":
				media = append(media, &models.InputMediaAudio{
					Media:   item.FileID,
					Caption: cap,
				})
			default:
				media = append(media, &models.InputMediaPhoto{
					Media:                 item.FileID,
					Caption:               cap,
					ShowCaptionAboveMedia: true,
				})
			}
			short := item.FileID
			if len(short) > 20 {
				short = short[:20]
			}
			slog.Debug("forward_message album item", "i", i, "file_id", short, "media_type", mediaType, "has_caption", cap != "")
		}
		result, err := e.bot.SendMediaGroup(ctx, &tgbot.SendMediaGroupParams{
			ChatID:          chatID,
			MessageThreadID: tgThreadID,
			Media:           media,
		})
		if err != nil {
			slog.Error("failed to send media group in forward tool", "error", err)
			return fmt.Sprintf("Не удалось переслать альбом: %v", err), nil
		}
		if len(result) > 0 {
			lastLink = util.ChatLink(chatID, result[0].ID)
		}
		slog.Debug("forward_message album sent", "result_count", len(result))
		return fmt.Sprintf("Альбом из %d элементов переслан в топик «%s». %s", len(groupItems), topicName, lastLink), nil
	}

	// Single message: copy
	slog.Debug("forward_message single copy", "msg_id", forwardMsgID, "to_thread", tgThreadID)
	result, err := e.bot.CopyMessage(ctx, &tgbot.CopyMessageParams{
		ChatID:          chatID,
		FromChatID:      fmt.Sprintf("%d", chatID),
		MessageID:       forwardMsgID,
		MessageThreadID: tgThreadID,
	})
	if err != nil {
		slog.Error("failed to copy message in forward tool", "error", err, "message_id", forwardMsgID)
		return fmt.Sprintf("Не удалось переслать сообщение: %v", err), nil
	}
	lastLink = util.ChatLink(chatID, result.ID)
	slog.Debug("forward_message single copy done", "result_id", result.ID)

	return fmt.Sprintf("Сообщение переслано в топик «%s». %s", topicName, lastLink), nil
}

func (e *ToolExecutor) generateHashtags(ctx context.Context, chatID int64, text string) ([]string, error) {
	result, err := e.classifier.ClassifyMessage(ctx, chatID, text, "")
	if err != nil {
		return nil, err
	}
	return result.Hashtags, nil
}

func hashtagsToText(tags []string) string {
	var sb strings.Builder
	for i, t := range tags {
		t = strings.ReplaceAll(t, " ", "-")
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString("#" + t)
	}
	return sb.String()
}
