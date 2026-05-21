package classifier

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"omsu_bot/internal/llm"
)

type ClassifyResult struct {
	Topic      string   `json:"topic"`
	Hashtags   []string `json:"hashtags"`
	Confidence float64  `json:"confidence"`
}

type TopicsProvider interface {
	GetTopics(ctx context.Context, chatID int64) ([]TopicInfo, error)
}

type TopicInfo struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Classifier struct {
	llmClient *llm.Client
	prompts   *llm.PromptRegistry
	topics    TopicsProvider

	mu          sync.RWMutex
	visionCache map[string]ClassifyResult
}

func New(llmClient *llm.Client, prompts *llm.PromptRegistry, topics TopicsProvider) *Classifier {
	return &Classifier{
		llmClient:   llmClient,
		prompts:     prompts,
		topics:      topics,
		visionCache: make(map[string]ClassifyResult),
	}
}

func (c *Classifier) ClassifyMessage(ctx context.Context, chatID int64, text string, fileID string) (*ClassifyResult, error) {
	if fileID != "" {
		c.mu.RLock()
		if result, ok := c.visionCache[fileID]; ok {
			c.mu.RUnlock()
			return &result, nil
		}
		c.mu.RUnlock()
	}

	if c.topics == nil {
		return nil, fmt.Errorf("topics provider not configured")
	}

	prompt := c.prompts.Get("classify")

	topicList, err := c.topics.GetTopics(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get topics: %w", err)
	}

	prompt = c.fillPrompt(prompt, topicList, text)

	resp, err := c.llmClient.Call(ctx, "classify", "", prompt, false)
	if err != nil {
		return nil, fmt.Errorf("classification failed: %w", err)
	}

	var result ClassifyResult
	if err := json.Unmarshal([]byte(llm.ExtractJSON(resp.Content)), &result); err != nil {
		return nil, fmt.Errorf("failed to parse classification: %w", err)
	}

	if fileID != "" {
		c.mu.Lock()
		c.visionCache[fileID] = result
		c.mu.Unlock()
	}

	return &result, nil
}

func (c *Classifier) fillPrompt(template string, topics []TopicInfo, text string) string {
	var topicStr string
	for _, t := range topics {
		topicStr += fmt.Sprintf("- %s: %s\n", t.Name, t.Description)
	}

	result := template
	result = replacePlaceholder(result, "{topics}", topicStr)
	result = replacePlaceholder(result, "{text}", text)
	return result
}

func replacePlaceholder(s, placeholder, value string) string {
	idx := 0
	for {
		pos := indexOf(s, placeholder, idx)
		if pos == -1 {
			break
		}
		s = s[:pos] + value + s[pos+len(placeholder):]
		idx = pos + len(value)
	}
	return s
}

func indexOf(s, substr string, start int) int {
	if start >= len(s) {
		return -1
	}
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
