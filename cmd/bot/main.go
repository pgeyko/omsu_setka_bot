package main

// @title GroupBot API
// @version 1.0
// @description Telegram bot for student group — Admin REST API
// @host localhost:8081
// @BasePath /api
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer <JWT>" to authenticate

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/gofiber/fiber/v2"

	"omsu_bot/internal/agent"
	"omsu_bot/internal/api"
	"omsu_bot/internal/buffer"
	"omsu_bot/internal/classifier"
	"omsu_bot/internal/config"
	"omsu_bot/internal/db"
	"omsu_bot/internal/forwarder"
	handlers "omsu_bot/internal/handler"
	"omsu_bot/internal/llm"
	"omsu_bot/internal/media"
	"omsu_bot/internal/persona"
	"omsu_bot/internal/schedule"
	"omsu_bot/internal/telegram"
)

type telegramPoster struct {
	b      *tgbot.Bot
	chatID int64
}

func (p *telegramPoster) PostToThread(ctx context.Context, threadID int, text string) error {
	_, err := p.b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          p.chatID,
		MessageThreadID: threadID,
		Text:            text,
	})
	return err
}

func (p *telegramPoster) Send(ctx context.Context, chatID int64, text string) error {
	_, err := p.b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	return err
}

func main() {
	configPath := "config.yaml"
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		configPath = p
	}

	cfg := config.Load(configPath)
	setupLogger(cfg)

	database, err := db.New(cfg.DB.Path)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		slog.Error("failed to migrate database", "error", err)
		os.Exit(1)
	}

	personaStore := persona.NewStore(database.DB)
	if err := personaStore.Load(context.Background(), "persona.md"); err != nil {
		slog.Error("failed to load persona", "error", err)
		os.Exit(1)
	}

	prompts, err := llm.NewPromptRegistry("prompts")
	if err != nil {
		slog.Error("failed to load prompts", "error", err)
		os.Exit(1)
	}

	cmdReg, err := handlers.LoadCommands("commands.json")
	if err != nil {
		slog.Warn("failed to load commands.json, using defaults", "error", err)
		cmdReg = &handlers.CommandRegistry{}
	}

	var providers []*llm.Provider
	for _, pcfg := range cfg.LLM.Providers {
		capabilities := []llm.Capability{}
		if pcfg.Multimodal {
			capabilities = append(capabilities, llm.CapabilityMultimodal)
		}
		baseURL := pcfg.BaseURL
		if baseURL == "" {
			switch pcfg.Type {
			case "gemini":
				baseURL = "https://generativelanguage.googleapis.com"
			case "deepseek":
				baseURL = "https://api.deepseek.com"
			}
		}
		fallbackModels := pcfg.FallbackModels
		if len(fallbackModels) == 0 {
			fallbackModels = []string{"gemini-2.5-flash-lite"}
		}
		providers = append(providers, &llm.Provider{
			Name:           pcfg.Name,
			Type:           pcfg.Type,
			BaseURL:        baseURL,
			APIKey:         pcfg.APIKey,
			Model:          pcfg.Model,
			FallbackModels: fallbackModels,
			Capabilities:   capabilities,
		})
	}
	llmChain := llm.NewChain(providers)

	tracker := llm.NewTracker(database.DB, int64(cfg.LLM.DailyTokenLimit), 0.8)
	llmClient := llm.NewClient(llmChain, tracker, personaStore, prompts, cfg.LLM.RequestTimeoutSec, cfg.LLM.SkipFallbackModel)
	globalLLM = llmClient

	tgBot, err := tgbot.New(cfg.Telegram.Token)
	if err != nil {
		slog.Error("failed to create Telegram bot, continuing without bot", "error", err)
	}

	providerCount = len(providers)
	if tgBot != nil {
		me, err := tgBot.GetMe(context.Background())
		if err == nil {
			botUsername = me.Username
		}
	}

	slog.Info("GroupBot started",
		"group_id", cfg.Telegram.GroupID,
		"persona", personaStore.Get().Name,
		"providers", providerCount,
		"bot_username", botUsername,
	)

	var poster *telegramPoster
	if tgBot != nil {
		poster = &telegramPoster{b: tgBot, chatID: cfg.Telegram.GroupID}
	}
	diffEngine := schedule.NewDiffEngine(database.DB, poster, llmClient)
	webhookHandler := handlers.NewWebhookHandler(diffEngine, cfg.Webhook.ScheduleSecret, cfg.Webhook.AnnounceThreadID)

	var botSender api.BotSender
	if poster != nil {
		botSender = poster
	}

	authMw := api.NewAuthMiddleware(cfg.API.AdminSecret, cfg.API.JWTSecret)
	apiServer := api.NewServer(
		database.DB,
		personaStore,
		prompts,
		authMw,
		cfg.SwaggerEnabled,
		cfg.AppEnv,
		cfg.API.CORSOrigin,
		llmChain,
		llmClient,
		botSender,
		cfg.Telegram.GroupID,
		cfg.LLM.SkipFallbackModel,
		cfg.RateLimit.APIGeneral,
		cfg.RateLimit.APISearch,
		cfg.RateLimit.APIWindow,
		cfg.Setka.BaseURL,
		cfg.Setka.AdminKey,
		cfg.Webhook.ScheduleSecret,
		cfg.API.Listen,
	)

	apiServer.App.Static("/admin", "./admin/dist", fiber.Static{Index: "index.html"})
	apiServer.App.Get("/admin/*", func(c *fiber.Ctx) error {
		return c.SendFile("./admin/dist/index.html")
	})
	apiServer.App.Post("/webhook/schedule", webhookHandler.Handle)

	go func() {
		sigHup := make(chan os.Signal, 1)
		signal.Notify(sigHup, syscall.SIGHUP)
		for range sigHup {
			slog.Info("SIGHUP received, reloading prompts")
			if err := prompts.Reload(); err != nil {
				slog.Error("failed to reload prompts", "error", err)
			} else {
				slog.Info("prompts reloaded successfully")
			}
		}
	}()

	if tgBot != nil {
		classif := classifier.New(llmClient, prompts, &dbTopicsProvider{db: database.DB})
		fwd := forwarder.New(tgBot, personaStore)
		summaryBuf := buffer.NewSummaryBuffer(database.DB, 200)
		usernameCache := telegram.NewUsernameCache()
		h := handlers.NewHandler(classif, fwd, database.DB, summaryBuf, usernameCache)

		sessionStore := telegram.NewSessionStore()
		adminCache := telegram.NewAdminCache(tgBot, cfg.Telegram.GroupID)
		settingsHandler := handlers.NewSettingsHandler(
			database.DB,
			sessionStore,
			adminCache,
			cfg.Setka.BaseURL,
			cfg.Setka.AdminKey,
			cfg.Webhook.ScheduleSecret,
			cfg.API.Listen,
		)

		toolExecutor := agent.NewToolExecutor(database.DB, tgBot, summaryBuf, usernameCache, cfg.Setka.BaseURL, cfg.Setka.PublicURL)
		orchestrator := agent.NewAgentOrchestrator(llmClient, toolExecutor)
		mentionHandler := handlers.NewMentionHandler(orchestrator, database.DB, botUsername, cmdReg)
		antispam := handlers.NewAntispam()
		mediaProcessor := media.NewMediaProcessor(tgBot, cfg.Telegram.Token, llmClient)

		helpText := fmt.Sprintf(`🤖 <b>Пятница</b> — ИИ-ассистент группы

/start — приветствие
/help — эта справка
/id — ID топика
/topics — список топиков
/resend — переслать в топик
/register — зарегистрировать топик (админ)
/summary — саммари
/settings — настройки группы (админ)
/status — состояние

Подробнее: @%s`, botUsername)

		startHardcoded := "👋 Привет! Я <b>Пятница</b> — ИИ-ассистент. Работаю только в групповом чате. Напиши /help чтобы узнать что я умею."

		tgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/start", tgbot.MatchTypeExact, func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			if database.IsGroupActive(ctx, update.Message.Chat.ID) {
				if globalLLM != nil {
					resp, err := globalLLM.Call(ctx, "diagnostic", "", "Поприветствуй нового пользователя в группе. Представься как Пятница. Кратко расскажи что умеешь: пересылать сообщения между топиками, показывать расписание, делать саммари. Важно: используй ТОЛЬКО HTML-теги (<b>текст</b>), НЕ используй markdown (**). Максимум 100-150 слов. Эмодзи 1-2.", false)
					if err == nil {
						b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: update.Message.Chat.ID, MessageThreadID: update.Message.MessageThreadID, Text: resp.Content, ParseMode: models.ParseModeHTML})
						return
					}
				}
				b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: update.Message.Chat.ID, MessageThreadID: update.Message.MessageThreadID, Text: startHardcoded, ParseMode: models.ParseModeHTML})
			} else {
				b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: startHardcoded, ParseMode: models.ParseModeHTML})
			}
		})

		tgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/help", tgbot.MatchTypeExact, func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				MessageThreadID: update.Message.MessageThreadID,
				Text:            helpText,
				ParseMode:       models.ParseModeHTML,
			})
		})

		// Captcha callbacks
		tgBot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
			return update.CallbackQuery != nil && strings.HasPrefix(update.CallbackQuery.Data, "captcha:")
		}, func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			antispam.HandleCallbackQuery(ctx, b, update)
		})

		// Settings callbacks
		tgBot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
			return update.CallbackQuery != nil && strings.HasPrefix(update.CallbackQuery.Data, "settings:")
		}, func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			settingsHandler.HandleCallbackQuery(ctx, b, update)
		})

		// New Chat Members captcha prompt
		tgBot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
			return update.Message != nil && len(update.Message.NewChatMembers) > 0 && database.IsGroupActive(context.Background(), update.Message.Chat.ID)
		}, func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			antispam.HandleNewChatMembers(ctx, b, update.Message.Chat.ID, update.Message.NewChatMembers)
		})

		// Active groups messaging
		tgBot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
			return update.Message != nil && database.IsGroupActive(context.Background(), update.Message.Chat.ID) && update.Message.Text != "/start" && update.Message.Text != "/help"
		}, func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			if antispam.CheckFloodAndLinks(ctx, b, update.Message) {
				return
			}

			msg := update.Message
			state := sessionStore.Get(msg.Chat.ID, msg.From.ID)
			if state != telegram.StateNone {
				settingsHandler.HandleAdminInput(ctx, b, update, state)
				return
			}

			if msg.Voice != nil {
				txt, err := mediaProcessor.ProcessVoice(ctx, msg.Voice.FileID)
				if err != nil {
					slog.Error("failed to process voice", "error", err)
				} else if txt != "" {
					msg.Text = txt
				}
			}

			if len(msg.Photo) > 0 {
				ocrText, err := mediaProcessor.ProcessPhoto(ctx, msg.Photo[len(msg.Photo)-1].FileID)
				if err != nil {
					slog.Error("failed to process photo", "error", err)
				} else if ocrText != "" {
					if msg.Caption != "" {
						msg.Caption = msg.Caption + "\n" + ocrText
					} else {
						msg.Text = ocrText
					}
				}
			}

			text := msg.Text
			if text == "" {
				text = msg.Caption
			}

			if isBotCommand(msg) {
				handleSlashCommand(ctx, b, update, database.DB, msg.Chat.ID, mentionHandler, helpText, cmdReg, settingsHandler)
			} else if isBotMention(msg) {
				mentionHandler.Handle(ctx, b, update)
			} else if cmdReg != nil && text != "" && cmdReg.IsPersonaMention(text, personaStore.Get().Name) {
				mentionHandler.Handle(ctx, b, update)
			} else {
				h.HandleMessage(ctx, b, update)
			}
		})

		// Private / inactive groups handler
		tgBot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
			return update.Message != nil && !database.IsGroupActive(context.Background(), update.Message.Chat.ID) && update.Message.Text != "/start" && update.Message.Text != "/help"
		}, func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			// Private chat / inactive group — ignore all except /start and /help (handled above)
		})
	}

	go func() {
		slog.Info("starting HTTP server", "addr", cfg.API.Listen)
		if err := apiServer.App.Listen(cfg.API.Listen); err != nil {
			slog.Error("HTTP server failed", "error", err)
		}
	}()

	if cfg.Setka.BaseURL != "" && cfg.Setka.AdminKey != "" {
		registerWithSetka(context.Background(), cfg)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if tgBot != nil {
		tgBot.Start(ctx)
	} else {
		<-ctx.Done()
	}

	apiServer.App.Shutdown()
}

var (
	botStartTime  = time.Now()
	botUsername   string
	providerCount int
	globalLLM     *llm.Client
)

func handleSlashCommand(ctx context.Context, b *tgbot.Bot, update *models.Update, db *sql.DB, groupID int64, mh *handlers.MentionHandler, helpText string, cmd *handlers.CommandRegistry, settingsHandler *handlers.SettingsHandler) {
	msg := update.Message
	text := msg.Text

	parts := strings.SplitN(text, " ", 2)
	command := strings.TrimPrefix(parts[0], "/")
	command = strings.Split(command, "@")[0]
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	switch command {
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
			reply := "Укажи топик: /resend [название или slug]"
			if cmd != nil {
				reply = cmd.Response("resend_usage", nil)
			}
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: reply})
			return
		}
		var tgThreadID int
		err := db.QueryRowContext(ctx,
			`SELECT tg_thread_id FROM topics WHERE group_id = ? AND (slug = ? OR name = ?) AND is_active = 1 LIMIT 1`,
			msg.Chat.ID, args, args,
		).Scan(&tgThreadID)
		if err != nil {
			reply := fmt.Sprintf("Топик «%s» не найден. Напиши /topics", args)
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: reply})
			return
		}

		forwardMsgID := msg.ID
		if msg.ReplyToMessage != nil {
			forwardMsgID = msg.ReplyToMessage.ID
		} else {
			reply := "Ответь на сообщение, которое хочешь переслать."
			if cmd != nil {
				reply = cmd.Response("resend_no_reply", nil)
			}
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: reply})
			return
		}

		fromChatID := fmt.Sprintf("%d", msg.Chat.ID)
		_, err = b.CopyMessage(ctx, &tgbot.CopyMessageParams{
			ChatID: msg.Chat.ID, FromChatID: fromChatID, MessageID: forwardMsgID, MessageThreadID: tgThreadID,
		})
		if err != nil {
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: "Ошибка при пересылке."})
			return
		}

	case "summary", "саммари":
		update.Message.Text = "саммари"
		mh.Handle(ctx, b, update)

	case "start":
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
			Text:      "👋 Привет! Я <b>Пятница</b> — ваш ИИ-ассистент.\n\n" + helpText,
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
			statusPrompt := "Расскажи о себе и своём состоянии, используя эти данные:\n" + statsStr + "\nОтветь кратко, в своём стиле. Используй HTML-теги для форматирования. Эмодзи — 1-2 максимум."
			resp, err := globalLLM.Call(ctx, "diagnostic", "", statusPrompt, false)
			if err == nil {
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
			reply := "Укажи название: /register [название топика]"
			if cmd != nil {
				reply = cmd.Response("register_usage", nil)
			}
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: reply})
			return
		}
		if msg.MessageThreadID == 0 {
			reply := "❌ Это общий чат. Напиши /register в нужном топике."
			if cmd != nil {
				reply = cmd.Response("register_general", nil)
			}
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, Text: reply})
			return
		}
		var existing int
		db.QueryRowContext(ctx, `SELECT COUNT(*) FROM topics WHERE group_id = ? AND tg_thread_id = ?`, msg.Chat.ID, msg.MessageThreadID).Scan(&existing)
		if existing > 0 {
			reply := "⚠️ Этот топик уже зарегистрирован."
			if cmd != nil {
				reply = cmd.Response("register_exists", nil)
			}
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: reply})
			return
		}
		slug := makeSlug(args)
		_, err := db.ExecContext(ctx,
			`INSERT INTO topics (group_id, tg_thread_id, name, slug, aliases, description, hashtags, is_active, created_at)
			 VALUES (?, ?, ?, ?, '[]', '', '[]', 1, CURRENT_TIMESTAMP)`,
			msg.Chat.ID, msg.MessageThreadID, args, slug)
		if err != nil {
			b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
				Text: "❌ Ошибка при регистрации топика."})
			return
		}
		reply := fmt.Sprintf("✅ Топик «%s» зарегистрирован (ID: %d)", args, msg.MessageThreadID)
		if cmd != nil {
			reply = cmd.Response("register_ok", map[string]string{"name": args, "id": fmt.Sprintf("%d", msg.MessageThreadID)})
		}
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
			list = "Нет зарегистрированных топиков."
		}
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
			Text: "📋 <b>Топики:</b>\n" + list, ParseMode: models.ParseModeHTML,
		})

	case "id", "topic_id":
		if msg.MessageThreadID != 0 {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID,
				Text: fmt.Sprintf("🆔 ID этого топика: %d", msg.MessageThreadID),
			})
		} else {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: msg.Chat.ID, Text: "📋 Это общий чат, у него нет ID топика.",
			})
		}

	default:
		reply := "Неизвестная команда. Напиши /help чтобы увидеть список."
		b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: msg.Chat.ID, MessageThreadID: msg.MessageThreadID, Text: reply,
		})
	}
}

func isBotCommand(msg *models.Message) bool {
	if msg.Entities == nil {
		return false
	}
	for _, e := range msg.Entities {
		if e.Type == models.MessageEntityTypeBotCommand {
			return true
		}
	}
	return false
}

func setupLogger(cfg *config.Config) {
	level := slog.LevelInfo
	if cfg.Logging.Level == "debug" {
		level = slog.LevelDebug
	} else if cfg.Logging.Level == "warn" {
		level = slog.LevelWarn
	} else if cfg.Logging.Level == "error" {
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if cfg.Logging.Format == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(h))
}

func registerWithSetka(ctx context.Context, cfg *config.Config) {
	body := map[string]interface{}{
		"url":       fmt.Sprintf("http://localhost%s/webhook/schedule", cfg.API.Listen),
		"secret":    cfg.Webhook.ScheduleSecret,
		"group_ids": []int{cfg.Telegram.OmsuGroupID},
		"enabled":   true,
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/v1/admin/webhooks", cfg.Setka.BaseURL),
		bytes.NewReader(data))
	if err != nil {
		slog.Error("failed to create setka registration request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Key", cfg.Setka.AdminKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("failed to register with omsu_setka", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		slog.Info("registered webhook with omsu_setka")
	} else {
		slog.Warn("omsu_setka registration returned non-2xx", "status", resp.StatusCode)
	}
}

func makeSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.NewReplacer(
		"а", "a", "б", "b", "в", "v", "г", "g", "д", "d", "е", "e", "ё", "e",
		"ж", "zh", "з", "z", "и", "i", "й", "y", "к", "k", "л", "l", "м", "m",
		"н", "n", "о", "o", "п", "p", "р", "r", "с", "s", "т", "t", "у", "u",
		"ф", "f", "х", "kh", "ц", "ts", "ч", "ch", "ш", "sh", "щ", "shch",
		"ы", "y", "э", "e", "ю", "yu", "я", "ya",
	).Replace(slug)
	slug = strings.ReplaceAll(slug, " ", "_")
	slug = strings.ReplaceAll(slug, ".", "")
	slug = strings.ReplaceAll(slug, "-", "_")
	slug = strings.ReplaceAll(slug, "'", "")
	slug = strings.ReplaceAll(slug, "`", "")
	slug = strings.ReplaceAll(slug, "\"", "")
	return slug
}

func isBotMention(msg *models.Message) bool {
	checkEntities := func(entities []models.MessageEntity, text string) bool {
		if len(entities) == 0 || text == "" {
			return false
		}
		u16 := utf16.Encode([]rune(text))
		for _, e := range entities {
			if e.Type == models.MessageEntityTypeMention {
				if e.Offset >= 0 && e.Offset+e.Length <= len(u16) {
					mention := string(utf16.Decode(u16[e.Offset : e.Offset+e.Length]))
					if strings.EqualFold(mention, "@"+botUsername) {
						return true
					}
				}
			}
		}
		return false
	}

	if checkEntities(msg.Entities, msg.Text) {
		return true
	}
	if checkEntities(msg.CaptionEntities, msg.Caption) {
		return true
	}
	return false
}

type dbTopicsProvider struct {
	db *sql.DB
}

func (p *dbTopicsProvider) GetTopics(ctx context.Context, chatID int64) ([]classifier.TopicInfo, error) {
	rows, err := p.db.QueryContext(ctx, "SELECT slug, name, description FROM topics WHERE group_id = ? AND is_active = 1", chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []classifier.TopicInfo
	for rows.Next() {
		var t classifier.TopicInfo
		if err := rows.Scan(&t.Slug, &t.Name, &t.Description); err != nil {
			return nil, err
		}
		topics = append(topics, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return topics, nil
}
