package buffer

import (
	"database/sql"
	"fmt"
	"log/slog"
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
	topics   map[string]*ringBuffer // Key: "chatID:threadID"
	capacity int
	db       *sql.DB
}

func NewSummaryBuffer(db *sql.DB, capacity int) *SummaryBuffer {
	sb := &SummaryBuffer{
		topics:   make(map[string]*ringBuffer),
		capacity: capacity,
		db:       db,
	}
	sb.restoreFromDB()
	return sb
}

func (b *SummaryBuffer) restoreFromDB() {
	if b.db == nil {
		return
	}
	rows, err := b.db.Query(`
		SELECT chat_id, thread_id, username, text, created_at
		FROM (
			SELECT chat_id, thread_id, username, text, created_at,
				ROW_NUMBER() OVER(PARTITION BY chat_id, thread_id ORDER BY created_at DESC) as rn
			FROM message_buffer
		)
		WHERE rn <= ?
		ORDER BY chat_id, thread_id, created_at ASC
	`, b.capacity)
	if err != nil {
		slog.Error("failed to restore summary buffer", "error", err)
		return
	}
	defer rows.Close()

	b.mu.Lock()
	defer b.mu.Unlock()
	for rows.Next() {
		var chatID int64
		var threadID int
		var username, text string
		var createdAt time.Time
		if err := rows.Scan(&chatID, &threadID, &username, &text, &createdAt); err == nil {
			key := fmt.Sprintf("%d:%d", chatID, threadID)
			rb, ok := b.topics[key]
			if !ok {
				rb = newRingBuffer(b.capacity)
				b.topics[key] = rb
			}
			rb.push(bufferedMessage{
				Username:  username,
				Text:      text,
				Timestamp: createdAt,
			})
		}
	}
}

func (b *SummaryBuffer) Push(chatID int64, threadID int, username, text string) {
	if text == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	key := fmt.Sprintf("%d:%d", chatID, threadID)
	rb, ok := b.topics[key]
	if !ok {
		rb = newRingBuffer(b.capacity)
		b.topics[key] = rb
	}
	msg := bufferedMessage{
		Username:  username,
		Text:      text,
		Timestamp: time.Now(),
	}
	rb.push(msg)

	if b.db != nil {
		go func() {
			_, err := b.db.Exec(`
				INSERT INTO message_buffer (chat_id, thread_id, username, text, created_at)
				VALUES (?, ?, ?, ?, ?)`,
				chatID, threadID, msg.Username, msg.Text, msg.Timestamp,
			)
			if err != nil {
				slog.Error("failed to save message to buffer db", "error", err)
			}
		}()
	}
}

func (b *SummaryBuffer) GetMessages(chatID int64, threadID int) string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	key := fmt.Sprintf("%d:%d", chatID, threadID)
	rb, ok := b.topics[key]
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
