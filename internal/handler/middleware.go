package handler

import (
	"context"
	"sync"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type rateLimiterShard struct {
	mu       sync.Mutex
	requests map[int64][]time.Time
}

type RateLimiter struct {
	shards []*rateLimiterShard
	limit  int
	window time.Duration
}

const numShards = 16

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	r := &RateLimiter{
		shards: make([]*rateLimiterShard, numShards),
		limit:  limit,
		window: window,
	}
	for i := range r.shards {
		r.shards[i] = &rateLimiterShard{
			requests: make(map[int64][]time.Time),
		}
	}
	return r
}

func (r *RateLimiter) shard(userID int64) *rateLimiterShard {
	return r.shards[uint64(userID)&(numShards-1)]
}

func (r *RateLimiter) Allow(userID int64) bool {
	s := r.shard(userID)
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	times := s.requests[userID]
	var valid []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= r.limit {
		s.requests[userID] = valid
		return false
	}

	valid = append(valid, now)
	s.requests[userID] = valid

	// Evict stale entries periodically to prevent unbounded growth
	if len(s.requests) > 10000 {
		globalCutoff := now.Add(-2 * r.window)
		for uid, ts := range s.requests {
			if len(ts) > 0 && ts[len(ts)-1].Before(globalCutoff) {
				delete(s.requests, uid)
			}
		}
	}

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

func (m *Middleware) Allow(userID int64) bool {
	return m.rateLimit.Allow(userID)
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
