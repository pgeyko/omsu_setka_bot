package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"omsu_bot/internal/llm"
)

type mockLLMClient struct {
	callFunc func(ctx context.Context, reqType, systemExtra, userPrompt string, requiresVision bool) (*llm.Response, error)
	callGroupHistoryFunc func(ctx context.Context, chatID int64, reqType, systemExtra string, history []llm.AgentMessage, tools []llm.Tool, requiresVision bool) (*llm.Response, error)
}

func (m *mockLLMClient) Call(ctx context.Context, reqType, systemExtra, userPrompt string, requiresVision bool) (*llm.Response, error) {
	if m.callFunc != nil {
		return m.callFunc(ctx, reqType, systemExtra, userPrompt, requiresVision)
	}
	return &llm.Response{Content: "mock response"}, nil
}

func (m *mockLLMClient) CallGroupHistory(ctx context.Context, chatID int64, reqType, systemExtra string, history []llm.AgentMessage, tools []llm.Tool, requiresVision bool) (*llm.Response, error) {
	if m.callGroupHistoryFunc != nil {
		return m.callGroupHistoryFunc(ctx, chatID, reqType, systemExtra, history, tools, requiresVision)
	}
	return &llm.Response{Content: "mock response"}, nil
}

func (m *mockLLMClient) CallWithSystemHistory(ctx context.Context, chatID int64, reqType, systemPrompt string, history []llm.AgentMessage, requiresVision bool) (*llm.Response, error) {
	return &llm.Response{Content: "mock response"}, nil
}

func (m *mockLLMClient) CallWithSystemPrompt(ctx context.Context, reqType, systemPrompt, userPrompt string) (*llm.Response, error) {
	return &llm.Response{Content: "mock response"}, nil
}

func (m *mockLLMClient) HasMultimodalProvider() bool { return false }
func (m *mockLLMClient) SetSkipFallbackModel(skip bool) {}

type mockAdminChecker struct {
	isAdmin bool
	isOwner bool
}

func (m *mockAdminChecker) IsAdmin(ctx context.Context, chatID int64, userID int64) bool {
	return m.isAdmin
}

func (m *mockAdminChecker) IsOwner(ctx context.Context, chatID int64, userID int64) bool {
	return m.isOwner
}

type mockToolExecutor struct {
	executeFunc func(ctx context.Context, chatID int64, name string, arguments string) (string, error)
}

func (m *mockToolExecutor) SetMessageContext(sourceMessageID, replyToMessageID int) {}
func (m *mockToolExecutor) Execute(ctx context.Context, chatID int64, name string, arguments string) (string, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, chatID, name, arguments)
	}
	return "executed", nil
}

func setupOrchestratorTest(t *testing.T, chatID int64) (*AgentOrchestrator, string, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)

	// Create minimal features.json so loadEnabledTools returns default tools
	features := map[string]bool{
		"generate_summary": true,
		"get_schedule":     true,
		"moderate_user":    false,
	}
	featuresDir := filepath.Join(tmpDir, "data", "groups", "123")
	os.MkdirAll(featuresDir, 0755)
	fb, _ := json.Marshal(features)
	os.WriteFile(filepath.Join(featuresDir, "features.json"), fb, 0644)

	ao := NewAgentOrchestrator(
		&mockLLMClient{},
		&mockToolExecutor{},
		&mockAdminChecker{isAdmin: true},
		nil,
	)

	cleanup := func() { os.Chdir(origWd) }
	return ao, tmpDir, cleanup
}

func TestAgentOrchestrator_NoToolCall_ReturnsContent(t *testing.T) {
	t.Parallel()
	ao, _, cleanup := setupOrchestratorTest(t, 123)
	defer cleanup()

	// LLM returns a text response with no tool calls
	ao.llmClient = &mockLLMClient{
		callGroupHistoryFunc: func(ctx context.Context, chatID int64, reqType, systemExtra string, history []llm.AgentMessage, tools []llm.Tool, requiresVision bool) (*llm.Response, error) {
			return &llm.Response{Content: "Привет! Чем могу помочь?"}, nil
		},
	}

	result, err := ao.Run(context.Background(), 123, 1, "привет", "user1", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Привет! Чем могу помочь?" {
		t.Errorf("expected 'Привет! Чем могу помочь?', got '%s'", result)
	}
}

func TestAgentOrchestrator_ToolExecution_ReturnsToolResult(t *testing.T) {
	t.Parallel()
	ao, _, cleanup := setupOrchestratorTest(t, 123)
	defer cleanup()

	callCount := 0
	ao.llmClient = &mockLLMClient{
		callGroupHistoryFunc: func(ctx context.Context, chatID int64, reqType, systemExtra string, history []llm.AgentMessage, tools []llm.Tool, requiresVision bool) (*llm.Response, error) {
			callCount++
			if callCount == 1 {
				// First call: return a tool call
				return &llm.Response{
					Content: "",
					ToolCalls: []llm.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: llm.FunctionCall{
								Name:      "get_schedule",
								Arguments: `{"group":"test"}`,
							},
						},
					},
				}, nil
			}
			// Second call: return final text
			return &llm.Response{Content: "Расписание загружено"}, nil
		},
	}

	ao.executor = &mockToolExecutor{
		executeFunc: func(ctx context.Context, chatID int64, name string, arguments string) (string, error) {
			if name != "get_schedule" {
				t.Errorf("expected get_schedule tool, got %s", name)
			}
			return "schedule data", nil
		},
	}

	result, err := ao.Run(context.Background(), 123, 1, "расписание", "user1", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Расписание загружено" {
		t.Errorf("expected 'Расписание загружено', got '%s'", result)
	}
}

func TestAgentOrchestrator_ToolFiltering_NonAdminRestricted(t *testing.T) {
	t.Parallel()
	ao, _, cleanup := setupOrchestratorTest(t, 123)
	defer cleanup()

	ao.llmClient = &mockLLMClient{
		callGroupHistoryFunc: func(ctx context.Context, chatID int64, reqType, systemExtra string, history []llm.AgentMessage, tools []llm.Tool, requiresVision bool) (*llm.Response, error) {
			// Verify that restricted tools are not in the enabled list for non-admins
			for _, tl := range tools {
				if tl.Name == "moderate_user" {
					t.Error("moderate_user should be filtered out for non-admin users")
				}
			}
			return &llm.Response{Content: "ok"}, nil
		},
	}

	// Non-admin user
	ao.adminChecker = &mockAdminChecker{isAdmin: false}
	ao.Run(context.Background(), 123, 1, "test", "user1", 100)
}

func TestAgentOrchestrator_MaxSteps_ExitsAfterLimit(t *testing.T) {
	t.Parallel()
	ao, _, cleanup := setupOrchestratorTest(t, 123)
	defer cleanup()

	callCount := 0
	ao.llmClient = &mockLLMClient{
		callGroupHistoryFunc: func(ctx context.Context, chatID int64, reqType, systemExtra string, history []llm.AgentMessage, tools []llm.Tool, requiresVision bool) (*llm.Response, error) {
			callCount++
			// Keep returning tool calls to exhaust max steps
			return &llm.Response{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_x",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "get_schedule",
							Arguments: `{}`,
						},
					},
				},
			}, nil
		},
	}

	ao.executor = &mockToolExecutor{
		executeFunc: func(ctx context.Context, chatID int64, name string, arguments string) (string, error) {
			return "done", nil
		},
	}

	_, err := ao.Run(context.Background(), 123, 1, "loop", "user1", 100)
	if err == nil {
		t.Fatal("expected error for exceeding max steps")
	}
	if err.Error() != "agent loop exceeded max steps" {
		t.Errorf("expected 'agent loop exceeded max steps', got '%v'", err)
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 LLM calls, got %d", callCount)
	}
}

func TestAgentOrchestrator_AsyncDispatch_IndependentTools(t *testing.T) {
	t.Parallel()
	ao, _, cleanup := setupOrchestratorTest(t, 123)
	defer cleanup()

	callCount := 0
	ao.llmClient = &mockLLMClient{
		callGroupHistoryFunc: func(ctx context.Context, chatID int64, reqType, systemExtra string, history []llm.AgentMessage, tools []llm.Tool, requiresVision bool) (*llm.Response, error) {
			callCount++
			if callCount == 1 {
				// Return multiple independent tool calls
				return &llm.Response{
					ToolCalls: []llm.ToolCall{
						{ID: "c1", Type: "function", Function: llm.FunctionCall{Name: "get_schedule", Arguments: `{}`}},
						{ID: "c2", Type: "function", Function: llm.FunctionCall{Name: "generate_summary", Arguments: `{}`}},
					},
				}, nil
			}
			return &llm.Response{Content: "both done"}, nil
		},
	}

	ao.executor = &mockToolExecutor{
		executeFunc: func(ctx context.Context, chatID int64, name string, arguments string) (string, error) {
			return "result", nil
		},
	}

	result, err := ao.Run(context.Background(), 123, 1, "run tools", "user1", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "both done" {
		t.Errorf("expected 'both done', got '%s'", result)
	}
}

func TestAgentOrchestrator_ErrorPath_AllProvidersFailed(t *testing.T) {
	t.Parallel()
	ao, _, cleanup := setupOrchestratorTest(t, 123)
	defer cleanup()

	ao.llmClient = &mockLLMClient{
		callGroupHistoryFunc: func(ctx context.Context, chatID int64, reqType, systemExtra string, history []llm.AgentMessage, tools []llm.Tool, requiresVision bool) (*llm.Response, error) {
			return nil, errors.New("all providers failed")
		},
	}

	_, err := ao.Run(context.Background(), 123, 1, "fail", "user1", 100)
	if err == nil {
		t.Fatal("expected error for all providers failed")
	}
}

func TestAgentOrchestrator_SequentialDispatch_SideEffectTools(t *testing.T) {
	t.Parallel()
	ao, _, cleanup := setupOrchestratorTest(t, 123)
	defer cleanup()

	callCount := 0
	execOrder := []string{}
	ao.llmClient = &mockLLMClient{
		callGroupHistoryFunc: func(ctx context.Context, chatID int64, reqType, systemExtra string, history []llm.AgentMessage, tools []llm.Tool, requiresVision bool) (*llm.Response, error) {
			callCount++
			if callCount == 1 {
				return &llm.Response{
					ToolCalls: []llm.ToolCall{
						{ID: "c1", Type: "function", Function: llm.FunctionCall{Name: "manage_topic", Arguments: `{}`}},
						{ID: "c2", Type: "function", Function: llm.FunctionCall{Name: "generate_summary", Arguments: `{}`}},
					},
				}, nil
			}
			return &llm.Response{Content: "done"}, nil
		},
	}

	ao.executor = &mockToolExecutor{
		executeFunc: func(ctx context.Context, chatID int64, name string, arguments string) (string, error) {
			execOrder = append(execOrder, name)
			return "executed " + name, nil
		},
	}

	ao.Run(context.Background(), 123, 1, "sequential", "user1", 100)
	if len(execOrder) != 2 {
		t.Fatalf("expected 2 tool executions, got %d", len(execOrder))
	}
	if execOrder[0] != "manage_topic" {
		t.Errorf("expected manage_topic first (side-effect), got %s", execOrder[0])
	}
}
