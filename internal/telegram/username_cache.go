package telegram

import (
	"strings"
	"sync"
	"time"
)

const (
	maxUsernameCacheSize  = 10000
	usernameCacheTTL      = 6 * time.Hour
	usernameCacheInterval = 30 * time.Minute
)

type cacheEntry struct {
	userID    int64
	createdAt time.Time
}

type UsernameCache struct {
	mu       sync.RWMutex
	cache    map[string]*cacheEntry
	evictBuf []string
	done     chan struct{}
}

func NewUsernameCache() *UsernameCache {
	uc := &UsernameCache{
		cache:    make(map[string]*cacheEntry),
		evictBuf: make([]string, 0, 128),
		done:     make(chan struct{}),
	}
	go uc.cleanupLoop()
	return uc
}

func (uc *UsernameCache) cleanupLoop() {
	t := time.NewTicker(usernameCacheInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			uc.evictExpired()
		case <-uc.done:
			return
		}
	}
}

func (uc *UsernameCache) evictExpired() {
	cutoff := time.Now().Add(-usernameCacheTTL)
	uc.mu.Lock()
	defer uc.mu.Unlock()
	for k, v := range uc.cache {
		if v.createdAt.Before(cutoff) {
			delete(uc.cache, k)
		}
	}
}

func (uc *UsernameCache) Store(username string, userID int64) {
	if username == "" {
		return
	}
	username = strings.ToLower(strings.TrimPrefix(username, "@"))
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if _, exists := uc.cache[username]; !exists && len(uc.cache) >= maxUsernameCacheSize {
		for k := range uc.cache {
			uc.evictBuf = append(uc.evictBuf, k)
			if len(uc.evictBuf) >= 64 {
				break
			}
		}
		for _, k := range uc.evictBuf {
			delete(uc.cache, k)
		}
		uc.evictBuf = uc.evictBuf[:0]
	}
	uc.cache[username] = &cacheEntry{userID: userID, createdAt: time.Now()}
}

func (uc *UsernameCache) Get(username string) (int64, bool) {
	username = strings.ToLower(strings.TrimPrefix(username, "@"))
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	entry, ok := uc.cache[username]
	if !ok {
		return 0, false
	}
	return entry.userID, true
}

func (uc *UsernameCache) Close() {
	close(uc.done)
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
