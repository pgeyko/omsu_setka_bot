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
	"strconv"
	"strings"
	"sync"
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
	omsudb "omsu_bot/internal/db"
	"omsu_bot/internal/forwarder"
	"omsu_bot/internal/util"
	handlers "omsu_bot/internal/handler"
	"omsu_bot/internal/llm"
	"omsu_bot/internal/media"
	"omsu_bot/internal/messages"
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

	// Set timezone for all time.Now() calls
	if cfg.Timezone != "" {
		os.Setenv("TZ", cfg.Timezone)
		if _, err := time.LoadLocation("Local"); err != nil {
			slog.Warn("invalid timezone", "tz", cfg.Timezone, "error", err)
		} else {
			slog.Info("timezone set", "tz", cfg.Timezone)
		}
	}

	database, err := omsudb.New(cfg.DB.Path)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		slog.Error("failed to migrate database", "error", err)
		os.Exit(1)
	}
	database.StartCleanup(context.Background())

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

	cmdReg := handlers.NewCommandRegistry()

	botMessages := messages.Load("messages.yaml")

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
			Name:     pcfg.Name,
			Type:     pcfg.Type,
			BaseURL:  baseURL,
			APIKey:   pcfg.APIKey,
			Model:    pcfg.Model,
			FallbackModels: pcfg.FallbackModels,
			RPMLimit: pcfg.RPMLimit,
			TPMLimit: pcfg.TPMLimit,
			RPDLimit: pcfg.RPDLimit,
			TPDLimit: pcfg.TPDLimit,
		}

		switch pcfg.Chain {
		case "agent":
			agentProviders = append(agentProviders, p)
		case "simple":
			simpleProviders = append(simpleProviders, p)
		case "vision":
			p.Capabilities = []llm.Capability{llm.CapabilityMultimodal}
			visionProviders = append(visionProviders, p)
		case "audio":
			p.Capabilities = []llm.Capability{llm.CapabilityMultimodal}
			audioProviders = append(audioProviders, p)
		default:
			simpleProviders = append(simpleProviders, p)
		}
	}

	agentChain := llm.NewChain(agentProviders)
	simpleChain := llm.NewChain(simpleProviders)
	visionChain := llm.NewChain(visionProviders)
	audioChain := llm.NewChain(audioProviders)

	// Combined chain for API diagnostics (shows all providers)
	var allProviders []*llm.Provider
	allProviders = append(allProviders, agentProviders...)
	allProviders = append(allProviders, simpleProviders...)
	allProviders = append(allProviders, visionProviders...)
	allProviders = append(allProviders, audioProviders...)
	llmChain := llm.NewChain(allProviders)

	tracker := llm.NewTracker(database.DB, int64(cfg.LLM.DailyTokenLimit), 0.8)
	llmClient := llm.NewClient(agentChain, simpleChain, visionChain, audioChain, tracker, personaStore, prompts, cfg.LLM.RequestTimeoutSec, cfg.LLM.SkipFallbackModel)
	globalLLM = llmClient

	tgBot, err := tgbot.New(cfg.Telegram.Token)
	if err != nil {
		slog.Error("failed to create Telegram bot, continuing without bot", "error", err)
	}

	providerCount = len(allProviders)
	if tgBot != nil {
		me, err := tgBot.GetMe(context.Background())
		if err == nil {
			botUsername = me.Username
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
		"providers", providerCount,
		"bot_username", botUsername,
	)

	var poster *telegramPoster
	if tgBot != nil {
		poster = &telegramPoster{b: tgBot, chatID: cfg.Telegram.GroupID}
	}
	diffEngine := schedule.NewDiffEngine(database.DB, poster, llmClient, schedule.NewAnnouncer(llmClient, prompts))
	webhookHandler := handlers.NewWebhookHandler(diffEngine, cfg.Webhook.ScheduleSecret, cfg.Webhook.AnnounceThreadID)

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
		cfg.Setka.PublicURL,
		cfg.Webhook.ScheduleSecret,
		cfg.API.Listen,
	)
	// Restore persisted runtime config (skip_fallback_model, global_voice, global_photo)
	// overriding the defaults set from environment variables.
	apiServer.LoadConfigFromDB(context.Background())

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
		h := handlers.NewHandler(classif, fwd, tgBot, database.DB, summaryBuf, usernameCache)

		sessionStore := telegram.NewSessionStore()
		adminCache := telegram.NewAdminCache(tgBot, cfg.Telegram.GroupID)
		settingsHandler := handlers.NewSettingsHandler(
			database.DB,
			sessionStore,
			adminCache,
			cfg.Setka.BaseURL,
			cfg.Setka.AdminKey,
			cfg.Webhook.ScheduleSecret,
			cfg.Setka.PublicURL,
			apiServer.GlobalVoiceTranscription.Load,
			apiServer.GlobalPhotoProcessing.Load,
		)

		toolExecutor := agent.NewToolExecutor(database.DB, tgBot, summaryBuf, usernameCache, cfg.Setka.BaseURL, cfg.Setka.PublicURL, adminCache, &mediaGroupMessages, classif)
		orchestrator := agent.NewAgentOrchestrator(llmClient, toolExecutor, adminCache)
		mentionHandler := handlers.NewMentionHandler(orchestrator, database.DB, botUsername)
		antispam := handlers.NewAntispam(settingsHandler.LoadFeatures)
		mediaProcessor := media.NewMediaProcessor(tgBot, cfg.Telegram.Token, llmClient, prompts)

		botDisplayName := personaStore.Get().Name
		helpHeader := botMessages.Format(botMessages.HelpHeader, map[string]string{"name": botDisplayName})
		helpText := helpHeader + "\n\n" + botMessages.HelpCommands + "\n\n" + botMessages.Format(botMessages.HelpDetail, map[string]string{"username": botUsername})

		// Fallback text for /start when LLM is unavailable — loaded from prompts/start_fallback.txt.
		startFallback := strings.ReplaceAll(prompts.Get("start_fallback"), "{name}", botDisplayName)

		tgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/start", tgbot.MatchTypeExact, func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			if database.IsGroupActive(ctx, update.Message.Chat.ID) {
				if globalLLM != nil {
					greetingPrompt := strings.ReplaceAll(prompts.Get("greeting"), "{name}", botDisplayName)
					resp, err := globalLLM.Call(ctx, "diagnostic", "", greetingPrompt, false)
					if err == nil {
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
				// Reuse handleSlashCommand for the actual logic
				handleSlashCommand(ctx, b, update, database.DB, update.Message.Chat.ID, mentionHandler, helpText, cmdReg, settingsHandler, botMessages, prompts)
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
			return update.Message != nil && len(update.Message.NewChatMembers) > 0 && database.IsGroupActive(context.Background(), update.Message.Chat.ID)
		}, func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
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

		// Active groups messaging
		tgBot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
			return update.Message != nil && database.IsGroupActive(context.Background(), update.Message.Chat.ID) && update.Message.Text != "/start" && update.Message.Text != "/help" && update.Message.Text != "/settings" && update.Message.Text != "/настройки"
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

			// Photo processing: only when bot is explicitly mentioned (reply, @bot, alias).
			// Auto-OCR on all photos is wasteful — ~60s per photo with fallback chain.
			if len(msg.Photo) > 0 && apiServer.GlobalPhotoProcessing.Load() && features["enable_photo_processing"] {
				// Track message IDs and file IDs for media groups (used by forward_message for albums)
				if msg.MediaGroupID != "" {
					existing, _ := mediaGroupMessages.Load(msg.MediaGroupID)
					var items []agent.MediaGroupItem
					if existing != nil {
						items = existing.([]agent.MediaGroupItem)
					}
					fileID := ""
					if len(msg.Photo) > 0 {
						fileID = msg.Photo[len(msg.Photo)-1].FileID
					}
					items = append(items, agent.MediaGroupItem{
						MessageID: msg.ID,
						FileID:    fileID,
						Caption:   msg.Caption,
					})
					mediaGroupMessages.Store(msg.MediaGroupID, items)
				}

				shouldOCR := isBotMention(msg) ||
					(msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.Username == botUsername)

				if !shouldOCR && originalCaption != "" {
					lowerCaption := strings.ToLower(originalCaption)
					if p.Name != "" && strings.Contains(lowerCaption, strings.ToLower(p.Name)) {
						shouldOCR = true
					} else {
						for _, alias := range p.Aliases {
							if strings.Contains(lowerCaption, strings.ToLower(alias)) {
								shouldOCR = true
								break
							}
						}
					}
				}

				// Skip photos in already-processed media groups to avoid duplicate OCR
				if msg.MediaGroupID != "" {
					if _, seen := processedMediaGroups.LoadOrStore(msg.MediaGroupID, true); seen {
						shouldOCR = false
					}
				}

				if shouldOCR {
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
				handleSlashCommand(ctx, b, update, database.DB, msg.Chat.ID, mentionHandler, helpText, cmdReg, settingsHandler, botMessages, prompts)
			} else if isMentionOrAlias {
				mentionHandler.Handle(ctx, b, update)
			} else {
				h.HandleMessage(ctx, b, update)
			}
		})

		// Private / inactive groups handler
		tgBot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
			text := ""
			if update.Message != nil {
				text = update.Message.Text
			}
			return update.Message != nil && !database.IsGroupActive(context.Background(), update.Message.Chat.ID) && text != "/start" && text != "/help" && !strings.HasPrefix(text, "/init")
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
	botStartTime         = time.Now()
	botUsername          string
	providerCount        int
	globalLLM            *llm.Client
	processedMediaGroups sync.Map // key=media_group_id, value=true — dedup OCR per group
	mediaGroupMessages   sync.Map // key=media_group_id, value=[]int — message IDs in group
)

func handleSlashCommand(ctx context.Context, b *tgbot.Bot, update *models.Update, db *sql.DB, groupID int64, mh *handlers.MentionHandler, helpText string, cmd *handlers.CommandRegistry, settingsHandler *handlers.SettingsHandler, botMsgs *messages.Messages, promptRegistry *llm.PromptRegistry) {
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
			// Silently ignore or deny
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
		var g omsudb.Group
		existingGroup, err := dDB.GetGroup(ctx, msg.Chat.ID)
		if err != nil {
			g = omsudb.Group{
				ChatID:      msg.Chat.ID,
				Title:       msg.Chat.Title,
				APIToken:    fmt.Sprintf("init-%d-%d", msg.Chat.ID, time.Now().Unix()),
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
	publicURL := cfg.Setka.PublicURL
	if publicURL == "" {
		publicURL = fmt.Sprintf("http://localhost%s", cfg.API.Listen)
	}
	body := map[string]interface{}{
		"url":       publicURL + "/webhook/schedule",
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

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
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
