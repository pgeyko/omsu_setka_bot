package api

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	_ "omsu_bot/docs"
	"omsu_bot/internal/llm"
	"omsu_bot/internal/persona"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/swagger"
)

type BotSender interface {
	Send(ctx context.Context, chatID int64, text string) error
}

type Server struct {
	App               *fiber.App
	DB                *sql.DB
	Persona           *persona.Store
	Prompts           *llm.PromptRegistry
	AuthMiddleware    *AuthMiddleware
	SwaggerEnabled    bool
	AppEnv            string
	CORSOrigin        string
	Chain             *llm.Chain
	LLMClient         *llm.Client
	TelegramBot       BotSender
	TelegramGroupID   int64
	SkipFallbackModel bool
	SetkaBaseURL      string
	SetkaAdminKey     string
	WebhookSecret     string
	ListenAddr        string
}

func NewServer(db *sql.DB, persona *persona.Store, prompts *llm.PromptRegistry, auth *AuthMiddleware,
	swaggerEnabled bool, appEnv string, corsOrigin string, chain *llm.Chain, llmClient *llm.Client,
	tgBot BotSender, tgGroupID int64, skipFallbackModel bool,
	rateLimitGeneral, rateLimitSearch, rateLimitWindowSec int,
	setkaBaseURL, setkaAdminKey, webhookSecret, listenAddr string) *Server {

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		BodyLimit:             1024,
		ReadTimeout:           5 * time.Second,
		WriteTimeout:          5 * time.Second,
		ReadBufferSize:        4096,
		ProxyHeader:           fiber.HeaderXForwardedFor,
		TrustedProxies:        []string{"172.16.0.0/12", "192.168.0.0/16", "10.0.0.0/8"},
		CaseSensitive:         true,
	})

	app.Use(recover.New())
	app.Use(securityHeaders())
	app.Use(cors.New(cors.Config{
		AllowOrigins: corsOrigin,
		AllowHeaders: "Authorization, Content-Type",
	}))
	app.Use(etag.New())

	if appEnv != "production" {
		app.Use(requestLogger())
	}

	s := &Server{
		App:               app,
		DB:                db,
		Persona:           persona,
		Prompts:           prompts,
		AuthMiddleware:    auth,
		SwaggerEnabled:    swaggerEnabled,
		AppEnv:            appEnv,
		CORSOrigin:        corsOrigin,
		Chain:             chain,
		LLMClient:         llmClient,
		TelegramBot:       tgBot,
		TelegramGroupID:   tgGroupID,
		SkipFallbackModel: skipFallbackModel,
		SetkaBaseURL:      setkaBaseURL,
		SetkaAdminKey:     setkaAdminKey,
		WebhookSecret:     webhookSecret,
		ListenAddr:        listenAddr,
	}

	startTime := time.Now()
	app.Get("/health", func(c *fiber.Ctx) error {
		if err := db.PingContext(c.Context()); err != nil {
			return c.Status(503).JSON(fiber.Map{"status": "unhealthy", "db": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "ok", "uptime": time.Since(startTime).String()})
	})

	s.setupRoutes(rateLimitGeneral, rateLimitSearch, rateLimitWindowSec)
	return s
}

func securityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; font-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self';")
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		return c.Next()
	}
}

func requestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		slog.Debug("api",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"latency", time.Since(start),
			"ip", c.IP(),
		)
		return err
	}
}

func (s *Server) setupRoutes(rateLimitGeneral, rateLimitSearch, rateLimitWindowSec int) {
	if s.SwaggerEnabled && s.AppEnv != "production" {
		s.App.Get("/swagger/*", swagger.HandlerDefault)
	}

	s.App.Post("/api/auth/token", s.AuthMiddleware.Login)

	window := time.Duration(rateLimitWindowSec) * time.Second
	api := s.App.Group("/api", s.AuthMiddleware.RequireAuth)
	api.Use(limiter.New(limiter.Config{
		Max:        rateLimitGeneral,
		Expiration: window,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return respondError(c, fiber.StatusTooManyRequests, ErrRateLimited, "rate limit exceeded")
		},
	}))

	api.Get("/persona", s.handleGetPersona)
	api.Put("/persona", s.handleUpdatePersona)
	api.Post("/persona/reset", s.handleResetPersona)

	// Group CRUD and Context Routes
	api.Get("/groups", s.handleListGroups)
	api.Post("/groups", s.handleCreateGroup)
	api.Get("/groups/:chat_id", s.handleGetGroup)
	api.Put("/groups/:chat_id", s.handleUpdateGroup)
	api.Delete("/groups/:chat_id", s.handleDeleteGroup)

	// Superadmin Routes
	api.Get("/admin/superadmins", s.handleListSuperadmins)
	api.Post("/admin/superadmins", s.handleAddSuperadmin)
	api.Delete("/admin/superadmins/:user_id", s.handleRemoveSuperadmin)
	api.Post("/admin/groups/register-webhooks", s.handleRegisterWebhooks)

	api.Get("/groups/:chat_id/context/persona", s.handleGetGroupPersona)
	api.Put("/groups/:chat_id/context/persona", s.handleUploadPersona)
	api.Get("/groups/:chat_id/context/system-prompt", s.handleGetGroupSystemPrompt)
	api.Put("/groups/:chat_id/context/system-prompt", s.handleUploadSystemPrompt)
	api.Get("/groups/:chat_id/context/knowledge", s.handleGetGroupKnowledge)
	api.Put("/groups/:chat_id/context/knowledge", s.handleUploadKnowledge)
	api.Get("/groups/:chat_id/context/features", s.handleGetGroupFeatures)
	api.Put("/groups/:chat_id/context/features", s.handleUploadFeatures)

	api.Get("/topics", s.handleGetTopics)
	api.Post("/topics", s.handleCreateTopic)
	api.Get("/topics/:id", s.handleGetTopic)
	api.Put("/topics/:id", s.handleUpdateTopic)
	api.Delete("/topics/:id", s.handleDeleteTopic)
	api.Post("/topics/:id/close", s.handleCloseTopic)
	api.Post("/topics/:id/open", s.handleOpenTopic)

	api.Get("/permissions", s.handleGetPermissions)
	api.Put("/permissions/:command", s.handleUpdatePermission)

	api.Get("/prompts", s.handleGetPrompts)
	api.Get("/prompts/:name", s.handleGetPrompt)
	api.Put("/prompts/:name", s.handleUpdatePrompt)
	api.Delete("/prompts/:name", s.handleDeletePrompt)

	api.Get("/config", s.handleGetConfig)
	api.Put("/config", s.handleUpdateConfig)

	api.Get("/stats/tokens", s.handleStatsTokens)
	api.Get("/stats/requests", s.handleStatsRequests)
	api.Get("/stats/forwards", s.handleStatsForwards)
	api.Get("/stats/messages", s.handleStatsMessages)
	api.Get("/stats/providers", s.handleStatsProviders)
	api.Get("/stats/check-providers", s.handleCheckProviders)
	api.Post("/stats/test-model", limiter.New(limiter.Config{
		Max: rateLimitSearch, Expiration: window,
		KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
		LimitReached: func(c *fiber.Ctx) error {
			return respondError(c, fiber.StatusTooManyRequests, ErrRateLimited, "rate limit exceeded")
		},
	}), s.handleTestModel)

	api.Post("/bot/send", s.handleSendMessage)

	api.Get("/schedule/snapshots", s.handleScheduleSnapshots)
	api.Get("/schedule/anomalies", s.handleScheduleAnomalies)
}
