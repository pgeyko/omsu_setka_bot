package telegram

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type SessionState int

const (
	StateNone SessionState = iota
	StateWaitingForKB
	StateWaitingForPersona
	StateWaitingForPrompt
	StateWaitingForScheduleSearch
)

type sessionEntry struct {
	state     SessionState
	expiresAt time.Time
}

type SessionStore struct {
	mu    sync.RWMutex
	store map[string]sessionEntry
	ttl   time.Duration
}

func NewSessionStore(ttl ...time.Duration) *SessionStore {
	d := 30 * time.Minute
	if len(ttl) > 0 && ttl[0] > 0 {
		d = ttl[0]
	}
	return &SessionStore{
		store: make(map[string]sessionEntry),
		ttl:   d,
	}
}

func (s *SessionStore) Set(chatID int64, userID int64, state SessionState) {
	key := getSessionKey(chatID, userID)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[key] = sessionEntry{
		state:     state,
		expiresAt: time.Now().Add(s.ttl),
	}
}

func (s *SessionStore) Get(chatID int64, userID int64) SessionState {
	key := getSessionKey(chatID, userID)
	s.mu.RLock()
	entry, ok := s.store[key]
	s.mu.RUnlock()

	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			s.Clear(chatID, userID)
		}
		return StateNone
	}
	return entry.state
}

func (s *SessionStore) Clear(chatID int64, userID int64) {
	key := getSessionKey(chatID, userID)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, key)
}

func (s *SessionStore) StartCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.deleteExpired()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *SessionStore) deleteExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, entry := range s.store {
		if now.After(entry.expiresAt) {
			delete(s.store, k)
		}
	}
}

func getSessionKey(chatID int64, userID int64) string {
	return fmt.Sprintf("%d:%d", chatID, userID)
}
