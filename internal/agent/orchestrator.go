package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"omsu_bot/internal/llm"
)

type AdminChecker interface {
	IsAdmin(ctx context.Context, chatID int64, userID int64) bool
	IsOwner(ctx context.Context, chatID int64, userID int64) bool
}

type AgentOrchestrator struct {
	llmClient    *llm.Client
	executor     *ToolExecutor
	adminChecker AdminChecker
}

func NewAgentOrchestrator(llmClient *llm.Client, executor *ToolExecutor, adminChecker AdminChecker) *AgentOrchestrator {
	return &AgentOrchestrator{
		llmClient:    llmClient,
		executor:     executor,
		adminChecker: adminChecker,
	}
}

var restrictedTools = map[string]bool{
	"moderate_user": true,
	"run_protocol":  true,
	"manage_topic":  true,
}

// Run executes the agent loop for a given user query
func (ao *AgentOrchestrator) Run(ctx context.Context, chatID int64, threadID int, query string, username string, userID int64) (string, error) {
	// 1. Load group features
	enabledTools := ao.loadEnabledTools(chatID)

	// 2. Prepare history
	// Build systemExtra containing today's date, the current user's username, etc.
	systemExtra := fmt.Sprintf("Текущее время: %s\nПользователь, к которому ты обращаешься: @%s\nID текущего топика: %d\nID текущего чата: %d",
		time.Now().Format("2006-01-02 15:04:05 Mon"),
		username,
		threadID,
		chatID,
	)

	// Add knowledge base content if it exists
	kbPath := fmt.Sprintf("data/groups/%d/knowledge_base.txt", chatID)
	if kbBytes, err := os.ReadFile(kbPath); err == nil && len(kbBytes) > 0 {
		systemExtra += "\n\nБаза знаний группы:\n" + string(kbBytes)
	}

	history := []llm.AgentMessage{
		{
			Role:    "user",
			Content: query,
		},
	}

	// 3. Loop up to N times
	maxSteps := 5
	for step := 0; step < maxSteps; step++ {
		resp, err := ao.llmClient.CallGroupHistory(ctx, chatID, "agent_loop", systemExtra, history, enabledTools, false)
		if err != nil {
			return "", fmt.Errorf("llm call failed: %w", err)
		}

		// Append assistant response to history
		assistantMsg := llm.AgentMessage{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		history = append(history, assistantMsg)

		if len(resp.ToolCalls) == 0 {
			// No more tool calls, return the final text response
			return resp.Content, nil
		}

		// Execute each tool call
		for _, tc := range resp.ToolCalls {
			var toolResult string
			var err error

			if restrictedTools[tc.Function.Name] && ao.adminChecker != nil && !ao.adminChecker.IsAdmin(ctx, chatID, userID) {
				toolResult = fmt.Sprintf("⛔ Инструмент «%s» доступен только администраторам группы.", tc.Function.Name)
				slog.Warn("non-admin tried to use restricted tool", "tool", tc.Function.Name, "user_id", userID, "chat_id", chatID)
			} else {
				toolResult, err = ao.executor.Execute(ctx, chatID, tc.Function.Name, tc.Function.Arguments)
				if err != nil {
					slog.Error("failed to execute tool", "tool", tc.Function.Name, "error", err)
					toolResult = fmt.Sprintf("Ошибка при выполнении инструмента: %v", err)
				}
			}

			// Append tool output to history
			history = append(history, llm.AgentMessage{
				Role:       "tool",
				Content:    toolResult,
				ToolCallID: tc.ID,
				ToolName:   tc.Function.Name,
			})
		}
	}

	return "", fmt.Errorf("agent loop exceeded max steps")
}

func (ao *AgentOrchestrator) loadEnabledTools(chatID int64) []llm.Tool {
	featuresPath := fmt.Sprintf("data/groups/%d/features.json", chatID)
	bytes, err := os.ReadFile(featuresPath)
	if err != nil {
		// File does not exist, return all available tools
		return AvailableTools
	}

	var features map[string]bool
	if err := json.Unmarshal(bytes, &features); err != nil {
		slog.Error("failed to parse features.json", "chat_id", chatID, "error", err)
		return AvailableTools
	}

	var filtered []llm.Tool
	for _, t := range AvailableTools {
		if enabled, exists := features[t.Name]; !exists || enabled {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
