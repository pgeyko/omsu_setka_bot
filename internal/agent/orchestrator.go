package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"omsu_bot/internal/llm"
	"omsu_bot/internal/permissions"
	"omsu_bot/internal/skills"
)

const maxKBSize = 8 * 1024

type AdminChecker interface {
	IsAdmin(ctx context.Context, chatID int64, userID int64) bool
	IsOwner(ctx context.Context, chatID int64, userID int64) bool
}

type ToolExecutorInterface interface {
	SetMessageContext(sourceMessageID, replyToMessageID int)
	Execute(ctx context.Context, chatID int64, name string, arguments string) (string, error)
}

type AgentOrchestrator struct {
	llmClient      llm.LLMClient
	executor       ToolExecutorInterface
	adminChecker   AdminChecker
	permService    *permissions.Service
	skillsRegistry *skills.Registry
	featuresCache  sync.Map // key=chatID, value=featuresCacheEntry
	kbCache        sync.Map // key=chatID, value=kbCacheEntry
}

type featuresCacheEntry struct {
	data      []llm.Tool
	expiresAt time.Time
}

type kbCacheEntry struct {
	data      []byte
	expiresAt time.Time
}

func NewAgentOrchestrator(llmClient llm.LLMClient, executor ToolExecutorInterface, adminChecker AdminChecker, permService *permissions.Service, skillsRegistry *skills.Registry) *AgentOrchestrator {
	return &AgentOrchestrator{
		llmClient:      llmClient,
		executor:       executor,
		adminChecker:   adminChecker,
		permService:    permService,
		skillsRegistry: skillsRegistry,
	}
}

var restrictedTools = map[string]bool{
	"moderate_user": true,
	"run_protocol":  true,
	"manage_topic":  true,
}

func (ao *AgentOrchestrator) Run(ctx context.Context, chatID int64, threadID int, query string, username string, userID int64) (string, error) {
	return ao.RunWithContext(ctx, chatID, threadID, query, username, userID, 0, 0)
}

// RunWithContext executes the agent loop for a given user query with full message context.
// sourceMessageID is the ID of the message containing the mention (for forwarding).
// replyToMessageID is the ID of the replied-to message (if the mention was a reply).
func (ao *AgentOrchestrator) RunWithContext(ctx context.Context, chatID int64, threadID int, query string, username string, userID int64, sourceMessageID int, replyToMessageID int) (string, error) {
	// 1. Load group features and determine enabled tools
	isAuthorized := ao.adminChecker != nil && (ao.adminChecker.IsAdmin(ctx, chatID, userID) || ao.adminChecker.IsOwner(ctx, chatID, userID))

	var enabledTools []llm.Tool
	if ao.skillsRegistry != nil {
		features := ao.loadFeatures(chatID)
		enabledTools = ao.skillsRegistry.GetTools(chatID, isAuthorized, features)
	} else {
		enabledTools = ao.loadEnabledTools(chatID)
	}

	// Filter restricted tools based on DB command_permissions (P1#7)
	var filtered []llm.Tool
	for _, t := range enabledTools {
		if restrictedTools[t.Name] && !isAuthorized {
			continue
		}
		if ao.permService != nil && ao.permService.IsAdminOnly(ctx, chatID, t.Name) && !isAuthorized {
			continue
		}
		filtered = append(filtered, t)
	}
	enabledTools = filtered

	// Inject available protocol names into run_protocol description
	for i := range enabledTools {
		if enabledTools[i].Name == "run_protocol" {
			enabledTools[i] = injectProtocolNames(enabledTools[i])
			break
		}
	}

	// 2. Prepare history
	systemExtra := fmt.Sprintf("Текущее время: %s\nПользователь: @%s\nID топика: %d\nID чата: %d",
		time.Now().Format("2006-01-02 15:04:05 Mon"),
		username,
		threadID,
		chatID,
	)
	if sourceMessageID != 0 {
		systemExtra += fmt.Sprintf("\nID текущего сообщения: %d", sourceMessageID)
	}
	if replyToMessageID != 0 {
		systemExtra += fmt.Sprintf("\nID сообщения, на которое ответили (для пересылки): %d", replyToMessageID)
	}

	// Set message context on executor so forward_message can access message IDs
	ao.executor.SetMessageContext(sourceMessageID, replyToMessageID)

	// Add knowledge base content if it exists
	var kbBytes []byte
	if entry, ok := ao.kbCache.Load(chatID); ok {
		if cached := entry.(kbCacheEntry); time.Now().Before(cached.expiresAt) {
			kbBytes = cached.data
		}
	}
	if kbBytes == nil {
		kbPath := fmt.Sprintf("data/groups/%d/knowledge_base.md", chatID)
		if data, err := os.ReadFile(kbPath); err == nil && len(data) > 0 {
			kbBytes = data
		}
		ao.kbCache.Store(chatID, kbCacheEntry{
			data:      kbBytes,
			expiresAt: time.Now().Add(30 * time.Second),
		})
	}
	if len(kbBytes) > 0 {
		kb := kbBytes
		if len(kb) > maxKBSize {
			kb = kb[:maxKBSize]
		}
		systemExtra += "\n\nБаза знаний группы:\n" + string(kb)
	}

	if len(query) > 2000 {
		query = query[:2000]
	}

	history := []llm.AgentMessage{
		{
			Role:    "user",
			Content: query,
		},
	}

	type completedStep struct {
		Tool   string `json:"tool"`
		Status string `json:"status"`
		Result string `json:"result"`
	}
	var completedSteps []completedStep
	baseSystemExtra := systemExtra

	// 3. Loop up to N times
	maxSteps := 3
	for step := 0; step < maxSteps; step++ {
		// Reset systemExtra each step — only inject completed context, don't accumulate
		stepExtra := baseSystemExtra
		if len(completedSteps) > 0 {
			ctxJSON, _ := json.Marshal(completedSteps)
			stepExtra += fmt.Sprintf("\n\nУже выполнено (НЕ делай это снова, НЕ закрывай темы без запроса): %s", string(ctxJSON))
		}

		var resp *llm.Response
		var err error

		// Retry LLM call up to 2 extra times when all providers are exhausted (transient outage)
		for attempt := 0; attempt < 3; attempt++ {
			resp, err = ao.llmClient.CallGroupHistory(ctx, chatID, "agent_loop", stepExtra, history, enabledTools, false)
			if err == nil {
				break
			}
			if attempt < 2 && strings.Contains(err.Error(), "all providers failed") {
				slog.Warn("agent llm call failed, retrying after delay", "attempt", attempt+1, "error", err)
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(time.Duration(attempt+1) * 3 * time.Second):
				}
				continue
			}
			return "", fmt.Errorf("llm call failed: %w", err)
		}
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
			if resp.Content != "" {
				return resp.Content, nil
			}
			// LLM returned empty content after tool execution — return last tool result
			if step > 0 {
				for i := len(history) - 1; i >= 0; i-- {
					if history[i].Role == "tool" && history[i].Content != "" {
						return history[i].Content, nil
					}
				}
			}
			return resp.Content, nil
		}

		// Execute tool calls — async for independent tools, sequential for dependent
		sideEffectTools := map[string]bool{"manage_topic": true, "moderate_user": true, "run_protocol": true}
		hasSideEffect := false
		for _, tc := range resp.ToolCalls {
			if sideEffectTools[tc.Function.Name] {
				hasSideEffect = true
				break
			}
		}

		if !hasSideEffect && len(resp.ToolCalls) > 1 {
			// Async: run independent tools in parallel
			type toolResult struct {
				idx  int
				id   string
				name string
				text string
			}
			ch := make(chan toolResult, len(resp.ToolCalls))
			for i, tc := range resp.ToolCalls {
				go func(i int, tc llm.ToolCall) {
					var tr string
					var execErr error
					if !ao.canExecuteTool(ctx, chatID, userID, tc.Function.Name) {
						tr = fmt.Sprintf("⛔ Инструмент «%s» доступен только администраторам группы.", tc.Function.Name)
					} else {
						tr, execErr = ao.executor.Execute(ctx, chatID, tc.Function.Name, tc.Function.Arguments)
						if execErr != nil {
							slog.Error("failed to execute tool", "tool", tc.Function.Name, "error", execErr)
							tr = fmt.Sprintf("Ошибка при выполнении инструмента: %v", execErr)
						}
					}
					ch <- toolResult{idx: i, id: tc.ID, name: tc.Function.Name, text: tr}
				}(i, tc)
			}
			results := make([]toolResult, len(resp.ToolCalls))
			for range resp.ToolCalls {
				r := <-ch
				results[r.idx] = r
			}
			for _, r := range results {
				history = append(history, llm.AgentMessage{Role: "tool", Content: r.text, ToolCallID: r.id, ToolName: r.name})
				completedSteps = append(completedSteps, completedStep{Tool: r.name, Status: "done", Result: fmt.Sprintf("%.200s", r.text)})
			}
		} else {
			// Sequential: respect dependencies
			for _, tc := range resp.ToolCalls {
				var toolResult string
				var execErr error
				if !ao.canExecuteTool(ctx, chatID, userID, tc.Function.Name) {
					toolResult = fmt.Sprintf("⛔ Инструмент «%s» доступен только администраторам группы.", tc.Function.Name)
					slog.Warn("non-admin tried to use restricted tool", "tool", tc.Function.Name, "user_id", userID, "chat_id", chatID)
				} else {
					toolResult, execErr = ao.executor.Execute(ctx, chatID, tc.Function.Name, tc.Function.Arguments)
					if execErr != nil {
						slog.Error("failed to execute tool", "tool", tc.Function.Name, "error", execErr)
						toolResult = fmt.Sprintf("Ошибка при выполнении инструмента: %v", execErr)
					}
				}
				history = append(history, llm.AgentMessage{Role: "tool", Content: toolResult, ToolCallID: tc.ID, ToolName: tc.Function.Name})
				completedSteps = append(completedSteps, completedStep{Tool: tc.Function.Name, Status: "done", Result: fmt.Sprintf("%.200s", toolResult)})
			}
		}
	}

	return "", fmt.Errorf("agent loop exceeded max steps")
}

func (ao *AgentOrchestrator) loadEnabledTools(chatID int64) []llm.Tool {
	if entry, ok := ao.featuresCache.Load(chatID); ok {
		if cached := entry.(featuresCacheEntry); time.Now().Before(cached.expiresAt) {
			return cached.data
		}
	}

	featuresPath := fmt.Sprintf("data/groups/%d/features.json", chatID)
	bytes, err := os.ReadFile(featuresPath)
	if err != nil {
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

	ao.featuresCache.Store(chatID, featuresCacheEntry{
		data:      filtered,
		expiresAt: time.Now().Add(30 * time.Second),
	})
	return filtered
}

func (ao *AgentOrchestrator) loadFeatures(chatID int64) map[string]bool {
	featuresPath := fmt.Sprintf("data/groups/%d/features.json", chatID)
	bytes, err := os.ReadFile(featuresPath)
	if err != nil {
		return nil
	}
	var features map[string]bool
	if err := json.Unmarshal(bytes, &features); err != nil {
		return nil
	}
	return features
}

var (
	protocolsMu     sync.RWMutex
	protocolsData   []string
	protocolsLoaded bool
)

func loadProtocolNames() []string {
	protocolsMu.RLock()
	loaded := protocolsLoaded
	protocolsMu.RUnlock()
	if loaded {
		protocolsMu.RLock()
		defer protocolsMu.RUnlock()
		return protocolsData
	}
	return reloadProtocolNames()
}

func ReloadProtocols() {
	reloadProtocolNames()
}

func reloadProtocolNames() []string {
	content, err := os.ReadFile("protocols.json")
	if err != nil {
		slog.Warn("failed to read protocols.json for tool description", "error", err)
		return nil
	}
	var cfg struct {
		Protocols []struct {
			Name string `json:"name"`
		} `json:"protocols"`
	}
	if err := json.Unmarshal(content, &cfg); err != nil {
		slog.Warn("failed to parse protocols.json", "error", err)
		return nil
	}
	var names []string
	for _, p := range cfg.Protocols {
		names = append(names, p.Name)
	}
	protocolsMu.Lock()
	protocolsData = names
	protocolsLoaded = true
	protocolsMu.Unlock()
	return names
}

func injectProtocolNames(t llm.Tool) llm.Tool {
	names := loadProtocolNames()
	if len(names) == 0 {
		return t
	}
	desc := t.Description
	desc += fmt.Sprintf(" Доступные протоколы: %s", strings.Join(names, ", "))
	t.Description = desc
	return t
}

// canExecuteTool checks whether a user is allowed to execute a given tool.
// Uses DB command_permissions table first, then falls back to hardcoded restrictedTools.
func (ao *AgentOrchestrator) canExecuteTool(ctx context.Context, chatID int64, userID int64, toolName string) bool {
	isAdminOrOwner := ao.adminChecker != nil && (ao.adminChecker.IsAdmin(ctx, chatID, userID) || ao.adminChecker.IsOwner(ctx, chatID, userID))

	// Hardcoded restricted tools (legacy fallback)
	if restrictedTools[toolName] && !isAdminOrOwner {
		return false
	}

	// DB-backed permissions (P1#7)
	if ao.permService != nil && ao.permService.IsAdminOnly(ctx, chatID, toolName) && !isAdminOrOwner {
		return false
	}

	return true
}
