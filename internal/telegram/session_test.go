package telegram

import (
	"sync"
	"testing"
)

func TestSessionStore(t *testing.T) {
	store := NewSessionStore()

	// Initial check
	if state := store.Get(123, 456); state != StateNone {
		t.Errorf("expected initial state to be StateNone, got %v", state)
	}

	// Set state
	store.Set(123, 456, StateWaitingForKB)
	if state := store.Get(123, 456); state != StateWaitingForKB {
		t.Errorf("expected state to be StateWaitingForKB, got %v", state)
	}

	// Set state on another chat/user
	store.Set(111, 222, StateWaitingForScheduleSearch)
	if state := store.Get(111, 222); state != StateWaitingForScheduleSearch {
		t.Errorf("expected state to be StateWaitingForScheduleSearch, got %v", state)
	}
	// Check the first one remains unchanged
	if state := store.Get(123, 456); state != StateWaitingForKB {
		t.Errorf("expected first user state to be StateWaitingForKB, got %v", state)
	}

	// Clear state
	store.Clear(123, 456)
	if state := store.Get(123, 456); state != StateNone {
		t.Errorf("expected cleared state to be StateNone, got %v", state)
	}

	// Concurrent read/write test
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(id int64) {
			defer wg.Done()
			store.Set(id, id, StateWaitingForPersona)
		}(int64(i))
		go func(id int64) {
			defer wg.Done()
			_ = store.Get(id, id)
		}(int64(i))
	}
	wg.Wait()
}
