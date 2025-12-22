package agent

import (
	"context"
	"testing"

	"github.com/simonyos/Z-CODE/internal/llm"
	"github.com/simonyos/Z-CODE/internal/tools"
)

// MockProvider is a test implementation of the LLM Provider interface
type MockProvider struct {
	responses []string
	callCount int
}

func NewMockProvider(responses ...string) *MockProvider {
	return &MockProvider{responses: responses}
}

func (m *MockProvider) Generate(ctx context.Context, messages []llm.Message) (string, error) {
	if m.callCount >= len(m.responses) {
		return "final response", nil
	}
	response := m.responses[m.callCount]
	m.callCount++
	return response, nil
}

func (m *MockProvider) GenerateStream(ctx context.Context, messages []llm.Message) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	go func() {
		defer close(ch)
		response, _ := m.Generate(ctx, messages)
		ch <- llm.StreamChunk{Text: response, Done: true}
	}()
	return ch, nil
}

// MockEventHandler records events for testing
type MockEventHandler struct {
	ThinkingCalls  int
	ToolUseCalls   []string
	ToolResultLogs []string
}

func (h *MockEventHandler) OnThinking() {
	h.ThinkingCalls++
}

func (h *MockEventHandler) OnToolUse(name string, args map[string]any) {
	h.ToolUseCalls = append(h.ToolUseCalls, name)
}

func (h *MockEventHandler) OnToolResult(name string, result tools.ToolResult) {
	h.ToolResultLogs = append(h.ToolResultLogs, name)
}

func alwaysConfirm(prompt string) bool {
	return true
}

func TestNewAgent(t *testing.T) {
	provider := NewMockProvider()
	agent := New(provider, alwaysConfirm)

	if agent == nil {
		t.Fatal("New() returned nil")
	}
	if agent.provider == nil {
		t.Error("New() agent.provider is nil")
	}
	if agent.registry == nil {
		t.Error("New() agent.registry is nil")
	}
	if len(agent.messages) == 0 {
		t.Error("New() should initialize with system message")
	}
	if agent.messages[0].Role != "system" {
		t.Errorf("New() first message role = %q, want %q", agent.messages[0].Role, "system")
	}
}

func TestAgent_SetEventHandler(t *testing.T) {
	provider := NewMockProvider()
	agent := New(provider, alwaysConfirm)

	handler := &MockEventHandler{}
	agent.SetEventHandler(handler)

	if agent.handler == nil {
		t.Error("SetEventHandler() did not set handler")
	}
}

func TestAgent_Chat_SimpleResponse(t *testing.T) {
	provider := NewMockProvider("Hello! How can I help you?")
	agent := New(provider, alwaysConfirm)

	ctx := context.Background()
	result, err := agent.Chat(ctx, "Hi there")

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if result == nil {
		t.Fatal("Chat() returned nil result")
	}
	if result.Response != "Hello! How can I help you?" {
		t.Errorf("Chat().Response = %q, want %q", result.Response, "Hello! How can I help you?")
	}
	if len(result.ToolCalls) != 0 {
		t.Errorf("Chat() should have no tool calls, got %d", len(result.ToolCalls))
	}
}

func TestAgent_Chat_WithToolCall(t *testing.T) {
	// First response is a tool call, second is the final response
	provider := NewMockProvider(
		`{"tool": "list_dir", "path": "."}`,
		"The directory contains several files.",
	)
	agent := New(provider, alwaysConfirm)

	ctx := context.Background()
	result, err := agent.Chat(ctx, "What files are here?")

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if result == nil {
		t.Fatal("Chat() returned nil result")
	}
	if len(result.ToolCalls) != 1 {
		t.Errorf("Chat() should have 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Name != "list_dir" {
		t.Errorf("Chat() tool call name = %q, want %q", result.ToolCalls[0].Name, "list_dir")
	}
	if result.Response != "The directory contains several files." {
		t.Errorf("Chat().Response = %q", result.Response)
	}
}

func TestAgent_Chat_WithEventHandler(t *testing.T) {
	provider := NewMockProvider(
		`{"tool": "list_dir", "path": "."}`,
		"Done!",
	)
	agent := New(provider, alwaysConfirm)

	handler := &MockEventHandler{}
	agent.SetEventHandler(handler)

	ctx := context.Background()
	_, err := agent.Chat(ctx, "List files")

	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if handler.ThinkingCalls < 1 {
		t.Error("OnThinking() should be called at least once")
	}
	if len(handler.ToolUseCalls) != 1 {
		t.Errorf("OnToolUse() should be called once, got %d", len(handler.ToolUseCalls))
	}
	if len(handler.ToolResultLogs) != 1 {
		t.Errorf("OnToolResult() should be called once, got %d", len(handler.ToolResultLogs))
	}
}

func TestAgent_History(t *testing.T) {
	provider := NewMockProvider("Response 1", "Response 2")
	agent := New(provider, alwaysConfirm)

	ctx := context.Background()
	agent.Chat(ctx, "First message")
	agent.Chat(ctx, "Second message")

	history := agent.History()

	// Should have: system, user1, assistant1, user2, assistant2
	expectedMinLen := 5
	if len(history) < expectedMinLen {
		t.Errorf("History() len = %d, want at least %d", len(history), expectedMinLen)
	}

	// First should be system message
	if history[0].Role != "system" {
		t.Errorf("History()[0].Role = %q, want %q", history[0].Role, "system")
	}
}

func TestAgent_Reset(t *testing.T) {
	provider := NewMockProvider("Response")
	agent := New(provider, alwaysConfirm)

	ctx := context.Background()
	agent.Chat(ctx, "Some message")

	// Should have more than just system message now
	if len(agent.messages) <= 1 {
		t.Error("messages should grow after Chat()")
	}

	agent.Reset()

	// Should only have system message
	if len(agent.messages) != 1 {
		t.Errorf("Reset() should leave 1 message, got %d", len(agent.messages))
	}
	if agent.messages[0].Role != "system" {
		t.Error("Reset() should keep system message")
	}
}

func TestAgent_AddTool(t *testing.T) {
	provider := NewMockProvider()
	agent := New(provider, alwaysConfirm)

	// Create a custom tool
	customTool := &CustomTool{
		BaseTool: tools.BaseTool{
			Def: tools.ToolDefinition{
				Name:        "custom_tool",
				Description: "A custom test tool",
			},
		},
	}

	agent.AddTool(customTool)

	// Verify tool was added by checking registry
	tool, ok := agent.registry.Get("custom_tool")
	if !ok {
		t.Error("AddTool() tool not found in registry")
	}
	if tool == nil {
		t.Error("AddTool() tool is nil")
	}
}

func TestAgent_ChatStream(t *testing.T) {
	provider := NewMockProvider("Streamed response")
	agent := New(provider, alwaysConfirm)

	ctx := context.Background()
	events := agent.ChatStream(ctx, "Stream test")

	var receivedEvents []StreamEvent
	for event := range events {
		receivedEvents = append(receivedEvents, event)
	}

	if len(receivedEvents) < 2 {
		t.Errorf("ChatStream() should return at least 2 events (start, done), got %d", len(receivedEvents))
	}

	// Check for start event
	hasStart := false
	hasDone := false
	for _, e := range receivedEvents {
		if e.Type == "start" {
			hasStart = true
		}
		if e.Type == "done" {
			hasDone = true
		}
	}

	if !hasStart {
		t.Error("ChatStream() should emit 'start' event")
	}
	if !hasDone {
		t.Error("ChatStream() should emit 'done' event")
	}
}

func TestFormatArgs(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		args     map[string]any
		want     string
	}{
		{
			name:     "run_command",
			toolName: "run_command",
			args:     map[string]any{"command": "ls -la"},
			want:     "ls -la",
		},
		{
			name:     "read_file",
			toolName: "read_file",
			args:     map[string]any{"path": "/tmp/test.txt"},
			want:     "/tmp/test.txt",
		},
		{
			name:     "write_file",
			toolName: "write_file",
			args:     map[string]any{"path": "/tmp/out.txt", "content": "hello"},
			want:     "/tmp/out.txt",
		},
		{
			name:     "list_dir with path",
			toolName: "list_dir",
			args:     map[string]any{"path": "/home"},
			want:     "/home",
		},
		{
			name:     "list_dir without path",
			toolName: "list_dir",
			args:     map[string]any{},
			want:     ".",
		},
		{
			name:     "unknown tool",
			toolName: "unknown",
			args:     map[string]any{"foo": "bar"},
			want:     `{"foo":"bar"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatArgs(tt.toolName, tt.args)
			if result != tt.want {
				t.Errorf("formatArgs(%q, %v) = %q, want %q", tt.toolName, tt.args, result, tt.want)
			}
		})
	}
}

// CustomTool is a test tool that embeds BaseTool
type CustomTool struct {
	tools.BaseTool
}

func (t *CustomTool) Execute(ctx context.Context, args map[string]any) tools.ToolResult {
	return tools.ToolResult{Success: true, Output: "custom result"}
}
