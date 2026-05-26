package buffer

import (
	"sync"
	"testing"
	"time"
)

func TestRingBufferConcurrentPush(t *testing.T) {
	t.Parallel()

	const capacity = 50
	rb := newRingBuffer(capacity)

	const numGoroutines = 4
	const pushesPerGoroutine = 100

	var wg sync.WaitGroup
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < pushesPerGoroutine; j++ {
				rb.push(bufferedMessage{
					MessageID: id*pushesPerGoroutine + j,
					Username:  "user",
					Text:      "hello",
					Timestamp: time.Now(),
				})
			}
		}(i)
	}
	wg.Wait()

	msgs := rb.all()
	if len(msgs) > capacity {
		t.Errorf("expected at most %d messages, got %d", capacity, len(msgs))
	}

	seen := make(map[int]bool)
	for _, m := range msgs {
		if seen[m.MessageID] {
			t.Errorf("duplicate message ID: %d", m.MessageID)
		}
		seen[m.MessageID] = true
	}
}
