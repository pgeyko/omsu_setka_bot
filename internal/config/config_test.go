package config

import (
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	cfg := &Config{
		LLM: llmConfig{
			DailyTokenLimit:     100000,
			RequestTimeoutSec:   10,
		},
		API: apiConfig{
			Listen: ":8081",
		},
		RateLimit: rateLimitConfig{
			GlobalPerUserPerMin: 5,
		},
	}

	if cfg.LLM.DailyTokenLimit != 100000 {
		t.Errorf("expected DailyTokenLimit 100000, got %d", cfg.LLM.DailyTokenLimit)
	}
	if cfg.API.Listen != ":8081" {
		t.Errorf("expected APIListen ':8081', got '%s'", cfg.API.Listen)
	}
	if cfg.RateLimit.GlobalPerUserPerMin != 5 {
		t.Errorf("expected GlobalPerUserPerMin 5, got %d", cfg.RateLimit.GlobalPerUserPerMin)
	}
}
