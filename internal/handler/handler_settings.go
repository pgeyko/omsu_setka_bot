package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"omsu_bot/internal/db"
	"omsu_bot/internal/telegram"
)

type SettingsHandler struct {
	db               *sql.DB
	sessionStore     *telegram.SessionStore
	adminCache       *telegram.AdminCache
	setkaBaseURL     string
	setkaAdminKey    string
	webhookSecret    string
	setkaPublicURL   string
	globalVoice      func() bool
	globalPhoto      func() bool
}

func NewSettingsHandler(db *sql.DB, sessionStore *telegram.SessionStore, adminCache *telegram.AdminCache, setkaBaseURL, setkaAdminKey, webhookSecret, setkaPublicURL string, globalVoice, globalPhoto func() bool) *SettingsHandler {
	return &SettingsHandler{
		db:               db,
		sessionStore:     sessionStore,
		adminCache:       adminCache,
		setkaBaseURL:     setkaBaseURL,
		setkaAdminKey:    setkaAdminKey,
		webhookSecret:    webhookSecret,
		setkaPublicURL:   setkaPublicURL,
		globalVoice:      globalVoice,
		globalPhoto:      globalPhoto,
	}
}

type setkaSearchResult struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	RealID int    `json:"real_group_id"`
}

type setkaBFFResponse struct {
	Success bool                `json:"success"`
	Data    []setkaSearchResult `json:"data"`
	Error   string              `json:"error,omitempty"`
}

func (h *SettingsHandler) isAuthorized(ctx context.Context, chatID int64, userID int64) bool {
	// First check superadmin
	d := &db.DB{DB: h.db}
	isSuper, err := d.IsSuperadmin(ctx, userID)
	if err == nil && isSuper {
		return true
	}
	// Then check group admin
	return h.adminCache.IsAdmin(ctx, chatID, userID)
}

func (h *SettingsHandler) HandleSettingsCommand(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	msg := update.Message
	chatID := msg.Chat.ID
	userID := msg.From.ID

	slog.Info("settings command", "user", userID, "chat", chatID, "type", msg.Chat.Type)

	if string(msg.Chat.Type) == "private" {
		h.sendMessage(ctx, b, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Настройки доступны только внутри группы.",
		})
		slog.Info("settings: private chat blocked", "user", userID)
		return
	}

	if !h.isAuthorized(ctx, chatID, userID) {
		slog.Info("settings: unauthorized", "user", userID, "chat", chatID)
		h.sendMessage(ctx, b, &tgbot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: msg.MessageThreadID,
			Text:            "⛔ У вас нет прав для изменения настроек этой группы.",
		})
		return
	}

	slog.Info("settings: sending main menu", "chat", chatID)
	h.sendMainMenu(ctx, b, chatID, msg.MessageThreadID, "⚙️ <b>Настройки группы</b>\n\nВыберите раздел для редактирования:")
}

func (h *SettingsHandler) sendMainMenu(ctx context.Context, b *tgbot.Bot, chatID int64, threadID int, text string) {
	keyboard := [][]models.InlineKeyboardButton{
		{
			{Text: "📖 База знаний", CallbackData: "settings:menu:kb"},
			{Text: "👤 Личность", CallbackData: "settings:menu:persona"},
		},
		{
			{Text: "📅 Расписание Setka", CallbackData: "settings:menu:setka"},
			{Text: "⚙️ Инструменты", CallbackData: "settings:menu:tools"},
		},
		{
			{Text: "❌ Закрыть", CallbackData: "settings:menu:close"},
		},
	}

	params := &tgbot.SendMessageParams{
		ChatID:          chatID,
		MessageThreadID: threadID,
		Text:            text,
		ParseMode:       models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: keyboard,
		},
	}
	h.sendMessage(ctx, b, params)
}

func (h *SettingsHandler) editMessage(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int, text string, keyboard [][]models.InlineKeyboardButton) {
	if b == nil {
		return
	}
	if _, err := b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: keyboard,
		},
	}); err != nil {
		slog.Warn("settings: editMessage failed", "chat", chatID, "msg_id", messageID, "error", err)
	}
}

func (h *SettingsHandler) sendMessage(ctx context.Context, b *tgbot.Bot, params *tgbot.SendMessageParams) {
	if b == nil {
		return
	}
	_, _ = b.SendMessage(ctx, params)
}

func (h *SettingsHandler) HandleCallbackQuery(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if b == nil || update.CallbackQuery == nil {
		slog.Warn("settings callback: b or cb is nil")
		return
	}
	cb := update.CallbackQuery

	slog.Info("settings callback received", "data", cb.Data, "from", cb.From.ID)

	// Always answer callback to dismiss Telegram loading state
	defer b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: cb.ID,
	})

	data := cb.Data
	if !strings.HasPrefix(data, "settings:") {
		slog.Debug("settings callback: not our prefix", "data", data)
		return
	}

	// Extract chat/message from callback, handling inaccessible messages
	mim := cb.Message
	if mim.Message == nil {
		slog.Warn("settings callback: message is inaccessible", "data", data, "has_inaccessible", mim.InaccessibleMessage != nil)
		return
	}
	chatID := mim.Message.Chat.ID
	messageID := mim.Message.ID
	userID := cb.From.ID

	if !h.isAuthorized(ctx, chatID, userID) {
		slog.Info("settings callback: unauthorized", "user", userID, "chat", chatID)
		return
	}

	action := strings.TrimPrefix(data, "settings:")
	slog.Info("settings callback: executing action", "action", action, "chat", chatID)

	switch {
	case action == "menu:main":
		h.showMainScreen(ctx, b, chatID, messageID)
	case action == "menu:kb":
		h.showKBScreen(ctx, b, chatID, messageID)
	case action == "menu:persona":
		h.showPersonaScreen(ctx, b, chatID, messageID)
	case action == "menu:setka":
		h.showSetkaScreen(ctx, b, chatID, messageID)
	case action == "menu:tools":
		h.showToolsScreen(ctx, b, chatID, messageID)
	case action == "menu:close":
		_, _ = b.DeleteMessage(ctx, &tgbot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: messageID,
		})
	case action == "kb:write":
		h.sessionStore.Set(chatID, userID, telegram.StateWaitingForKB)
		h.editMessage(ctx, b, chatID, messageID, "📖 <b>Запись базы знаний</b>\n\nОтправьте текст базы знаний одним сообщением. Старый текст будет перезаписан.\n\nИспользуйте команду /cancel для отмены.", [][]models.InlineKeyboardButton{
			{{Text: "⬅️ Назад", CallbackData: "settings:menu:kb"}},
		})
	case action == "kb:clear":
		filePath := fmt.Sprintf("data/groups/%d/knowledge_base.txt", chatID)
		_ = os.Remove(filePath)
		h.sessionStore.Clear(chatID, userID)
		h.editMessage(ctx, b, chatID, messageID, "🗑️ <b>База знаний очищена.</b>", [][]models.InlineKeyboardButton{
			{{Text: "⬅️ Назад", CallbackData: "settings:menu:kb"}},
		})
	case action == "persona:write":
		h.sessionStore.Set(chatID, userID, telegram.StateWaitingForPersona)
		h.editMessage(ctx, b, chatID, messageID, "👤 <b>Изменение Persona.md</b>\n\nОтправьте новый markdown текст для Persona.md.\n\nИспользуйте команду /cancel для отмены.", [][]models.InlineKeyboardButton{
			{{Text: "⬅️ Назад", CallbackData: "settings:menu:persona"}},
		})
	case action == "prompt:write":
		h.sessionStore.Set(chatID, userID, telegram.StateWaitingForPrompt)
		h.editMessage(ctx, b, chatID, messageID, "✍️ <b>Изменение System Prompt</b>\n\nОтправьте новый текст системных инструкций (system_prompt.txt).\n\nИспользуйте команду /cancel для отмены.", [][]models.InlineKeyboardButton{
			{{Text: "⬅️ Назад", CallbackData: "settings:menu:persona"}},
		})
	case action == "setka:search":
		h.sessionStore.Set(chatID, userID, telegram.StateWaitingForScheduleSearch)
		h.editMessage(ctx, b, chatID, messageID, "🔍 <b>Поиск расписания Setka</b>\n\nВведите название группы (например, <i>ИВТ-101</i>) для поиска в Setka.\n\nИспользуйте команду /cancel для отмены.", [][]models.InlineKeyboardButton{
			{{Text: "⬅️ Назад", CallbackData: "settings:menu:setka"}},
		})
	case action == "setka:unlink":
		_, err := h.db.ExecContext(ctx, "UPDATE groups SET omsu_group_id = 0 WHERE chat_id = ?", chatID)
		if err != nil {
			slog.Error("failed to unlink Setka group", "error", err, "chat_id", chatID)
		}
		go func() {
			_, _ = telegram.RegisterWebhooksWithSetka(context.Background(), h.db, h.setkaBaseURL, h.setkaAdminKey, h.webhookSecret, h.setkaPublicURL)
		}()
		h.showSetkaScreen(ctx, b, chatID, messageID)
	case strings.HasPrefix(action, "setka_sel:"):
		idStr := strings.TrimPrefix(action, "setka_sel:")
		id, err := strconv.Atoi(idStr)
		if err == nil {
			_, err = h.db.ExecContext(ctx, "UPDATE groups SET omsu_group_id = ? WHERE chat_id = ?", id, chatID)
			if err != nil {
				slog.Error("failed to update omsu_group_id", "error", err, "chat_id", chatID)
			}
			go func() {
		_, _ = telegram.RegisterWebhooksWithSetka(context.Background(), h.db, h.setkaBaseURL, h.setkaAdminKey, h.webhookSecret, h.setkaPublicURL)
			}()
		}
		h.sessionStore.Clear(chatID, userID)
		h.showSetkaScreen(ctx, b, chatID, messageID)
	case action == "toggle:photo_processing":
		features := h.LoadFeatures(chatID)
		auto := features["enable_photo_processing"]
		mention := features["photo_on_mention"]
		// Cycle: off → auto → mention → off
		if !auto && !mention {
			features["enable_photo_processing"] = true  // → auto
			features["photo_on_mention"] = false
		} else if auto && !mention {
			features["enable_photo_processing"] = true  // → mention
			features["photo_on_mention"] = true
		} else {
			features["enable_photo_processing"] = false // → off
			features["photo_on_mention"] = false
		}
		_ = h.saveFeatures(chatID, features)
		h.showToolsScreen(ctx, b, chatID, messageID)

	case strings.HasPrefix(action, "toggle:"):
		feature := strings.TrimPrefix(action, "toggle:")
		features := h.LoadFeatures(chatID)
		if val, exists := features["enable_"+feature]; exists {
			features["enable_"+feature] = !val
		} else {
			features["enable_"+feature] = false
		}
		_ = h.saveFeatures(chatID, features)
		h.showToolsScreen(ctx, b, chatID, messageID)
	case action == "planned:deadline_digest":
		b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "🕐 Будет добавлено в одном из следующих обновлений",
			ShowAlert:       true,
		})
	case action == "planned:dead_topic":
		b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "🕐 Будет добавлено в одном из следующих обновлений",
			ShowAlert:       true,
		})
	}
}

func (h *SettingsHandler) showMainScreen(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int) {
	keyboard := [][]models.InlineKeyboardButton{
		{
			{Text: "📖 База знаний", CallbackData: "settings:menu:kb"},
			{Text: "👤 Личность", CallbackData: "settings:menu:persona"},
		},
		{
			{Text: "📅 Расписание Setka", CallbackData: "settings:menu:setka"},
			{Text: "⚙️ Инструменты", CallbackData: "settings:menu:tools"},
		},
		{
			{Text: "❌ Закрыть", CallbackData: "settings:menu:close"},
		},
	}
	h.editMessage(ctx, b, chatID, messageID, "⚙️ <b>Настройки группы</b>\n\nВыберите раздел для редактирования:", keyboard)
}

func (h *SettingsHandler) showKBScreen(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int) {
	keyboard := [][]models.InlineKeyboardButton{
		{
			{Text: "✍️ Записать", CallbackData: "settings:kb:write"},
			{Text: "🗑️ Очистить", CallbackData: "settings:kb:clear"},
		},
		{
			{Text: "⬅️ Назад", CallbackData: "settings:menu:main"},
		},
	}
	h.editMessage(ctx, b, chatID, messageID, "📖 <b>Управление базой знаний группы</b>\n\nЗдесь вы можете изменить или очистить базу знаний группы. База знаний содержит ссылки, контакты и правила группы.", keyboard)
}

func (h *SettingsHandler) showPersonaScreen(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int) {
	keyboard := [][]models.InlineKeyboardButton{
		{
			{Text: "✍️ Изменить Persona", CallbackData: "settings:persona:write"},
			{Text: "✍️ Изменить System Prompt", CallbackData: "settings:prompt:write"},
		},
		{
			{Text: "⬅️ Назад", CallbackData: "settings:menu:main"},
		},
	}
	h.editMessage(ctx, b, chatID, messageID, "👤 <b>Настройка личности и промптов</b>\n\nВы можете настроить имя, стиль общения и системные инструкции (промпты) для бота.", keyboard)
}

func (h *SettingsHandler) showSetkaScreen(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int) {
	var omsuGroupID int
	var title string
	_ = h.db.QueryRowContext(ctx, "SELECT title, COALESCE(omsu_group_id, 0) FROM groups WHERE chat_id = ?", chatID).Scan(&title, &omsuGroupID)

	setkaStr := "отсутствует"
	if omsuGroupID > 0 {
		setkaStr = strconv.Itoa(omsuGroupID)
	}

	keyboard := [][]models.InlineKeyboardButton{
		{
			{Text: "🔍 Поиск группы", CallbackData: "settings:setka:search"},
			{Text: "❌ Отвязать", CallbackData: "settings:setka:unlink"},
		},
		{
			{Text: "⬅️ Назад", CallbackData: "settings:menu:main"},
		},
	}
	text := fmt.Sprintf("📅 <b>Интеграция расписания Setka</b>\n\nТекущая группа: <b>%s</b>\nПривязанный ID Setka: <b>%s</b>\n\nДля автоматического получения расписания и анонсов изменений привяжите группу.", title, setkaStr)
	h.editMessage(ctx, b, chatID, messageID, text, keyboard)
}

func (h *SettingsHandler) showToolsScreen(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int) {
	features := h.LoadFeatures(chatID)

	tick := func(key string) string {
		if features[key] {
			return "✅"
		}
		return "❌"
	}

	keyboard := [][]models.InlineKeyboardButton{
		{
			{Text: fmt.Sprintf("Расписание: %s", tick("enable_schedule")), CallbackData: "settings:toggle:schedule"},
		},
		{
			{Text: fmt.Sprintf("Саммари: %s", tick("enable_summary")), CallbackData: "settings:toggle:summary"},
		},
		{
			{Text: fmt.Sprintf("Модерация (общая): %s", tick("enable_moderation")), CallbackData: "settings:toggle:moderation"},
		},
		{
			{Text: fmt.Sprintf("Матем. капча: %s", tick("enable_captcha")), CallbackData: "settings:toggle:captcha"},
		},
		{
			{Text: fmt.Sprintf("Фильтр ссылок: %s", tick("enable_link_filter")), CallbackData: "settings:toggle:link_filter"},
		},
		{
			{Text: fmt.Sprintf("Флуд-контроль: %s", tick("enable_flood_control")), CallbackData: "settings:toggle:flood_control"},
		},
	}

	// Voice transcription — show only if globally enabled
	if h.globalVoice == nil || h.globalVoice() {
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: fmt.Sprintf("Расшифровка аудио: %s", tick("enable_voice_transcription")), CallbackData: "settings:toggle:voice_transcription"},
		})
	}

	// Photo processing — three-state toggle: off / auto / via @mention
	if h.globalPhoto == nil || h.globalPhoto() {
		photoLabel := "Обработка фото: ❌"
		if features["enable_photo_processing"] && !features["photo_on_mention"] {
			photoLabel = "Обработка фото: ✅"
		} else if features["enable_photo_processing"] && features["photo_on_mention"] {
			photoLabel = "Обработка фото: ✅ @"
		}
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: photoLabel, CallbackData: "settings:toggle:photo_processing"},
		})
	}

	// Planned features (always visible, info-only)
	keyboard = append(keyboard,
		[]models.InlineKeyboardButton{
			{Text: "📅 Дедлайн-дайджест (скоро)", CallbackData: "settings:planned:deadline_digest"},
		},
		[]models.InlineKeyboardButton{
			{Text: "🛌 Детектор мёртвых топиков (скоро)", CallbackData: "settings:planned:dead_topic"},
		},
		[]models.InlineKeyboardButton{
			{Text: "⬅️ Назад", CallbackData: "settings:menu:main"},
		},
	)

	h.editMessage(ctx, b, chatID, messageID, "⚙️ <b>Управление функциями и инструментами ИИ</b>\n\nВключите или отключите определенные функции ИИ-ассистента для этого чата:", keyboard)
}

func (h *SettingsHandler) HandleAdminInput(ctx context.Context, b *tgbot.Bot, update *models.Update, state telegram.SessionState) {
	if update.Message == nil {
		return
	}
	msg := update.Message
	chatID := msg.Chat.ID
	userID := msg.From.ID

	if strings.TrimSpace(msg.Text) == "/cancel" {
		h.sessionStore.Clear(chatID, userID)
		h.sendMessage(ctx, b, &tgbot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: msg.MessageThreadID,
			Text:            "❌ Изменения отменены.",
		})
		return
	}

	switch state {
	case telegram.StateWaitingForKB:
		dir := fmt.Sprintf("data/groups/%d", chatID)
		if err := os.MkdirAll(dir, 0755); err != nil {
			h.sendMessage(ctx, b, &tgbot.SendMessageParams{ChatID: chatID, MessageThreadID: msg.MessageThreadID, Text: "❌ Ошибка при создании директории."})
			return
		}
		filePath := filepath.Join(dir, "knowledge_base.txt")
		if err := os.WriteFile(filePath, []byte(msg.Text), 0644); err != nil {
			h.sendMessage(ctx, b, &tgbot.SendMessageParams{ChatID: chatID, MessageThreadID: msg.MessageThreadID, Text: "❌ Ошибка при записи файла."})
			return
		}
		h.sessionStore.Clear(chatID, userID)
		h.sendMessage(ctx, b, &tgbot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: msg.MessageThreadID,
			Text:            "✅ База знаний успешно сохранена.",
		})

	case telegram.StateWaitingForPersona:
		dir := fmt.Sprintf("data/groups/%d", chatID)
		if err := os.MkdirAll(dir, 0755); err != nil {
			h.sendMessage(ctx, b, &tgbot.SendMessageParams{ChatID: chatID, MessageThreadID: msg.MessageThreadID, Text: "❌ Ошибка при создании директории."})
			return
		}
		filePath := filepath.Join(dir, "persona.md")
		if err := os.WriteFile(filePath, []byte(msg.Text), 0644); err != nil {
			h.sendMessage(ctx, b, &tgbot.SendMessageParams{ChatID: chatID, MessageThreadID: msg.MessageThreadID, Text: "❌ Ошибка при записи файла."})
			return
		}
		h.sessionStore.Clear(chatID, userID)
		h.sendMessage(ctx, b, &tgbot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: msg.MessageThreadID,
			Text:            "✅ Persona.md успешно обновлена.",
		})

	case telegram.StateWaitingForPrompt:
		dir := fmt.Sprintf("data/groups/%d", chatID)
		if err := os.MkdirAll(dir, 0755); err != nil {
			h.sendMessage(ctx, b, &tgbot.SendMessageParams{ChatID: chatID, MessageThreadID: msg.MessageThreadID, Text: "❌ Ошибка при создании директории."})
			return
		}
		filePath := filepath.Join(dir, "system_prompt.txt")
		if err := os.WriteFile(filePath, []byte(msg.Text), 0644); err != nil {
			h.sendMessage(ctx, b, &tgbot.SendMessageParams{ChatID: chatID, MessageThreadID: msg.MessageThreadID, Text: "❌ Ошибка при записи файла."})
			return
		}
		h.sessionStore.Clear(chatID, userID)
		h.sendMessage(ctx, b, &tgbot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: msg.MessageThreadID,
			Text:            "✅ System Prompt успешно сохранен.",
		})

	case telegram.StateWaitingForScheduleSearch:
		query := strings.TrimSpace(msg.Text)
		if len(query) < 2 {
			h.sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID:          chatID,
				MessageThreadID: msg.MessageThreadID,
				Text:            "⚠️ Поисковый запрос должен быть не менее 2 символов.",
			})
			return
		}

		if h.setkaBaseURL == "" {
			h.sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID:          chatID,
				MessageThreadID: msg.MessageThreadID,
				Text:            "❌ Интеграция с Setka не настроена (URL пуст).",
			})
			h.sessionStore.Clear(chatID, userID)
			return
		}

		results, err := h.searchSetkaGroups(ctx, query)
		if err != nil {
			slog.Error("Setka search failed", "error", err)
			h.sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID:          chatID,
				MessageThreadID: msg.MessageThreadID,
				Text:            "❌ Произошла ошибка при поиске в Setka: " + err.Error(),
			})
			return
		}

		if len(results) == 0 {
			h.sendMessage(ctx, b, &tgbot.SendMessageParams{
				ChatID:          chatID,
				MessageThreadID: msg.MessageThreadID,
				Text:            "🔍 Группы по запросу не найдены. Попробуйте еще раз или напишите /cancel.",
			})
			return
		}

		if len(results) > 8 {
			results = results[:8]
		}

		var keyboard [][]models.InlineKeyboardButton
		for _, r := range results {
			realIDVal := r.RealID
			if realIDVal == 0 {
				realIDVal = r.ID
			}
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{
					Text:         r.Name,
					CallbackData: fmt.Sprintf("settings:setka_sel:%d", realIDVal),
				},
			})
		}
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{
				Text:         "⬅️ Назад к меню",
				CallbackData: "settings:menu:setka",
			},
		})

		h.sendMessage(ctx, b, &tgbot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: msg.MessageThreadID,
			Text:            "🔍 <b>Результаты поиска в Setka:</b>\n\nВыберите вашу группу из списка ниже:",
			ParseMode:       models.ParseModeHTML,
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: keyboard,
			},
		})
	}
}

func (h *SettingsHandler) searchSetkaGroups(ctx context.Context, query string) ([]setkaSearchResult, error) {
	searchURL := fmt.Sprintf("%s/api/v1/search?q=%s&type=group", h.setkaBaseURL, url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var bff setkaBFFResponse
	if err := json.NewDecoder(resp.Body).Decode(&bff); err != nil {
		return nil, err
	}

	if !bff.Success {
		return nil, fmt.Errorf("Setka API error: %s", bff.Error)
	}

	return bff.Data, nil
}

func defaultFeatures() map[string]bool {
	return map[string]bool{
		"enable_schedule":            true,
		"enable_summary":             true,
		"enable_moderation":          true,
		"enable_captcha":             true,
		"enable_link_filter":         true,
		"enable_flood_control":        true,
		"enable_voice_transcription": true,
		"enable_photo_processing":    true,
		"photo_on_mention":          false,
	}
}

func (h *SettingsHandler) LoadFeatures(chatID int64) map[string]bool {
	filePath := fmt.Sprintf("data/groups/%d/features.json", chatID)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return defaultFeatures()
	}
	var features map[string]bool
	if err := json.Unmarshal(content, &features); err != nil {
		return defaultFeatures()
	}
	// fill defaults for missing features
	defaults := defaultFeatures()
	for k, v := range defaults {
		if _, exists := features[k]; !exists {
			features[k] = v
		}
	}
	return features
}

func (h *SettingsHandler) saveFeatures(chatID int64, features map[string]bool) error {
	dir := fmt.Sprintf("data/groups/%d", chatID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	bytes, err := json.Marshal(features)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "features.json"), bytes, 0644)
}
