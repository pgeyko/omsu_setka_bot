package buffer

import (
	"strings"
	"testing"
)

func TestSummaryBuffer_PushAndGet(t *testing.T) {
	buf := NewSummaryBuffer(nil, 3)

	buf.Push(12345, 1, 101, "user1", "hello")
	buf.Push(12345, 1, 102, "user2", "world")

	msgs := buf.GetMessages(12345, 1)
	if msgs == "" {
		t.Fatal("expected non-empty messages")
	}
}

func TestSummaryBuffer_EmptyTopic(t *testing.T) {
	buf := NewSummaryBuffer(nil, 10)

	msgs := buf.GetMessages(12345, 999)
	if msgs != "" {
		t.Errorf("expected empty messages for unknown topic, got '%s'", msgs)
	}
}

func TestSummaryBuffer_Capacity(t *testing.T) {
	buf := NewSummaryBuffer(nil, 3)

	for i := 0; i < 10; i++ {
		buf.Push(12345, 1, i, "user", "msg")
	}

	msgs := buf.GetMessages(12345, 1)
	if msgs == "" {
		t.Fatal("expected messages even after overflow")
	}
}

func TestSummaryBuffer_IgnoresEmptyText(t *testing.T) {
	buf := NewSummaryBuffer(nil, 10)

	buf.Push(12345, 1, 201, "user", "")

	msgs := buf.GetMessages(12345, 1)
	if msgs != "" {
		t.Errorf("expected no messages for empty text, got '%s'", msgs)
	}
}

func TestSummaryBuffer_MultipleTopics(t *testing.T) {
	buf := NewSummaryBuffer(nil, 10)

	buf.Push(12345, 1, 301, "user1", "topic1 msg")
	buf.Push(12345, 2, 302, "user2", "topic2 msg")

	msgs1 := buf.GetMessages(12345, 1)
	msgs2 := buf.GetMessages(12345, 2)

	if msgs1 == "" {
		t.Error("expected messages for topic 1")
	}
	if msgs2 == "" {
		t.Error("expected messages for topic 2")
	}
}

func TestRingBuffer_All(t *testing.T) {
	rb := newRingBuffer(3)

	rb.push(bufferedMessage{Text: "a"})
	rb.push(bufferedMessage{Text: "b"})
	rb.push(bufferedMessage{Text: "c"})
	rb.push(bufferedMessage{Text: "d"})

	msgs := rb.all()
	if len(msgs) != 3 {
		t.Errorf("expected 3 messages, got %d", len(msgs))
	}
}

func TestRingBuffer_NotFull(t *testing.T) {
	rb := newRingBuffer(10)

	rb.push(bufferedMessage{Text: "a"})

	msgs := rb.all()
	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}
}

func TestSummaryBuffer_RemoveMessages(t *testing.T) {
	buf := NewSummaryBuffer(nil, 5)

	buf.Push(12345, 1, 101, "user1", "hello")
	buf.Push(12345, 1, 102, "user2", "world")
	buf.Push(12345, 1, 103, "user3", "testing")

	buf.RemoveMessages(12345, 1, []int{102})

	msgs := buf.GetMessages(12345, 1)
	if !strings.Contains(msgs, "hello") || !strings.Contains(msgs, "testing") {
		t.Error("expected 'hello' and 'testing' to remain in buffer")
	}
	if strings.Contains(msgs, "world") {
		t.Error("expected 'world' (id 102) to be deleted from buffer")
	}
}
