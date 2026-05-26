package llm

import (
	"testing"
	"time"

	"omsu_bot/internal/circuitbreaker"
)

func TestProvider_HasCapability(t *testing.T) {
	p := &Provider{
		Name:         "test",
		Capabilities: []Capability{CapabilityMultimodal},
	}

	if !p.HasCapability(CapabilityMultimodal) {
		t.Error("expected multimodal capability")
	}
}

func TestProvider_IsActive(t *testing.T) {
	p := &Provider{cb: circuitbreaker.New(3, 5*time.Minute)}

	if !p.IsActive() {
		t.Error("expected provider to be active initially")
	}

	p.cb.RecordFailure()
	p.cb.RecordFailure()
	p.cb.RecordFailure()
	if p.IsActive() {
		t.Error("expected provider to be inactive when disabled")
	}
}

func TestChain_Pick(t *testing.T) {
	providers := []*Provider{
		{Name: "p1"},
		{Name: "p2", Capabilities: []Capability{CapabilityMultimodal}},
	}

	chain := NewChain(providers)

	picked, err := chain.Pick()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if picked.Name != "p1" {
		t.Errorf("expected p1, got %s", picked.Name)
	}

	picked, err = chain.Pick(CapabilityMultimodal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if picked.Name != "p2" {
		t.Errorf("expected p2 (multimodal), got %s", picked.Name)
	}
}

func TestChain_Pick_NoProvider(t *testing.T) {
	chain := NewChain(nil)
	_, err := chain.Pick()
	if err == nil {
		t.Error("expected error when no providers")
	}
}

func TestCircuitBreaker_RecordsFailure(t *testing.T) {
	p := &Provider{cb: circuitbreaker.New(3, 5*time.Minute)}

	for i := 0; i < 3; i++ {
		p.RecordFailure()
	}

	if p.IsActive() {
		t.Error("expected provider to be inactive after 3 failures")
	}
}

func TestCircuitBreaker_RecordSuccessResets(t *testing.T) {
	p := &Provider{cb: circuitbreaker.New(3, 5*time.Minute)}

	p.RecordFailure()
	p.RecordFailure()
	p.RecordSuccess()

	// after RecordSuccess, failures should be 0; two more should not trip
	p.RecordFailure()
	p.RecordFailure()

	if !p.IsActive() {
		t.Error("expected provider to still be active after 2 failures following a reset")
	}
}
