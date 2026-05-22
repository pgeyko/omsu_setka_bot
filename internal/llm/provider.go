package llm

import (
	"fmt"
	"sync"
	"time"
)

type Capability string

const (
	CapabilityMultimodal Capability = "multimodal"
)

type providerState struct {
	failures   int
	disabledAt time.Time
	disabled   bool
}

type Provider struct {
	mu             sync.Mutex
	Name           string
	Type           string // "gemini" or "deepseek" or "openai"
	BaseURL        string
	APIKey         string
	Model          string   // primary model
	FallbackModels []string // tried after primary on same key (for rate limits)
	Capabilities   []Capability
	state          *providerState
}

func (p *Provider) HasCapability(c Capability) bool {
	for _, cap := range p.Capabilities {
		if cap == c {
			return true
		}
	}
	return false
}

func (p *Provider) IsActive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == nil {
		return true
	}
	if !p.state.disabled {
		return true
	}
	if time.Since(p.state.disabledAt) >= 5*time.Minute {
		p.state.disabled = false
		p.state.failures = 0
		return true
	}
	return false
}

func (p *Provider) RecordFailure() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == nil {
		p.state = &providerState{}
	}
	p.state.failures++
	if p.state.failures >= 3 {
		p.state.disabled = true
		p.state.disabledAt = time.Now()
	}
}

func (p *Provider) RecordSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != nil {
		p.state.failures = 0
	}
}

type Chain struct {
	providers []*Provider
	mu        sync.RWMutex
}

func NewChain(providers []*Provider) *Chain {
	for _, p := range providers {
		p.state = &providerState{}
	}
	return &Chain{providers: providers}
}

func (c *Chain) Providers() []*Provider {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*Provider, len(c.providers))
	copy(result, c.providers)
	return result
}

func (c *Chain) Pick(required ...Capability) (*Provider, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, p := range c.providers {
		if !p.IsActive() {
			continue
		}
		if len(required) == 0 {
			return p, nil
		}
		hasAll := true
		for _, cap := range required {
			if !p.HasCapability(cap) {
				hasAll = false
				break
			}
		}
		if hasAll {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no active provider available")
}
