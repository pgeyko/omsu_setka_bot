package circuitbreaker

import (
	"testing"
	"time"
)

func TestCircuitBreaker_InitialActive(t *testing.T) {
	cb := New(3, 5*time.Minute)
	if !cb.IsActive() {
		t.Error("expected circuit breaker to be active initially")
	}
}

func TestCircuitBreaker_TripsAfterThreshold(t *testing.T) {
	cb := New(3, 5*time.Minute)

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.IsActive() {
		t.Error("expected circuit breaker to be inactive after threshold failures")
	}
}

func TestCircuitBreaker_StaysActiveBelowThreshold(t *testing.T) {
	cb := New(3, 5*time.Minute)

	cb.RecordFailure()
	cb.RecordFailure()

	if !cb.IsActive() {
		t.Error("expected circuit breaker to stay active below threshold")
	}
}

func TestCircuitBreaker_RecordSuccessResetsFailures(t *testing.T) {
	cb := New(3, 5*time.Minute)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()

	// two more failures should not trip: success reset the counter
	cb.RecordFailure()
	cb.RecordFailure()

	if !cb.IsActive() {
		t.Error("expected circuit breaker to still be active after 2 failures following a reset")
	}
}

func TestCircuitBreaker_AutoResetAfterCooldown(t *testing.T) {
	cb := New(1, 50*time.Millisecond)

	cb.RecordFailure()

	if cb.IsActive() {
		t.Error("expected circuit breaker to be inactive immediately after failure")
	}

	time.Sleep(60 * time.Millisecond)

	if !cb.IsActive() {
		t.Error("expected circuit breaker to auto-reset after cooldown")
	}
}
