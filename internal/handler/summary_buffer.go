package handlers

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type bufferedMessage struct {
	Username  string
	Text      string
	Timestamp time.Time
}

type SummaryBuffer struct {
	mu       sync.RWMutex
	topics   map[int]*ringBuffer
	capacity int
}

func NewSummaryBuffer(capacity int) *SummaryBuffer {
	return &SummaryBuffer{
		topics:   make(map[int]*ringBuffer),
		capacity: capacity,
	}
}

func (b *SummaryBuffer) Push(threadID int, username, text string) {
	if text == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	rb, ok := b.topics[threadID]
	if !ok {
		rb = newRingBuffer(b.capacity)
		b.topics[threadID] = rb
	}
	rb.push(bufferedMessage{
		Username:  username,
		Text:      text,
		Timestamp: time.Now(),
	})
}

func (b *SummaryBuffer) GetMessages(threadID int) string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	rb, ok := b.topics[threadID]
	if !ok {
		return ""
	}

	messages := rb.all()
	if len(messages) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, m := range messages {
		sb.WriteString(fmt.Sprintf("[%s] @%s: %s\n",
			m.Timestamp.Format("15:04"),
			m.Username,
			m.Text,
		))
	}
	return sb.String()
}

type ringBuffer struct {
	buf  []bufferedMessage
	pos  int
	full bool
	mu   sync.Mutex
}

func newRingBuffer(capacity int) *ringBuffer {
	return &ringBuffer{
		buf: make([]bufferedMessage, capacity),
	}
}

func (rb *ringBuffer) push(msg bufferedMessage) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buf[rb.pos] = msg
	rb.pos++
	if rb.pos >= len(rb.buf) {
		rb.pos = 0
		rb.full = true
	}
}

func (rb *ringBuffer) all() []bufferedMessage {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if !rb.full {
		result := make([]bufferedMessage, rb.pos)
		copy(result, rb.buf[:rb.pos])
		return result
	}

	result := make([]bufferedMessage, len(rb.buf))
	copy(result, rb.buf[rb.pos:])
	copy(result[len(rb.buf)-rb.pos:], rb.buf[:rb.pos])
	return result
}
