package config

import (
	"log"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type telegramConfig struct {
	Token       string `yaml:"token" env:"BOT_TOKEN" env-required:"true"`
	GroupID     int64  `yaml:"group_id" env:"GROUP_ID" env-required:"true"`
	OmsuGroupID int    `yaml:"omsu_group_id" env:"OMSU_GROUP_ID" env-default:"0"`
}

type llmConfig struct {
	DailyTokenLimit             int                 `yaml:"daily_token_limit" env:"DAILY_TOKEN_LIMIT" env-default:"100000"`
	ClassifyConfidenceThreshold float64             `yaml:"classify_confidence_threshold" env:"CLASSIFY_CONFIDENCE_THRESHOLD" env-default:"0.75"`
	RequestTimeoutSec           int                 `yaml:"request_timeout_sec" env:"LLM_REQUEST_TIMEOUT_SEC" env-default:"10"`
	CircuitBreakerFailures      int                 `yaml:"circuit_breaker_failures" env:"CIRCUIT_BREAKER_FAILURES" env-default:"3"`
	CircuitBreakerCooldownMin   int                 `yaml:"circuit_breaker_cooldown_min" env:"CIRCUIT_BREAKER_COOLDOWN_MIN" env-default:"5"`
	SkipFallbackModel           bool                `yaml:"skip_fallback_model" env:"LLM_SKIP_FALLBACK_MODEL" env-default:"false"`
	Providers                   []LLMProviderConfig `yaml:"providers" env:"-"`
}

type apiConfig struct {
	Listen      string `yaml:"listen" env:"API_LISTEN" env-default:":8081"`
	AdminSecret string `yaml:"admin_secret" env:"ADMIN_SECRET" env-required:"true"`
	JWTSecret   string `yaml:"jwt_secret" env:"JWT_SECRET" env-required:"true"`
	CORSOrigin  string `yaml:"cors_origin" env:"CORS_ORIGIN" env-default:"*"`
}

type webhookConfig struct {
	ScheduleSecret   string `yaml:"schedule_secret" env:"SCHEDULE_WEBHOOK_SECRET" env-required:"true"`
	AnnounceThreadID int    `yaml:"announce_thread_id" env:"ANNOUNCE_THREAD_ID" env-default:"0"`
}

type dbConfig struct {
	Path string `yaml:"path" env:"DB_PATH" env-default:"./data/groupbot.db"`
}

type rateLimitConfig struct {
	GlobalPerUserPerMin int `yaml:"global_per_user_per_min" env:"RATE_LIMIT_GLOBAL_PER_USER" env-default:"5"`
	SummaryPerUserMin   int `yaml:"summary_per_user_min" env:"RATE_LIMIT_SUMMARY_PER_USER" env-default:"30"`
	APIGeneral          int `yaml:"api_general" env:"RATE_LIMIT_API_GENERAL" env-default:"120"`
	APISearch           int `yaml:"api_search" env:"RATE_LIMIT_API_SEARCH" env-default:"30"`
	APIWindow           int `yaml:"api_window_sec" env:"RATE_LIMIT_API_WINDOW_SEC" env-default:"60"`
}

type setkaConfig struct {
	BaseURL   string `yaml:"base_url" env:"SETKA_BASE_URL" env-default:""`
	AdminKey  string `yaml:"admin_key" env:"SETKA_ADMIN_KEY" env-default:""`
	PublicURL string `yaml:"public_url" env:"SETKA_PUBLIC_URL" env-default:""`
}

type loggingConfig struct {
	Level  string `yaml:"level" env:"LOG_LEVEL" envDefault:"info"`
	Format string `yaml:"format" env:"LOG_FORMAT" envDefault:"text"`
}

type Config struct {
	AppEnv         string `yaml:"app_env" env:"APP_ENV" envDefault:"development"`
	SwaggerEnabled bool   `yaml:"swagger_enabled" env:"SWAGGER_ENABLED" envDefault:"false"`
	Timezone       string `yaml:"timezone" env:"TZ" envDefault:"Asia/Omsk"`

	Telegram  telegramConfig  `yaml:"telegram"`
	LLM       llmConfig       `yaml:"llm"`
	API       apiConfig       `yaml:"api"`
	Webhook   webhookConfig   `yaml:"webhook"`
	DB        dbConfig        `yaml:"db"`
	RateLimit rateLimitConfig `yaml:"rate_limit"`
	Setka     setkaConfig     `yaml:"setka"`
	Logging   loggingConfig   `yaml:"logging"`
}

type LLMProviderConfig struct {
	Name           string   `yaml:"name"`
	Type           string   `yaml:"type"`
	APIKey         string   `yaml:"api_key"`
	Model          string   `yaml:"model"`
	FallbackModels []string `yaml:"fallback_models"`
	Multimodal     bool     `yaml:"multimodal"`
	Priority       int      `yaml:"priority"`
	BaseURL        string   `yaml:"base_url" env-default:""`
}

func Load(configPath string) *Config {
	cfg := &Config{}

	if err := cleanenv.ReadConfig(configPath, cfg); err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	for i := range cfg.LLM.Providers {
		cfg.LLM.Providers[i].APIKey = os.ExpandEnv(cfg.LLM.Providers[i].APIKey)
		if cfg.LLM.Providers[i].BaseURL != "" {
			cfg.LLM.Providers[i].BaseURL = os.ExpandEnv(cfg.LLM.Providers[i].BaseURL)
		}
	}

	return cfg
}
