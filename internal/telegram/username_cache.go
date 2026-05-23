package telegram

import (
	"strings"
	"sync"
)

type UsernameCache struct {
	mu    sync.RWMutex
	cache map[string]int64
}

func NewUsernameCache() *UsernameCache {
	return &UsernameCache{
		cache: make(map[string]int64),
	}
}

func (uc *UsernameCache) Store(username string, userID int64) {
	if username == "" {
		return
	}
	username = strings.ToLower(strings.TrimPrefix(username, "@"))
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.cache[username] = userID
}

func (uc *UsernameCache) Get(username string) (int64, bool) {
	username = strings.ToLower(strings.TrimPrefix(username, "@"))
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	uid, ok := uc.cache[username]
	return uid, ok
}

func (uc *UsernameCache) All() []string {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	names := make([]string, 0, len(uc.cache))
	for name := range uc.cache {
		names = append(names, name)
	}
	return names
}
