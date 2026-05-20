package schedule

import (
	"context"
	"encoding/json"
	"fmt"
)

type Announcer struct {
	client  LLMClient
	prompts PromptLoader
}

type PromptLoader interface {
	Get(name string) string
}

func NewAnnouncer(client LLMClient, prompts PromptLoader) *Announcer {
	return &Announcer{client: client, prompts: prompts}
}

func (a *Announcer) GenerateAnnouncement(ctx context.Context, anomalies []Anomaly, changes []Change) (string, error) {
	prompt := a.prompts.Get("schedule_announce")

	changesJSON, _ := json.Marshal(changes)
	anomaliesJSON, _ := json.Marshal(anomalies)

	filledPrompt := fmt.Sprintf("%s\n\nИзменения: %s\n\nАномалии: %s", prompt, string(changesJSON), string(anomaliesJSON))

	resp, err := a.client.Call(ctx, "schedule_announce", "", filledPrompt, false)
	if err != nil {
		return "", fmt.Errorf("llm announcement failed: %w", err)
	}

	return resp.Content, nil
}
