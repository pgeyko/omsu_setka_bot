package telegram

import (
	"context"
	"log/slog"
	"sync"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type adminCacheEntry struct {
	isAdmin   bool
	checkedAt time.Time
}

type AdminCache struct {
	bot     *tgbot.Bot
	groupID int64
	cache   map[int64]adminCacheEntry
	mu      sync.RWMutex
}

func NewAdminCache(bot *tgbot.Bot, groupID int64) *AdminCache {
	return &AdminCache{
		bot:     bot,
		groupID: groupID,
		cache:   make(map[int64]adminCacheEntry),
	}
}

func (ac *AdminCache) IsAdmin(ctx context.Context, userID int64) bool {
	ac.mu.RLock()
	entry, ok := ac.cache[userID]
	ac.mu.RUnlock()

	if ok && time.Since(entry.checkedAt) < 10*time.Minute {
		return entry.isAdmin
	}

	members, err := ac.bot.GetChatAdministrators(ctx, &tgbot.GetChatAdministratorsParams{
		ChatID: ac.groupID,
	})
	if err != nil {
		slog.Error("failed to get admins", "error", err)
		return false
	}

	isAdmin := false
	adminIDs := make(map[int64]bool)
	for _, m := range members {
		if uid := GetChatMemberUserID(m); uid != 0 {
			adminIDs[uid] = true
			if uid == userID {
				isAdmin = true
			}
		}
	}

	ac.mu.Lock()
	// Clean up old entries
	for uid, e := range ac.cache {
		if time.Since(e.checkedAt) > 10*time.Minute {
			delete(ac.cache, uid)
		}
	}

	// Cache true for all current admins
	for uid := range adminIDs {
		ac.cache[uid] = adminCacheEntry{
			isAdmin:   true,
			checkedAt: time.Now(),
		}
	}

	// Cache false for this user if they are not admin
	if !isAdmin {
		ac.cache[userID] = adminCacheEntry{
			isAdmin:   false,
			checkedAt: time.Now(),
		}
	}

	ac.mu.Unlock()

	return isAdmin
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
