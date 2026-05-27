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
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"omsu_bot/internal/agent"
	omsuapp "omsu_bot/internal/app"
	"omsu_bot/internal/api"
	"omsu_bot/internal/buffer"
	"omsu_bot/internal/classifier"
	"omsu_bot/internal/config"
	omsudb "omsu_bot/internal/db"
	"omsu_bot/internal/forwarder"
	"omsu_bot/internal/handler"
	"omsu_bot/internal/llm"
	"omsu_bot/internal/media"
	"omsu_bot/internal/messages"
	"omsu_bot/internal/permissions"
	"omsu_bot/internal/persona"
	"omsu_bot/internal/schedule"
	"omsu_bot/internal/skills"
	"omsu_bot/internal/telegram"
	"omsu_bot/internal/util"
)

type telegramPoster struct {
	b *tgbot.Bot
}

func (p *telegramPoster) PostToThread(ctx context.Context, chatID int64, threadID int, text string) error {
	_, err := p.b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          chatID,
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

const (
	processedMediaGroupsTTL = 10 * time.Minute
	mediaGroupMessagesTTL   = 30 * time.Minute
)

type App struct {
	BotStartTime         time.Time
	BotUsername          string
	ProviderCount        int
	GlobalLLM            llm.LLMClient
	ProcessedMediaGroups sync.Map
	MediaGroupMessages   sync.Map
}

var app = &App{
	BotStartTime: time.Now(),
}

func initLLM(cfg *config.Config, db *omsudb.DB, personaStore *persona.Store, prompts *llm.PromptRegistry) (agentChain, simpleChain, visionChain, audioChain, diagChain *llm.Chain, client *llm.Client) {
	var agentProviders, simpleProviders, visionProviders, audioProviders []*llm.Provider
	for _, pcfg := range cfg.LLM.Providers {
		baseURL := pcfg.BaseURL
		if baseURL == "" {
			switch pcfg.Type {
			case "gemini":
				baseURL = "https://generativelanguage.googleapis.com"
			case "deepseek":
				baseURL = "https://api.deepseek.com"
			case "groq":
				baseURL = "https://api.groq.com/openai"
			}
		}
		p := &llm.Provider{
			Name:           pcfg.Name,
			Type:           pcfg.Type,
			BaseURL:        baseURL,
			APIKey:         pcfg.APIKey,
			Model:          pcfg.Model,
			FallbackModels: pcfg.FallbackModels,
			RPMLimit:       pcfg.RPMLimit,
			TPMLimit:       pcfg.TPMLimit,
			RPDLimit:       pcfg.RPDLimit,
			TPDLimit:       pcfg.TPDLimit,
			Priority:       pcfg.Priority,
		}

		if pcfg.Multimodal {
			p.Capabilities = append(p.Capabilities, llm.CapabilityMultimodal)
		}

		switch pcfg.Chain {
		case "agent":
			agentProviders = append(agentProviders, p)
		case "simple":
			simpleProviders = append(simpleProviders, p)
		case "vision":
			p.Capabilities = append(p.Capabilities, llm.CapabilityMultimodal)
			visionProviders = append(visionProviders, p)
		case "audio":
			p.Capabilities = append(p.Capabilities, llm.CapabilityMultimodal)
			audioProviders = append(audioProviders, p)
		default:
			simpleProviders = append(simpleProviders, p)
		}
	}

	sort.Slice(agentProviders, func(i, j int) bool { return agentProviders[i].Priority < agentProviders[j].Priority })
	sort.Slice(simpleProviders, func(i, j int) bool { return simpleProviders[i].Priority < simpleProviders[j].Priority })
	sort.Slice(visionProviders, func(i, j int) bool { return visionProviders[i].Priority < visionProviders[j].Priority })
	sort.Slice(audioProviders, func(i, j int) bool { return audioProviders[i].Priority < audioProviders[j].Priority })

	agentChain = llm.NewChain(agentProviders)
	simpleChain = llm.NewChain(simpleProviders)
	visionChain = llm.NewChain(visionProviders)
	audioChain = llm.NewChain(audioProviders)

	var allProviders []*llm.Provider
	allProviders = append(allProviders, agentProviders...)
	allProviders = append(allProviders, simpleProviders...)
	allProviders = append(allProviders, visionProviders...)
	allProviders = append(allProviders, audioProviders...)
	diagChain = llm.NewChain(allProviders)

	tracker := llm.NewTracker(db.DB, int64(cfg.LLM.DailyTokenLimit), 0.8)
	client = llm.NewClient(agentChain, simpleChain, visionChain, audioChain, tracker, personaStore, prompts, cfg.LLM.RequestTimeoutSec, cfg.LLM.SkipFallbackModel)
	app.ProviderCount = len(allProviders)
	return
}

func initPersona(db *omsudb.DB, _ *llm.PromptRegistry) *persona.Store {
	personaStore := persona.NewStore(db.DB)
	if err := personaStore.Load(context.Background(), "prompts/persona.md"); err != nil {
		slog.Error("failed to load persona", "error", err)
		os.Exit(1)
	}
	return personaStore
}

func startSighupHandler(sighupCtx context.Context, prompts *llm.PromptRegistry) {
	go func() {
		<-sighupCtx.Done()
		slog.Info("SIGHUP received, reloading prompts and protocols")
		if err := prompts.Reload(); err != nil {
			slog.Error("failed to reload prompts", "error", err)
		} else {
			slog.Info("prompts reloaded successfully")
		}
		agent.ReloadProtocols()
		slog.Info("protocols reloaded")
	}()
}

func main() {
	configPath := "config.yaml"
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		configPath = p
	}

	cfg := config.Load(configPath)
	setupLogger(cfg)

	// Set timezone for all time.Now() calls
	if cfg.Timezone != "" {
		os.Setenv("TZ", cfg.Timezone)
		if _, err := time.LoadLocation("Local"); err != nil {
			slog.Warn("invalid timezone", "tz", cfg.Timezone, "error", err)
		} else {
			slog.Info("timezone set", "tz", cfg.Timezone)
		}
	}

	database := omsuapp.InitDB(cfg.DB.Path)
	defer database.Close()

	prompts, err := llm.NewPromptRegistry("prompts")
	if err != nil {
		slog.Error("failed to load prompts", "error", err)
		os.Exit(1)
	}

	personaStore := initPersona(database, prompts)

	_ = handler.NewCommandRegistry() // kept for backward compat, commands migrated to registry

	botMessages := messages.Load("messages.yaml")

	_, _, _, _, llmChain, llmClient := initLLM(cfg, database, personaStore, prompts)
	app.GlobalLLM = llmClient

	tgBot, err := tgbot.New(cfg.Telegram.Token)
	if err != nil {
		slog.Error("failed to create Telegram bot, continuing without bot", "error", err)
	}

	if tgBot != nil {
		me, err := tgBot.GetMe(context.Background())
		if err == nil {
			app.BotUsername = me.Username
		}

		// Register commands so they appear in Telegram's command menu
		tgBot.SetMyCommands(context.Background(), &tgbot.SetMyCommandsParams{
			Commands: []models.BotCommand{
				{Command: "start", Description: "приветствие"},
				{Command: "help", Description: "справка и список команд"},
				{Command: "id", Description: "ID текущего топика"},
				{Command: "topics", Description: "список топиков"},
				{Command: "init", Description: "инициализировать группу (админ)"},
				{Command: "tag", Description: "поиск сообщений по хэштегу"},
				{Command: "resend", Description: "переслать сообщение в топик"},
				{Command: "register", Description: "зарегистрировать топик (админ)"},
				{Command: "summary", Description: "саммари текущего топика"},
				{Command: "settings", Description: "настройки группы (админ)"},
				{Command: "status", Description: "состояние бота"},
			},
			LanguageCode: "ru",
		})
	}

	slog.Info("GroupBot started",
		"group_id", cfg.Telegram.GroupID,
		"persona", personaStore.Get().Name,
		"providers", app.ProviderCount,
		"bot_username", app.BotUsername,
	)

	var poster *telegramPoster
	if tgBot != nil {
		poster = &telegramPoster{b: tgBot}
	}
	diffEngine := schedule.NewDiffEngine(database.DB, poster, llmClient, schedule.NewAnnouncer(llmClient, prompts))
	webhookHandler := handler.NewWebhookHandler(diffEngine, database.DB, cfg.Webhook.ScheduleSecret)

	var botSender api.BotSender
	if poster != nil {
		botSender = poster
	}

	authMw := api.NewAuthMiddleware(cfg.API.AdminSecret, cfg.API.JWTSecret, database.DB)
	apiServer := api.NewServer(
		database.DB,
		personaStore,
		prompts,
		authMw,
		&api.ServerConfig{
			SwaggerEnabled:     cfg.SwaggerEnabled,
			AppEnv:             cfg.AppEnv,
			CORSOrigin:         cfg.API.CORSOrigin,
			SkipFallbackModel:  cfg.LLM.SkipFallbackModel,
			TelegramGroupID:    cfg.Telegram.GroupID,
			RateLimitGeneral:   cfg.RateLimit.APIGeneral,
			RateLimitSearch:    cfg.RateLimit.APISearch,
			RateLimitWindowSec: cfg.RateLimit.APIWindow,
			SetkaBaseURL:       cfg.Setka.BaseURL,
			SetkaAdminKey:      cfg.Setka.AdminKey,
			SetkaPublicURL:     cfg.Setka.PublicURL,
			WebhookSecret:      cfg.Webhook.ScheduleSecret,
			ListenAddr:         cfg.API.Listen,
		},
		llmChain,
		llmClient,
		botSender,
	)
	// Restore persisted runtime config (skip_fallback_model, global_voice, global_photo)
	// overriding the defaults set from environment variables.
	apiServer.LoadConfigFromDB(context.Background())

	apiServer.App.Static("/admin", "./admin/dist", fiber.Static{Index: "index.html"})
	apiServer.App.Get("/admin/*", func(c *fiber.Ctx) error {
		return c.SendFile("./admin/dist/index.html")
	})
	apiServer.App.Post("/webhook/schedule", limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			slog.Warn("webhook rate limit exceeded", "ip", c.IP())
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "rate limit exceeded"})
		},
	}), webhookHandler.Handle)

	sighupCtx, sighupCancel := signal.NotifyContext(context.Background(), syscall.SIGHUP)
	defer sighupCancel()
	startSighupHandler(sighupCtx, prompts)

	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for range t.C {
			cutoff := time.Now().Add(-processedMediaGroupsTTL)
			app.ProcessedMediaGroups.Range(func(key, value interface{}) bool {
				if value.(time.Time).Before(cutoff) {
					app.ProcessedMediaGroups.Delete(key)
				}
				return true
			})
			mediaCutoff := time.Now().Add(-mediaGroupMessagesTTL)
			app.MediaGroupMessages.Range(func(key, value interface{}) bool {
				if entry, ok := value.(*agent.MediaGroupEntry); ok && entry.CreatedAt.Before(mediaCutoff) {
					app.MediaGroupMessages.Delete(key)
				}
				return true
			})
		}
	}()

	var summaryBuf *buffer.SummaryBuffer

	if tgBot != nil {
		classif := classifier.New(llmClient, prompts, &dbTopicsProvider{db: database.DB})
		fwd := forwarder.New(tgBot, personaStore)
		summaryBuf = buffer.NewSummaryBuffer(database.DB, 200)
		summaryBuf.Start()
		usernameCache := telegram.NewUsernameCache()
		h := handler.NewHandler(classif, fwd, tgBot, cfg.Telegram.Token, database, summaryBuf, usernameCache)

		sessionStore := telegram.NewSessionStore()
		sessionStore.StartCleanup(context.Background())
		adminCache := telegram.NewAdminCache(tgBot)
		adminCache.StartEviction(context.Background())
		settingsHandler := handler.NewSettingsHandler(
			database,
			sessionStore,
			adminCache,
			cfg.Setka.BaseURL,
			cfg.Setka.AdminKey,
			cfg.Webhook.ScheduleSecret,
			cfg.Setka.PublicURL,
			apiServer.GlobalVoiceTranscription.Load,
			apiServer.GlobalPhotoProcessing.Load,
		)

		permService := permissions.NewService(database.DB)
		toolExecutor := agent.NewToolExecutor(&agent.ToolDeps{
			DB:                 database.DB,
			Bot:                tgBot,
			Buffer:             summaryBuf,
			UsernameCache:      usernameCache,
			SetkaBaseURL:       cfg.Setka.BaseURL,
			SetkaPublicURL:     cfg.Setka.PublicURL,
			AdminChecker:       adminCache,
			MediaGroupMessages: &app.MediaGroupMessages,
			Classifier:         classif,
		})
		skillRegistry, err := skills.Load("skills")
		if err != nil {
			slog.Warn("failed to load skills registry, using built-in tools", "error", err)
		}
		orchestrator := agent.NewAgentOrchestrator(llmClient, toolExecutor, adminCache, permService, skillRegistry)
		mentionHandler := handler.NewMentionHandler(orchestrator, database.DB, app.BotUsername)
		antispam := handler.NewAntispam(settingsHandler.LoadFeatures)
		mediaProcessor := media.NewMediaProcessor(tgBot, cfg.Telegram.Token, llmClient, prompts)

		botDisplayName := personaStore.Get().Name
		helpHeader := botMessages.Format(botMessages.HelpHeader, map[string]string{"name": botDisplayName})
		helpText := helpHeader + "\n\n" + botMessages.HelpCommands + "\n\n" + botMessages.Format(botMessages.HelpDetail, map[string]string{"username": app.BotUsername})

		// Fallback text for /start when LLM is unavailable — loaded from prompts/start_fallback.txt.
		startFallback := strings.ReplaceAll(prompts.Get("start_fallback"), "{name}", botDisplayName)

		tgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/start", tgbot.MatchTypeExact, func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			if database.IsGroupActive(ctx, update.Message.Chat.ID) {
				if app.GlobalLLM != nil {
					greetingPrompt := strings.ReplaceAll(prompts.Get("greeting"), "{name}", botDisplayName)
				resp, err := app.GlobalLLM.Call(ctx, "diagnostic", "", greetingPrompt, false)
				if err == nil {
					resp.Content = util.StripMarkdown(resp.Content)
					b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: update.Message.Chat.ID, MessageThreadID: update.Message.MessageThreadID, Text: resp.Content, ParseMode: models.ParseModeHTML})
						return
					}
				}
				b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: update.Message.Chat.ID, MessageThreadID: update.Message.MessageThreadID, Text: startFallback, ParseMode: models.ParseModeHTML})
			} else {
				b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: helpText, ParseMode: models.ParseModeHTML})
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

		// Init command — dedicated handler (works regardless of IsGroupActive)
		tgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/init", tgbot.MatchTypePrefix, func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			if database != nil && settingsHandler != nil {
			handleSlashCommand(ctx, b, update, &commandDeps{
				db: database, sqlDB: database.DB, mh: mentionHandler, settingsH: settingsHandler,
				helpText: helpText, botMsgs: botMessages, promptReg: prompts,
			})
			}
		})

		// Captcha callbacks — use RegisterHandler with HandlerTypeCallbackQueryData
		tgBot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, "captcha:", tgbot.MatchTypePrefix,
			func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
				antispam.HandleCallbackQuery(ctx, b, update)
			})
		// Settings callbacks — use RegisterHandler with HandlerTypeCallbackQueryData
		tgBot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, "settings:", tgbot.MatchTypePrefix,
			func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
				settingsHandler.HandleCallbackQuery(ctx, b, update)
			})

		// New Chat Members captcha prompt
		tgBot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
			return update.Message != nil && len(update.Message.NewChatMembers) > 0
		}, func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			checkCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			active := database.IsGroupActive(checkCtx, update.Message.Chat.ID)
			cancel()
			if !active {
				return
			}
			antispam.HandleNewChatMembers(ctx, b, update.Message.Chat.ID, update.Message.NewChatMembers)
		})

		// Settings command (dedicated handler: Telegram may not send BotCommand entity in groups)
		for _, cmd := range []string{"/settings", "/настройки"} {
			cmd := cmd
			tgBot.RegisterHandler(tgbot.HandlerTypeMessageText, cmd, tgbot.MatchTypeExact, func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
				if settingsHandler != nil {
					settingsHandler.HandleSettingsCommand(ctx, b, update)
				}
			})
		}

		// Auto-register topics created via Telegram
		tgBot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
			return update.Message != nil && (update.Message.ForumTopicCreated != nil || update.Message.ForumTopicEdited != nil)
		}, func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			msg := update.Message
			if msg.ForumTopicCreated != nil {
				slug := util.MakeSlug(msg.ForumTopicCreated.Name)
				_, err := database.DB.ExecContext(ctx,
					`INSERT OR IGNORE INTO topics (group_id, tg_thread_id, name, slug, aliases, description, hashtags, is_active, created_at)
					 VALUES (?, ?, ?, ?, '[]', '', '[]', 1, CURRENT_TIMESTAMP)`,
					msg.Chat.ID, msg.MessageThreadID, msg.ForumTopicCreated.Name, slug)
				if err != nil {
					slog.Error("failed to auto-register new topic", "error", err, "chat_id", msg.Chat.ID, "name", msg.ForumTopicCreated.Name)
				} else {
					slog.Info("auto-registered new topic", "chat_id", msg.Chat.ID, "name", msg.ForumTopicCreated.Name)
				}
			}

			if msg.ForumTopicEdited != nil && msg.ForumTopicEdited.Name != "" {
				slug := util.MakeSlug(msg.ForumTopicEdited.Name)
				_, err := database.DB.ExecContext(ctx,
					`UPDATE topics SET name = ?, slug = ? WHERE group_id = ? AND tg_thread_id = ?`,
					msg.ForumTopicEdited.Name, slug, msg.Chat.ID, msg.MessageThreadID)
				if err != nil {
					slog.Error("failed to auto-update topic name", "error", err, "chat_id", msg.Chat.ID)
				} else {
					slog.Info("auto-updated topic name", "chat_id", msg.Chat.ID, "new_name", msg.ForumTopicEdited.Name)
				}
			}
		})

		// Rate-limit middleware for Telegram handlers (P1#6)
		tgRateLimit := handler.NewMiddleware(cfg.RateLimit.GlobalPerUserPerMin)

		// Active groups messaging
		tgBot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
			return update.Message != nil && update.Message.Text != "/start" && update.Message.Text != "/help" && update.Message.Text != "/settings" && update.Message.Text != "/настройки" && !strings.HasPrefix(update.Message.Text, "/init")
		}, func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			checkCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			active := database.IsGroupActive(checkCtx, update.Message.Chat.ID)
			cancel()
			if !active {
				return
			}
			if antispam.CheckFloodAndLinks(ctx, b, update.Message) {
				return
			}

			msg := update.Message
			if msg.From == nil {
				return
			}
			state := sessionStore.Get(msg.Chat.ID, msg.From.ID)
			if state != telegram.StateNone {
				settingsHandler.HandleAdminInput(ctx, b, update, state)
				return
			}

			if strings.Contains(msg.Text, "ПЕРЕКЛИЧКА") {
				handleRollCall(ctx, b, msg, usernameCache)
				return
			}

			features := settingsHandler.LoadFeatures(msg.Chat.ID)
			p := persona.GetGroupPersona(msg.Chat.ID, personaStore.Get())

			if msg.Voice != nil && apiServer.GlobalVoiceTranscription.Load() && features["enable_voice_transcription"] {
				txt, err := mediaProcessor.ProcessVoice(ctx, msg.Voice.FileID)
				if err != nil {
					slog.Error("failed to process voice", "error", err)
				} else if txt != "" {
					msg.Text = txt
				}
			}

			originalCaption := msg.Caption

			// Track media group messages regardless of photo processing — needed for album forwarding
			if msg.MediaGroupID != "" {
				now := time.Now()
				entry := &agent.MediaGroupEntry{CreatedAt: now}
				val, loaded := app.MediaGroupMessages.LoadOrStore(msg.MediaGroupID, entry)
				entry = val.(*agent.MediaGroupEntry)
				fileID := ""
				mediaType := ""
				switch {
				case len(msg.Photo) > 0:
					fileID = msg.Photo[len(msg.Photo)-1].FileID
					mediaType = "photo"
				case msg.Document != nil:
					fileID = msg.Document.FileID
					mediaType = "document"
				case msg.Video != nil:
					fileID = msg.Video.FileID
					mediaType = "video"
				case msg.Audio != nil:
					fileID = msg.Audio.FileID
					mediaType = "audio"
				}
				entry.Mu.Lock()
				entry.Items = append(entry.Items, agent.MediaGroupItem{
					MessageID: msg.ID,
					FileID:    fileID,
					Caption:   msg.Caption,
					MediaType: mediaType,
				})
				itemsCount := len(entry.Items)
				entry.Mu.Unlock()

				if !loaded {
					entry.CreatedAt = now
				}

				// Persist to DB so album forwarding survives restarts
				database.DB.ExecContext(ctx,
					`INSERT OR IGNORE INTO media_group_items (media_group_id, message_id, chat_id, file_id, caption, media_type, created_at)
				 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
					msg.MediaGroupID, msg.ID, msg.Chat.ID, fileID, msg.Caption, mediaType,
				)
				slog.Debug("media group tracked", "group_id", msg.MediaGroupID, "msg_id", msg.ID, "items_count", itemsCount, "media_type", mediaType)
			}

			if len(msg.Photo) > 0 && apiServer.GlobalPhotoProcessing.Load() && features["enable_photo_processing"] {
				shouldOCR := true

				// Skip duplicate OCR in already-processed media groups
				if msg.MediaGroupID != "" {
					if stored, seen := app.ProcessedMediaGroups.LoadOrStore(msg.MediaGroupID, time.Now()); seen {
						if time.Since(stored.(time.Time)) < 10*time.Minute {
							shouldOCR = false
						} else {
							app.ProcessedMediaGroups.Store(msg.MediaGroupID, time.Now())
						}
					}
				}

				if shouldOCR {
					slog.Debug("processing photo OCR", "file_id", msg.Photo[len(msg.Photo)-1].FileID)
					ocrText, err := mediaProcessor.ProcessPhoto(ctx, msg.Photo[len(msg.Photo)-1].FileID)
					if err != nil {
						slog.Error("failed to process photo", "error", err)
					} else if ocrText != "" {
						if originalCaption != "" {
							msg.Caption = originalCaption + "\n---\n" + ocrText
						} else {
							msg.Caption = ocrText
						}
					}
				}
			}

			text := msg.Text
			if text == "" {
				text = msg.Caption
			}

			// Detect mention/alias from ORIGINAL caption+text only, not from OCR-generated text.
			// OCR adds noise and false triggers (e.g. "сессия" in OCR → alias match).
			mentionCheckText := msg.Text
			if mentionCheckText == "" {
				mentionCheckText = originalCaption
			}
			if mentionCheckText == "" {
				mentionCheckText = text
			}

			isMentionOrAlias := false
			if isBotMention(msg) {
				isMentionOrAlias = true
			} else if mentionCheckText != "" {
				lowerText := strings.ToLower(mentionCheckText)
				if p.Name != "" && strings.Contains(lowerText, strings.ToLower(p.Name)) {
					isMentionOrAlias = true
				} else {
					for _, alias := range p.Aliases {
						if strings.Contains(lowerText, strings.ToLower(alias)) {
							isMentionOrAlias = true
							break
						}
					}
				}
			}

			if isBotCommand(msg) {
				handleSlashCommand(ctx, b, update, &commandDeps{
				db: database, sqlDB: database.DB, mh: mentionHandler, settingsH: settingsHandler,
				helpText: helpText, botMsgs: botMessages, promptReg: prompts,
			})
				return
			}

			if !tgRateLimit.Allow(msg.From.ID) {
				b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   "⏳ Слишком много запросов. Подожди минуту.",
				})
				return
			}

			if isMentionOrAlias {
				mentionHandler.Handle(ctx, b, update)
			} else {
				h.HandleMessage(ctx, b, update)
			}
		})

		// Private / inactive groups handler
		tgBot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
			if update.Message == nil {
				return false
			}
			text := update.Message.Text
			if text == "/start" || text == "/help" || strings.HasPrefix(text, "/init") {
				return false
			}
			checkCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			active := database.IsGroupActive(checkCtx, update.Message.Chat.ID)
			cancel()
			return !active
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
		if _, err := telegram.RegisterWebhooksWithSetka(
			context.Background(), database.DB,
			cfg.Setka.BaseURL, cfg.Setka.AdminKey,
			cfg.Webhook.ScheduleSecret, cfg.Setka.PublicURL,
		); err != nil {
			slog.Warn("failed to register webhooks with setka", "error", err)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// P1-5: Periodic webhook re-registration every 10 minutes
	if cfg.Setka.BaseURL != "" && cfg.Setka.AdminKey != "" {
		go func() {
			ticker := time.NewTicker(10 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if _, err := telegram.RegisterWebhooksWithSetka(
						context.Background(), database.DB,
						cfg.Setka.BaseURL, cfg.Setka.AdminKey,
						cfg.Webhook.ScheduleSecret, cfg.Setka.PublicURL,
					); err != nil {
						slog.Warn("periodic webhook re-registration failed", "error", err)
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	if tgBot != nil {
		tgBot.Start(ctx)
	} else {
		<-ctx.Done()
	}

	if summaryBuf != nil {
		summaryBuf.Close()
	}
	apiServer.App.Shutdown()
}




