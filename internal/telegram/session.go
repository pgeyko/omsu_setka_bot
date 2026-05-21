package telegram

import (
	"fmt"
	"sync"
)

type SessionState int

const (
	StateNone SessionState = iota
	StateWaitingForKB
	StateWaitingForPersona
	StateWaitingForPrompt
	StateWaitingForScheduleSearch
)

type SessionStore struct {
	mu    sync.RWMutex
	store map[string]SessionState
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		store: make(map[string]SessionState),
	}
}

func (s *SessionStore) Set(chatID int64, userID int64, state SessionState) {
	key := getSessionKey(chatID, userID)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[key] = state
}

func (s *SessionStore) Get(chatID int64, userID int64) SessionState {
	key := getSessionKey(chatID, userID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store[key]
}

func (s *SessionStore) Clear(chatID int64, userID int64) {
	key := getSessionKey(chatID, userID)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, key)
}

func getSessionKey(chatID int64, userID int64) string {
	return fmt.Sprintf("%d:%d", chatID, userID)
}
