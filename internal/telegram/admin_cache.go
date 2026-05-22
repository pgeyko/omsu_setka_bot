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
	checkedAt time.Time
}

type AdminCache struct {
	bot     *tgbot.Bot
	cache   map[string]adminCacheEntry // Key format: "chatID:userID"
	mu      sync.RWMutex
}

func NewAdminCache(bot *tgbot.Bot, groupID int64) *AdminCache {
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
	for k, e := range ac.cache {
		if time.Since(e.checkedAt) > 10*time.Minute {
			delete(ac.cache, k)
		}
	}

	// Cache true for all current admins
	for uid := range adminIDs {
		k := fmt.Sprintf("%d:%d", chatID, uid)
		ac.cache[k] = adminCacheEntry{
			isAdmin:   true,
			checkedAt: time.Now(),
		}
	}

	// Cache false for this user if they are not admin
	if !isAdmin {
		ac.cache[key] = adminCacheEntry{
			isAdmin:   false,
			checkedAt: time.Now(),
		}
	}

	ac.mu.Unlock()

	return isAdmin
}

func (ac *AdminCache) IsOwner(ctx context.Context, chatID int64, userID int64) bool {
	if chatID == 0 || ac.bot == nil {
		return false
	}
	members, err := ac.bot.GetChatAdministrators(ctx, &tgbot.GetChatAdministratorsParams{
		ChatID: chatID,
	})
	if err != nil {
		slog.Error("failed to get admins for owner check", "error", err, "chat_id", chatID)
		return false
	}
	for _, m := range members {
		if m.Type == models.ChatMemberTypeOwner {
			if uid := GetChatMemberUserID(m); uid == userID {
				return true
			}
		}
	}
	return false
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
