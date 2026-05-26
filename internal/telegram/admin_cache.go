package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type adminCacheEntry struct {
	isAdmin   bool
	isOwner   bool
	checkedAt time.Time
}

type AdminCache struct {
	bot     *tgbot.Bot
	cache   map[string]adminCacheEntry // Key format: "chatID:userID"
	mu      sync.RWMutex
}

func NewAdminCache(bot *tgbot.Bot) *AdminCache {
	return &AdminCache{
		bot:   bot,
		cache: make(map[string]adminCacheEntry),
	}
}

func (ac *AdminCache) IsAdmin(ctx context.Context, chatID int64, userID int64) bool {
	if chatID == 0 {
		return false
	}
	key := fmt.Sprintf("%d:%d", chatID, userID)
	ac.mu.RLock()
	entry, ok := ac.cache[key]
	ac.mu.RUnlock()

	if ok && time.Since(entry.checkedAt) < 10*time.Minute {
		return entry.isAdmin
	}

	if ac.bot == nil {
		return false
	}

	members, err := ac.bot.GetChatAdministrators(ctx, &tgbot.GetChatAdministratorsParams{
		ChatID: chatID,
	})
	if err != nil {
		slog.Error("failed to get admins", "error", err, "chat_id", chatID)
		return false
	}

	isAdmin := false
	now := time.Now()
	for _, m := range members {
		if uid := GetChatMemberUserID(m); uid != 0 {
			isOwner := m.Type == models.ChatMemberTypeOwner
			k := fmt.Sprintf("%d:%d", chatID, uid)
			ac.cache[k] = adminCacheEntry{
				isAdmin:   true,
				isOwner:   isOwner,
				checkedAt: now,
			}
			if uid == userID {
				isAdmin = true
			}
		}
	}

	ac.mu.Lock()
	// Clean up old entries
	for k, e := range ac.cache {
		if now.Sub(e.checkedAt) > 10*time.Minute {
			delete(ac.cache, k)
		}
	}

	// Cache false for this user if they are not admin
	if !isAdmin {
		ac.cache[key] = adminCacheEntry{
			isAdmin:   false,
			checkedAt: now,
		}
	}

	ac.mu.Unlock()

	return isAdmin
}

func (ac *AdminCache) IsOwner(ctx context.Context, chatID int64, userID int64) bool {
	if chatID == 0 || ac.bot == nil {
		return false
	}
	key := fmt.Sprintf("%d:%d", chatID, userID)
	ac.mu.RLock()
	entry, ok := ac.cache[key]
	ac.mu.RUnlock()
	if ok && time.Since(entry.checkedAt) < 10*time.Minute {
		return entry.isOwner
	}
	// Cache miss — delegate to IsAdmin which populates the cache with all admins
	ac.IsAdmin(ctx, chatID, userID)
	ac.mu.RLock()
	entry, ok = ac.cache[key]
	ac.mu.RUnlock()
	return ok && entry.isOwner
}

func GetChatMemberUserID(m models.ChatMember) int64 {
	switch m.Type {
	case models.ChatMemberTypeOwner:
		if m.Owner != nil {
			return m.Owner.User.ID
		}
	case models.ChatMemberTypeAdministrator:
		if m.Administrator != nil {
			return m.Administrator.User.ID
		}
	}
	return 0
}
