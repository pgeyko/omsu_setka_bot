package handlers

import (
	"testing"
)

func TestMakeSlug(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Сессия", "sessiya"},
		{"Лекции и Практика", "lektsii_i_praktika"},
		{"ИВТ-101", "ivt_101"},
		{"test", "test"},
	}

	for _, tt := range tests {
		result := makeSlug(tt.input)
		if result != tt.expected {
			t.Errorf("makeSlug('%s') = '%s', expected '%s'", tt.input, result, tt.expected)
		}
	}
}
