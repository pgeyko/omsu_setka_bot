package llm

import (
	"testing"
	"time"
)

func TestProvider_HasCapability(t *testing.T) {
	p := &Provider{
		Name: "test",
		Capabilities: []Capability{CapabilityMultimodal},
	}

	if !p.HasCapability(CapabilityMultimodal) {
		t.Error("expected multimodal capability")
	}
}

func TestProvider_IsActive(t *testing.T) {
	p := &Provider{state: &providerState{}}

	if !p.IsActive() {
		t.Error("expected provider to be active initially")
	}

	p.state.disabled = true
	p.state.disabledAt = time.Now()
	if p.IsActive() {
		t.Error("expected provider to be inactive when disabled")
	}
}

func TestChain_Pick(t *testing.T) {
	providers := []*Provider{
		{Name: "p1", state: &providerState{}},
		{Name: "p2", Capabilities: []Capability{CapabilityMultimodal}, state: &providerState{}},
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
	p := &Provider{state: &providerState{}}

	for i := 0; i < 3; i++ {
		p.RecordFailure()
	}

	if !p.state.disabled {
		t.Error("expected provider to be disabled after 3 failures")
	}
}

func TestCircuitBreaker_RecordSuccessResets(t *testing.T) {
	p := &Provider{state: &providerState{}}

	p.RecordFailure()
	p.RecordFailure()
	p.RecordSuccess()

	if p.state.failures != 0 {
		t.Errorf("expected failures to be 0, got %d", p.state.failures)
	}
}
