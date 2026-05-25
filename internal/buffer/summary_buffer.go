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
	MessageID int
	Username  string
	Text      string
	Timestamp time.Time
}

type dbWrite struct {
	ChatID    int64
	ThreadID  int
	MessageID int
	Username  string
	Text      string
	Timestamp time.Time
}

type SummaryBuffer struct {
	mu       sync.RWMutex
	topics   map[string]*ringBuffer // Key: "chatID:threadID"
	capacity int
	db       *sql.DB
	dbWrites chan dbWrite
	done     chan struct{}
}

func NewSummaryBuffer(db *sql.DB, capacity int) *SummaryBuffer {
	sb := &SummaryBuffer{
		topics:   make(map[string]*ringBuffer),
		capacity: capacity,
		db:       db,
		dbWrites: make(chan dbWrite, 256),
		done:     make(chan struct{}),
	}
	sb.restoreFromDB()
	if db != nil {
		go sb.dbWriter()
	}
	return sb
}

func (b *SummaryBuffer) dbWriter() {
	for w := range b.dbWrites {
		_, err := b.db.Exec(`
			INSERT INTO message_buffer (chat_id, thread_id, message_id, username, text, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			w.ChatID, w.ThreadID, w.MessageID, w.Username, w.Text, w.Timestamp,
		)
		if err != nil {
			slog.Error("failed to save message to buffer db", "error", err)
		}
	}
	close(b.done)
}

func (b *SummaryBuffer) restoreFromDB() {
	if b.db == nil {
		return
	}
	rows, err := b.db.Query(`
		SELECT chat_id, thread_id, message_id, username, text, created_at
		FROM (
			SELECT chat_id, thread_id, message_id, username, text, created_at,
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
		var messageID int
		var username, text string
		var createdAt time.Time
		if err := rows.Scan(&chatID, &threadID, &messageID, &username, &text, &createdAt); err == nil {
			key := fmt.Sprintf("%d:%d", chatID, threadID)
			rb, ok := b.topics[key]
			if !ok {
				rb = newRingBuffer(b.capacity)
				b.topics[key] = rb
			}
			rb.push(bufferedMessage{
				MessageID: messageID,
				Username:  username,
				Text:      text,
				Timestamp: createdAt,
			})
		}
	}
}

func (b *SummaryBuffer) Push(chatID int64, threadID int, messageID int, username, text string) {
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
		MessageID: messageID,
		Username:  username,
		Text:      text,
		Timestamp: time.Now(),
	}
	rb.push(msg)

	select {
	case b.dbWrites <- dbWrite{
		ChatID:    chatID,
		ThreadID:  threadID,
		MessageID: msg.MessageID,
		Username:  msg.Username,
		Text:      msg.Text,
		Timestamp: msg.Timestamp,
	}:
	default:
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

func (b *SummaryBuffer) RemoveMessages(chatID int64, threadID int, messageIDs []int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	key := fmt.Sprintf("%d:%d", chatID, threadID)
	rb, ok := b.topics[key]
	if !ok {
		return
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Get all current messages
	var current []bufferedMessage
	if !rb.full {
		current = rb.buf[:rb.pos]
	} else {
		current = make([]bufferedMessage, len(rb.buf))
		copy(current, rb.buf[rb.pos:])
		copy(current[len(rb.buf)-rb.pos:], rb.buf[:rb.pos])
	}

	// Filter out the deleted message IDs
	toDelete := make(map[int]bool)
	for _, id := range messageIDs {
		toDelete[id] = true
	}

	var filtered []bufferedMessage
	for _, msg := range current {
		if !toDelete[msg.MessageID] {
			filtered = append(filtered, msg)
		}
	}

	// Rebuild the ring buffer with filtered messages
	newRb := newRingBuffer(b.capacity)
	for _, msg := range filtered {
		newRb.push(msg)
	}

	b.topics[key] = newRb
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
