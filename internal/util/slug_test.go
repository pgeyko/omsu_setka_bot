package util

import (
	"testing"
)

func TestMakeSlug(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Привет мир", "privet_mir"},
		{"ИВТ-101", "ivt_101"},
		{"hello world", "hello_world"},
		{"Test.Dot", "testdot"},
		{"O'Brien", "obrien"},
		{"", ""},
		{"123", "123"},
		{"A.B-C'D`E\"F", "ab_cdef"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MakeSlug(tt.name); got != tt.want {
				t.Errorf("MakeSlug(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestChatLink(t *testing.T) {
	tests := []struct {
		name   string
		chatID int64
		msgID  int
		want   string
	}{
		{"supergroup", -1001234567890, 42, "https://t.me/c/1234567890/42"},
		{"positive chat", 12345, 1, "https://t.me/c/12345/1"},
		{"short supergroup", -100123, 99, "https://t.me/c/123/99"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ChatLink(tt.chatID, tt.msgID); got != tt.want {
				t.Errorf("ChatLink(%d, %d) = %q, want %q", tt.chatID, tt.msgID, got, tt.want)
			}
		})
	}
}
