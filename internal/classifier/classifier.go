package classifier

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

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

type visionCacheEntry struct {
	result   ClassifyResult
	cachedAt time.Time
}

type Classifier struct {
	llmClient *llm.Client
	prompts   *llm.PromptRegistry
	topics    TopicsProvider

	mu          sync.RWMutex
	visionCache map[string]visionCacheEntry
	maxCache    int
	cacheTTL    time.Duration
}

func New(llmClient *llm.Client, prompts *llm.PromptRegistry, topics TopicsProvider) *Classifier {
	return &Classifier{
		llmClient:   llmClient,
		prompts:     prompts,
		topics:      topics,
		visionCache: make(map[string]visionCacheEntry),
		maxCache:    200,
		cacheTTL:    1 * time.Hour,
	}
}

func (c *Classifier) ClassifyMessage(ctx context.Context, chatID int64, text string, fileID string) (*ClassifyResult, error) {
	return c.ClassifyWithImage(ctx, chatID, text, fileID, nil, "")
}

func (c *Classifier) ClassifyWithImage(ctx context.Context, chatID int64, text string, fileID string, imageData []byte, imageMime string) (*ClassifyResult, error) {
	if fileID != "" {
		c.mu.RLock()
		if entry, ok := c.visionCache[fileID]; ok && time.Since(entry.cachedAt) < c.cacheTTL {
			c.mu.RUnlock()
			return &entry.result, nil
		}
		c.mu.RUnlock()
	}

	if c.topics == nil {
		return nil, fmt.Errorf("topics provider not configured")
	}

	topicList, err := c.topics.GetTopics(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get topics: %w", err)
	}

	userPrompt := c.fillPrompt(c.prompts.Get("classify"), topicList, text)
	systemPrompt := "Ты — классификатор сообщений студенческой группы. Отвечай ТОЛЬКО JSON без пояснений."

	var resp *llm.Response
	if len(imageData) > 0 {
		history := []llm.AgentMessage{{
			Role:    "user",
			Content: userPrompt,
			MediaParts: []llm.MediaPart{{
				MimeType: imageMime,
				Data:     imageData,
			}},
		}}
		resp, err = c.llmClient.CallGroupHistory(ctx, chatID, "classify", systemPrompt, history, nil, true)
	} else {
		resp, err = c.llmClient.CallWithSystemPrompt(ctx, "classify", systemPrompt, userPrompt)
	}
	if err != nil {
		return nil, fmt.Errorf("classification failed: %w", err)
	}

	var result ClassifyResult
	if err := json.Unmarshal([]byte(llm.ExtractJSON(resp.Content)), &result); err != nil {
		// Retry once with explicit JSON instruction
		retryPrompt := userPrompt + "\n\nОТВЕТЬ ТОЛЬКО JSON. Никакого текста, только фигурные скобки."
		resp2, err2 := c.llmClient.CallWithSystemPrompt(ctx, "classify", systemPrompt, retryPrompt)
		if err2 != nil {
			return nil, fmt.Errorf("classification failed after retry: %w", err2)
		}
		if err := json.Unmarshal([]byte(llm.ExtractJSON(resp2.Content)), &result); err != nil {
			return nil, fmt.Errorf("failed to parse classification: %w (content: %s)", err, truncateR(resp2.Content, 200))
		}
	}

	if fileID != "" {
		c.mu.Lock()
		if len(c.visionCache) >= c.maxCache {
			var oldest string
			var oldestTime time.Time
			for k, v := range c.visionCache {
				if oldest == "" || v.cachedAt.Before(oldestTime) {
					oldest = k
					oldestTime = v.cachedAt
				}
			}
			delete(c.visionCache, oldest)
		}
		c.visionCache[fileID] = visionCacheEntry{result: result, cachedAt: time.Now()}
		c.mu.Unlock()
	}

	return &result, nil
}

func truncateR(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
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
