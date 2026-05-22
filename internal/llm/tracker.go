package llm

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

type Tracker struct {
	db             *sql.DB
	mu             sync.RWMutex
	dailyTokens    atomic.Int64
	lastResetDate  string
	dailyLimit     int64
	alertThreshold float64
}

func NewTracker(db *sql.DB, dailyLimit int64, alertThreshold float64) *Tracker {
	t := &Tracker{
		db:             db,
		dailyLimit:     dailyLimit,
		alertThreshold: alertThreshold,
	}
	t.restoreDailyFromDB()
	t.resetDailyIfNeeded()
	return t
}

func (t *Tracker) restoreDailyFromDB() {
	if t.db == nil {
		return
	}
	var loaded int64
	err := t.db.QueryRow(`SELECT COALESCE(SUM(input_tokens + output_tokens), 0) FROM llm_requests WHERE date(created_at) = date('now')`).Scan(&loaded)
	if err != nil {
		slog.Warn("failed to restore daily token count from DB", "error", err)
		return
	}
	if loaded > 0 {
		t.dailyTokens.Store(loaded)
		slog.Debug("restored daily token count from DB", "tokens", loaded)
	}
}

func (t *Tracker) resetDailyIfNeeded() {
	today := time.Now().Format("2006-01-02")
	t.mu.RLock()
	last := t.lastResetDate
	t.mu.RUnlock()
	if last == today {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lastResetDate == today {
		return
	}
	t.dailyTokens.Store(0)
	t.lastResetDate = today
	slog.Debug("daily token counter reset")
}

func (t *Tracker) LogRequest(ctx context.Context, reqType, provider, model string, inputTokens, outputTokens int, costUSD float64) error {
	t.resetDailyIfNeeded()
	t.dailyTokens.Add(int64(inputTokens + outputTokens))

	_, err := t.db.ExecContext(ctx, `
		INSERT INTO llm_requests (type, provider, input_tokens, output_tokens, model, cost_usd, created_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, reqType, provider, inputTokens, outputTokens, model, costUSD)
	if err != nil {
		return fmt.Errorf("failed to log llm request: %w", err)
	}

	t.checkAlerts()
	return nil
}

func (t *Tracker) checkAlerts() {
	used := t.dailyTokens.Load()
	if t.dailyLimit == 0 {
		return
	}
	pct := float64(used) / float64(t.dailyLimit)
	if pct >= 1.0 {
		slog.Warn("daily token limit reached, auto-classification disabled", "used", used, "limit", t.dailyLimit)
	} else if pct >= t.alertThreshold {
		slog.Warn("daily token usage approaching limit", "used", used, "limit", t.dailyLimit, "pct", fmt.Sprintf("%.0f%%", pct*100))
	}
}

func (t *Tracker) IsLimitReached() bool {
	t.resetDailyIfNeeded()
	return t.dailyLimit > 0 && t.dailyTokens.Load() >= t.dailyLimit
}
