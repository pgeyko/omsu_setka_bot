package circuitbreaker

import (
	"sync"
	"time"
)

type CircuitBreaker struct {
	mu         sync.Mutex
	threshold  int
	cooldown   time.Duration
	failures   int
	disabledAt time.Time
	disabled   bool
}

func New(threshold int, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		cooldown:  cooldown,
	}
}

func (cb *CircuitBreaker) IsActive() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !cb.disabled {
		return true
	}
	if time.Since(cb.disabledAt) >= cb.cooldown {
		cb.disabled = false
		cb.failures = 0
		return true
	}
	return false
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	if cb.failures >= cb.threshold {
		cb.disabled = true
		cb.disabledAt = time.Now()
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
}

func (cb *CircuitBreaker) Failures() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failures
}
