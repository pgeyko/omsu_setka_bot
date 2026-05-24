package handlers

import (
	"context"
	"sync"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type RateLimiter struct {
	mu       sync.Mutex
	requests map[int64][]time.Time
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[int64][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (r *RateLimiter) Allow(userID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	times := r.requests[userID]
	var valid []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= r.limit {
		r.requests[userID] = valid
		return false
	}

	valid = append(valid, now)
	r.requests[userID] = valid
	return true
}

type Middleware struct {
	rateLimit *RateLimiter
}

func NewMiddleware(limitPerMin int) *Middleware {
	return &Middleware{
		rateLimit: NewRateLimiter(limitPerMin, 1*time.Minute),
	}
}

func (m *Middleware) RateLimit(next tgbot.HandlerFunc) tgbot.HandlerFunc {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message != nil && update.Message.From != nil && !m.rateLimit.Allow(update.Message.From.ID) {
			b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "⏳ Слишком много запросов. Подожди минуту.",
			})
			return
		}
		next(ctx, b, update)
	}
}
