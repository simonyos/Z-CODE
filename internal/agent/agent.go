package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/simonyos/Z-CODE/internal/llm"
	"github.com/simonyos/Z-CODE/internal/tools"
)

// ToolExecution records a single tool call and its result
type ToolExecution struct {
	ID     string
	Name   string
	Args   string // Formatted args string for display
	Result string
	Error  string
}

// HandoffInstruction represents a request to transfer control to another agent
type HandoffInstruction struct {
	TargetAgent string
	Reason      string
	Context     map[string]any
}

// ChatResult contains the response and any tool executions
type ChatResult struct {
	Response  string
	ToolCalls []ToolExecution
	Handoff   *HandoffInstruction // Non-nil if handoff was requested
}

// StreamEvent represents events during streaming chat
type StreamEvent struct {
	Type string // "start", "chunk", "tool_start", "tool_result", "tool_batch_start", "tool_batch_end", "done", "error"

	// For chunk events
	Text string

	// For tool events
	ToolID     string
	ToolName   string
	ToolArgs   string
	ToolResult string
	ToolError  bool

	// For batch events
	BatchSize int

	// For done event
	FinalResponse string

	// For error event
	Error error

	// For handoff event
	Handoff *HandoffInstruction
}

// EventHandler receives callbacks during agent execution.
// Implementations MUST be thread-safe as callbacks may be invoked
// concurrently from multiple goroutines during parallel tool execution.
type EventHandler interface {
	OnThinking()
	OnToolUse(name string, args map[string]any)
	OnToolResult(name string, result tools.ToolResult)
}

// Agent orchestrates the LLM and tools
type Agent struct {
	provider       llm.Provider
	registry       *tools.Registry
	messages       []llm.Message
	handler        EventHandler
	maxIterations  int
	maxToolRetries int
}

// AgentConfig holds configuration for creating a custom agent
type AgentConfig struct {
	Provider       llm.Provider
	ConfirmFn      tools.ConfirmFunc
	SystemPrompt   string   // Custom system prompt (empty = default)
	MaxIterations  int      // Max LLM calls per conversation (0 = default 10)
	AllowedTools   []string // Tool names to enable (empty = all tools)
	MaxToolRetries int      // Max retries for failed tool calls (0 = default 3)
}

// New creates a new agent with the given provider
func New(provider llm.Provider, confirmFn tools.ConfirmFunc) *Agent {
	reg := tools.NewRegistry()

	// Register default tools
	reg.Register(tools.NewReadFileTool())
	reg.Register(tools.NewListDirTool())
	reg.Register(tools.NewWriteFileTool(confirmFn))
	reg.Register(tools.NewEditTool(confirmFn))
	reg.Register(tools.NewBashTool(confirmFn))
	reg.Register(tools.NewGlobTool())
	reg.Register(tools.NewGrepTool())

	return &Agent{
		provider:       provider,
		registry:       reg,
		maxIterations:  10,
		maxToolRetries: 3,
		messages: []llm.Message{
			{Role: "system", Content: reg.BuildSystemPrompt()},
		},
	}
}

// NewWithConfig creates a new agent with custom configuration
func NewWithConfig(cfg AgentConfig) *Agent {
	reg := tools.NewRegistry()

	// Build map of all available tools
	allTools := map[string]tools.Tool{
		"read_file":  tools.NewReadFileTool(),
		"list_dir":   tools.NewListDirTool(),
		"write_file": tools.NewWriteFileTool(cfg.ConfirmFn),
		"edit_file":  tools.NewEditTool(cfg.ConfirmFn),
		"run_command": tools.NewBashTool(cfg.ConfirmFn),
		"glob":       tools.NewGlobTool(),
		"grep":       tools.NewGrepTool(),
	}

	// Register tools based on config
	if len(cfg.AllowedTools) == 0 {
		// Register all tools
		for _, tool := range allTools {
			reg.Register(tool)
		}
	} else {
		// Register only allowed tools
		for _, name := range cfg.AllowedTools {
			if tool, ok := allTools[name]; ok {
				reg.Register(tool)
			}
		}
	}

	// Determine system prompt
	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = reg.BuildSystemPrompt()
	}

	// Determine max iterations
	maxIter := cfg.MaxIterations
	if maxIter <= 0 {
		maxIter = 10
	}

	// Determine max tool retries
	maxRetries := cfg.MaxToolRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	return &Agent{
		provider:       cfg.Provider,
		registry:       reg,
		maxIterations:  maxIter,
		maxToolRetries: maxRetries,
		messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
		},
	}
}

// Provider returns the LLM provider
func (a *Agent) Provider() llm.Provider {
	return a.provider
}

// SetEventHandler sets the callback handler for agent events
func (a *Agent) SetEventHandler(h EventHandler) {
	a.handler = h
}

// AddTool dynamically registers a new tool
func (a *Agent) AddTool(tool tools.Tool) {
	a.registry.Register(tool)
	// Rebuild system prompt with new tool
	a.messages[0].Content = a.registry.BuildSystemPrompt()
}

// Chat sends a message and returns the response with tool execution info.
// All providers must implement ToolProvider for native tool calling support.
func (a *Agent) Chat(ctx context.Context, userMessage string) (*ChatResult, error) {
	toolProvider, ok := a.provider.(llm.ToolProvider)
	if !ok {
		return nil, fmt.Errorf("provider does not support native tool calling (must implement ToolProvider interface)")
	}
	return a.chatWithNativeTools(ctx, userMessage, toolProvider)
}

// chatWithNativeTools uses the provider's native tool calling API
func (a *Agent) chatWithNativeTools(ctx context.Context, userMessage string, toolProvider llm.ToolProvider) (*ChatResult, error) {
	a.messages = append(a.messages, llm.Message{Role: "user", Content: userMessage})

	result := &ChatResult{
		ToolCalls: []ToolExecution{},
	}

	// Get tool definitions in OpenAI format (already returns []llm.OpenAITool)
	llmTools := a.registry.GetOpenAIToolDefinitions()

	retryCount := 0 // Total retries allowed per Chat() call

	for i := 0; i < a.maxIterations; i++ {
		if a.handler != nil {
			a.handler.OnThinking()
		}

		response, err := toolProvider.GenerateWithTools(ctx, a.messages, llmTools)
		if err != nil {
			return nil, err
		}

		// Check if model returned tool calls
		if len(response.ToolCalls) > 0 {
			// Convert OpenAI tool calls to our ToolCall format with retry on parse failure
			var toolCalls []tools.ToolCall
			var parseErrors []string

			for _, tc := range response.ToolCalls {
				// Parse arguments JSON
				var args map[string]any
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					parseErrors = append(parseErrors, fmt.Sprintf(
						"Tool '%s' (id: %s): failed to parse arguments: %v. Raw: %s",
						tc.Function.Name, tc.ID, err, tc.Function.Arguments,
					))
					continue
				}
				toolCalls = append(toolCalls, tools.ToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: args,
				})
			}

			// If there were parse errors, inject error message and retry
			if len(parseErrors) > 0 && len(toolCalls) == 0 {
				retryCount++
				if retryCount > a.maxToolRetries {
					return nil, fmt.Errorf("max tool retries exceeded. Last errors:\n%s",
						strings.Join(parseErrors, "\n"))
				}

				// Inject error message for the model to fix
				errorMsg := fmt.Sprintf(
					"Tool call failed due to malformed arguments. Please fix and try again:\n%s",
					strings.Join(parseErrors, "\n"),
				)
				a.messages = append(a.messages,
					llm.Message{Role: "assistant", Content: response.Content},
					llm.Message{Role: "user", Content: errorMsg},
				)
				continue
			}

			// Execute tool calls (parallel if multiple)
			execResults := a.executeToolCalls(ctx, toolCalls)

			// Record all tool executions
			for _, exec := range execResults {
				result.ToolCalls = append(result.ToolCalls, exec)
			}

			// Build tool results for message history (OpenAI format)
			// Add assistant message with tool calls
			assistantMsg := llm.Message{
				Role:      "assistant",
				Content:   response.Content,
				ToolCalls: response.ToolCalls,
			}
			a.messages = append(a.messages, assistantMsg)

			// Add tool result messages with proper tool_call_id and name
			for _, exec := range execResults {
				content := exec.Result
				if exec.Error != "" {
					content = "Error: " + exec.Error
				}
				a.messages = append(a.messages, llm.Message{
					Role:       "tool",
					Content:    content,
					Name:       exec.Name,
					ToolCallID: exec.ID,
				})
			}

			continue
		}

		// No tool calls - final response
		a.messages = append(a.messages, llm.Message{Role: "assistant", Content: response.Content})
		result.Response = response.Content
		return result, nil
	}

	return nil, fmt.Errorf("max iterations reached")
}

// executeToolCalls executes multiple tool calls, in parallel if more than one
func (a *Agent) executeToolCalls(ctx context.Context, toolCalls []tools.ToolCall) []ToolExecution {
	if len(toolCalls) == 1 {
		// Single tool call - execute directly
		tc := toolCalls[0]
		if a.handler != nil {
			a.handler.OnToolUse(tc.Name, tc.Arguments)
		}

		toolResult := a.registry.Execute(ctx, tc)

		if a.handler != nil {
			a.handler.OnToolResult(tc.Name, toolResult)
		}

		return []ToolExecution{{
			ID:     tc.ID,
			Name:   tc.Name,
			Args:   formatArgs(tc.Name, tc.Arguments),
			Result: toolResult.Output,
			Error:  toolResult.Error,
		}}
	}

	// Multiple tool calls - execute in parallel
	results := make([]ToolExecution, len(toolCalls))
	var wg sync.WaitGroup

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, call tools.ToolCall) {
			defer wg.Done()

			if a.handler != nil {
				a.handler.OnToolUse(call.Name, call.Arguments)
			}

			toolResult := a.registry.Execute(ctx, call)

			if a.handler != nil {
				a.handler.OnToolResult(call.Name, toolResult)
			}

			results[idx] = ToolExecution{
				ID:     call.ID,
				Name:   call.Name,
				Args:   formatArgs(call.Name, call.Arguments),
				Result: toolResult.Output,
				Error:  toolResult.Error,
			}
		}(i, tc)
	}

	wg.Wait()
	return results
}

// formatArgs creates a display string for tool arguments
func formatArgs(toolName string, args map[string]any) string {
	switch toolName {
	case "run_command":
		if cmd, ok := args["command"].(string); ok {
			return cmd
		}
	case "read_file":
		if path, ok := args["path"].(string); ok {
			return path
		}
	case "write_file":
		if path, ok := args["path"].(string); ok {
			return path
		}
	case "edit_file":
		if path, ok := args["path"].(string); ok {
			return path
		}
	case "list_dir":
		if path, ok := args["path"].(string); ok {
			return path
		}
		return "."
	case "glob":
		if pattern, ok := args["pattern"].(string); ok {
			return pattern
		}
	case "grep":
		if pattern, ok := args["pattern"].(string); ok {
			return pattern
		}
	}
	// Fallback: JSON representation
	bytes, _ := json.Marshal(args)
	return string(bytes)
}

// History returns the conversation history
func (a *Agent) History() []llm.Message {
	return a.messages
}

// Reset clears the conversation history (keeps system prompt)
func (a *Agent) Reset() {
	a.messages = a.messages[:1] // Keep only system prompt
}

// ChatStream sends a message and streams the response through a channel.
// Unlike Chat(), tool calls are executed sequentially rather than in parallel.
// This is intentional to ensure proper event ordering for streaming UI updates:
// each tool_start event is followed by its corresponding tool_result before
// the next tool begins, making the output easier to follow in real-time.
//
// When multiple tools are requested, tool_batch_start and tool_batch_end events
// are emitted to indicate the grouping, but tools within the batch still execute
// sequentially (not in parallel) for predictable streaming output.
//
// All providers must implement ToolProvider for native tool calling support.
func (a *Agent) ChatStream(ctx context.Context, userMessage string) <-chan StreamEvent {
	toolProvider, ok := a.provider.(llm.ToolProvider)
	if !ok {
		events := make(chan StreamEvent)
		go func() {
			events <- StreamEvent{Type: "error", Error: fmt.Errorf("provider does not support native tool calling (must implement ToolProvider interface)")}
			close(events)
		}()
		return events
	}
	return a.chatStreamWithNativeTools(ctx, userMessage, toolProvider)
}

// chatStreamWithNativeTools uses the provider's native streaming tool calling API
func (a *Agent) chatStreamWithNativeTools(ctx context.Context, userMessage string, toolProvider llm.ToolProvider) <-chan StreamEvent {
	events := make(chan StreamEvent)

	go func() {
		defer close(events)

		a.messages = append(a.messages, llm.Message{Role: "user", Content: userMessage})

		events <- StreamEvent{Type: "start"}

		// Get tool definitions in OpenAI format (already returns []llm.OpenAITool)
		llmTools := a.registry.GetOpenAIToolDefinitions()

		retryCount := 0 // Total retries allowed per ChatStream() call

		for i := 0; i < a.maxIterations; i++ {
			// Use streaming generation with tools
			chunks, err := toolProvider.GenerateStreamWithTools(ctx, a.messages, llmTools)
			if err != nil {
				events <- StreamEvent{Type: "error", Error: err}
				return
			}

			var fullResponse string
			var toolCalls []llm.OpenAIToolCall

			for chunk := range chunks {
				if chunk.Error != nil {
					events <- StreamEvent{Type: "error", Error: chunk.Error}
					return
				}

				if chunk.Done {
					fullResponse = chunk.Text
					toolCalls = chunk.ToolCalls
				} else if chunk.Text != "" {
					// Stream the chunk to UI
					events <- StreamEvent{Type: "chunk", Text: chunk.Text}
				}
			}

			// Check if model returned tool calls
			if len(toolCalls) > 0 {
				// Parse and validate tool calls with retry on failure
				var parsedToolCalls []tools.ToolCall
				var parseErrors []string

				for _, tc := range toolCalls {
					var args map[string]any
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
						parseErrors = append(parseErrors, fmt.Sprintf(
							"Tool '%s' (id: %s): failed to parse arguments: %v",
							tc.Function.Name, tc.ID, err,
						))
						continue
					}
					parsedToolCalls = append(parsedToolCalls, tools.ToolCall{
						ID:        tc.ID,
						Name:      tc.Function.Name,
						Arguments: args,
					})
				}

				// If all tool calls failed to parse, retry
				if len(parseErrors) > 0 && len(parsedToolCalls) == 0 {
					retryCount++
					if retryCount > a.maxToolRetries {
						events <- StreamEvent{
							Type:  "error",
							Error: fmt.Errorf("max tool retries exceeded: %s", strings.Join(parseErrors, "; ")),
						}
						return
					}

					// Inject error message for retry
					errorMsg := fmt.Sprintf(
						"Tool call failed due to malformed arguments. Please fix and try again:\n%s",
						strings.Join(parseErrors, "\n"),
					)
					a.messages = append(a.messages,
						llm.Message{Role: "assistant", Content: fullResponse},
						llm.Message{Role: "user", Content: errorMsg},
					)
					continue
				}

				// Add assistant message with tool calls to history FIRST
				a.messages = append(a.messages, llm.Message{
					Role:      "assistant",
					Content:   fullResponse,
					ToolCalls: toolCalls,
				})

				// Notify about batch start if multiple tools
				if len(parsedToolCalls) > 1 {
					events <- StreamEvent{
						Type:      "tool_batch_start",
						BatchSize: len(parsedToolCalls),
					}
				}

				// Execute tool calls and stream results
				for _, toolCall := range parsedToolCalls {
					// Format args for display
					argsStr := formatArgs(toolCall.Name, toolCall.Arguments)

					// Notify about tool start
					events <- StreamEvent{
						Type:     "tool_start",
						ToolID:   toolCall.ID,
						ToolName: toolCall.Name,
						ToolArgs: argsStr,
					}

					// Execute tool
					toolResult := a.registry.Execute(ctx, toolCall)

					// Notify about tool result
					events <- StreamEvent{
						Type:       "tool_result",
						ToolID:     toolCall.ID,
						ToolName:   toolCall.Name,
						ToolResult: toolResult.Output,
						ToolError:  !toolResult.Success,
					}

					// Add tool result to message history with proper tool_call_id and name
					content := toolResult.Output
					if toolResult.Error != "" {
						content = "Error: " + toolResult.Error
					}
					a.messages = append(a.messages, llm.Message{
						Role:       "tool",
						Content:    content,
						Name:       toolCall.Name,
						ToolCallID: toolCall.ID,
					})
				}

				// Notify about batch end if multiple tools
				if len(parsedToolCalls) > 1 {
					events <- StreamEvent{
						Type:      "tool_batch_end",
						BatchSize: len(parsedToolCalls),
					}
				}

				continue
			}

			// Not a tool call - final response
			a.messages = append(a.messages, llm.Message{Role: "assistant", Content: fullResponse})
			events <- StreamEvent{Type: "done", FinalResponse: fullResponse}
			return
		}

		events <- StreamEvent{Type: "error", Error: fmt.Errorf("max iterations reached")}
	}()

	return events
}
