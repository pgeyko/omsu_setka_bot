package llm

import (
	"testing"
)

func TestTracker_IsLimitReached(t *testing.T) {
	tracker := NewTracker(nil, 100, 0.8)

	if tracker.IsLimitReached() {
		t.Error("expected limit not reached initially")
	}

	tracker.dailyTokens.Store(100)

	if !tracker.IsLimitReached() {
		t.Error("expected limit to be reached")
	}
}

func TestTracker_IsLimitReached_ZeroLimit(t *testing.T) {
	tracker := NewTracker(nil, 0, 0.8)

	if tracker.IsLimitReached() {
		t.Error("expected limit not reached with zero limit")
	}
}
