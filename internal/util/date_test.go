package util

import (
	"testing"
	"time"
)

func TestResolveDate(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	tests := []struct {
		name         string
		date         string
		relativeDate string
		want         string
	}{
		{"explicit date", "2026-01-15", "", "2026-01-15"},
		{"explicit over relative", "2026-01-15", "today", "2026-01-15"},
		{"today", "", "today", today},
		{"tomorrow", "", "tomorrow", tomorrow},
		{"empty relative", "", "", ""},
		{"invalid date with empty relative returns empty", "not-a-date", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveDate(tt.date, tt.relativeDate); got != tt.want {
				t.Errorf("ResolveDate(%q, %q) = %q, want %q", tt.date, tt.relativeDate, got, tt.want)
			}
		})
	}
}

func TestResolveDate_Weekday(t *testing.T) {
	now := time.Now()
	days := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
	russian := []string{"воскресенье", "понедельник", "вторник", "среда", "четверг", "пятница", "суббота"}
	for i, day := range days {
		diff := (i - int(now.Weekday()) + 7) % 7
		expected := now.AddDate(0, 0, diff).Format("2006-01-02")
		if got := ResolveDate("", day); got != expected {
			t.Errorf("ResolveDate('', %q) = %q, want %q", day, got, expected)
		}
		if got := ResolveDate("", russian[i]); got != expected {
			t.Errorf("ResolveDate('', %q) = %q, want %q", russian[i], got, expected)
		}
	}
}
