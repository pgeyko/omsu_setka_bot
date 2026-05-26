package llm

import (
	"fmt"
	"sync"
	"time"

	"omsu_bot/internal/circuitbreaker"
)

type Capability string

const (
	CapabilityMultimodal Capability = "multimodal"

	rateLimitWindow = 1 * time.Minute
	dayBoundary     = 24 * time.Hour
)

type rateEntry struct {
	at     time.Time
	tokens int
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
	Priority       int // lower = tried first when sorted
	cb             *circuitbreaker.CircuitBreaker

	// Rate limits
	RPMLimit int // requests per minute (0 = unlimited)
	TPMLimit int // tokens per minute (0 = unlimited)
	RPDLimit int // requests per day (0 = unlimited)
	TPDLimit int // tokens per day (0 = unlimited)

	rateHistory []rateEntry
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

	if p.cb != nil && !p.cb.IsActive() {
		return false
	}
	return !p.isRateLimitedLocked()
}

func (p *Provider) isRateLimitedLocked() bool {
	now := time.Now()
	cutoff := now.Add(-rateLimitWindow)
	dayCutoff := now.Truncate(dayBoundary)

	// Prune old entries and count
	var minuteCount int
	var minuteTokens int
	var dayCount int
	valid := p.rateHistory[:0]
	for _, e := range p.rateHistory {
		if e.at.Before(dayCutoff) {
			continue // older than today
		}
		valid = append(valid, e)
		if e.at.After(cutoff) {
			minuteCount++
			minuteTokens += e.tokens
		}
		dayCount++
	}
	p.rateHistory = valid

	if p.RPMLimit > 0 && minuteCount >= p.RPMLimit {
		return true
	}
	if p.TPMLimit > 0 && minuteTokens >= p.TPMLimit {
		return true
	}
	if p.RPDLimit > 0 && dayCount >= p.RPDLimit {
		return true
	}
	return false
}

func (p *Provider) recordCall(tokens int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	dayCutoff := now.Truncate(dayBoundary)
	cutoff := now.Add(-rateLimitWindow)
	valid := p.rateHistory[:0]
	for _, e := range p.rateHistory {
		if e.at.After(cutoff) {
			valid = append(valid, e)
		}
	}
	valid = append(valid, rateEntry{at: now, tokens: tokens})
	if len(valid) > 0 {
		first := valid[0].at
		if first.Before(dayCutoff) {
			_ = first
		}
	}
	p.rateHistory = valid
}

func (p *Provider) RecordFailure() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cb == nil {
		p.cb = circuitbreaker.New(3, 5*time.Minute)
	}
	p.cb.RecordFailure()
}

func (p *Provider) RecordSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cb != nil {
		p.cb.RecordSuccess()
	}
}

type Chain struct {
	providers []*Provider
	mu        sync.RWMutex
}

func NewChain(providers []*Provider) *Chain {
	for _, p := range providers {
		p.cb = circuitbreaker.New(3, 5*time.Minute)
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
