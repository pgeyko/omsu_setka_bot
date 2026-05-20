package handlers

import (
	"testing"
)

func TestCountWords(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 0},
		{"hello world", 0},
		{"a b c d e f g h i j k l m n o", 15},
		{"one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen", 15},
	}

	for _, tt := range tests {
		got := countWords(tt.input)
		if got != tt.want {
			t.Errorf("countWords(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
