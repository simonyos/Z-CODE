package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	provider      llm.Provider
	registry      *tools.Registry
	messages      []llm.Message
	handler       EventHandler
	maxIterations int
}

// AgentConfig holds configuration for creating a custom agent
type AgentConfig struct {
	Provider      llm.Provider
	ConfirmFn     tools.ConfirmFunc
	SystemPrompt  string   // Custom system prompt (empty = default)
	MaxIterations int      // Max LLM calls per conversation (0 = default 10)
	AllowedTools  []string // Tool names to enable (empty = all tools)
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
		provider:      provider,
		registry:      reg,
		maxIterations: 10,
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

	return &Agent{
		provider:      cfg.Provider,
		registry:      reg,
		maxIterations: maxIter,
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

// Chat sends a message and returns the response with tool execution info
func (a *Agent) Chat(ctx context.Context, userMessage string) (*ChatResult, error) {
	a.messages = append(a.messages, llm.Message{Role: "user", Content: userMessage})

	result := &ChatResult{
		ToolCalls: []ToolExecution{},
	}

	for i := 0; i < a.maxIterations; i++ {
		if a.handler != nil {
			a.handler.OnThinking()
		}

		response, err := a.provider.Generate(ctx, a.messages)
		if err != nil {
			return nil, err
		}

		// Try to parse as tool calls (supports multiple)
		toolCalls, err := tools.ParseToolCalls(response)
		if err == nil && len(toolCalls) > 0 {
			// Execute tool calls (parallel if multiple)
			execResults := a.executeToolCalls(ctx, toolCalls)

			// Record all tool executions
			for _, exec := range execResults {
				result.ToolCalls = append(result.ToolCalls, exec)
			}

			// Build XML results for message history
			var xmlResults []struct {
				ID     string
				Name   string
				Result tools.ToolResult
			}
			for _, exec := range execResults {
				xmlResults = append(xmlResults, struct {
					ID     string
					Name   string
					Result tools.ToolResult
				}{
					ID:   exec.ID,
					Name: exec.Name,
					Result: tools.ToolResult{
						Success: exec.Error == "",
						Output:  exec.Result,
						Error:   exec.Error,
					},
				})
			}

			// Add tool interaction to history using XML format
			a.messages = append(a.messages,
				llm.Message{Role: "assistant", Content: response},
				llm.Message{Role: "user", Content: tools.FormatToolResults(xmlResults)},
			)

			continue
		}

		// Not a tool call - final response
		a.messages = append(a.messages, llm.Message{Role: "assistant", Content: response})
		result.Response = response
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

// SetSystemPromptPrefix prepends content to the system prompt
func (a *Agent) SetSystemPromptPrefix(prefix string) {
	if len(a.messages) > 0 && a.messages[0].Role == "system" {
		a.messages[0].Content = prefix + "\n\n" + a.messages[0].Content
	}
}

// SetSystemPromptSuffix appends content to the system prompt
func (a *Agent) SetSystemPromptSuffix(suffix string) {
	if len(a.messages) > 0 && a.messages[0].Role == "system" {
		a.messages[0].Content = a.messages[0].Content + "\n\n" + suffix
	}
}

// GetSystemPrompt returns the current system prompt
func (a *Agent) GetSystemPrompt() string {
	if len(a.messages) > 0 && a.messages[0].Role == "system" {
		return a.messages[0].Content
	}
	return ""
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
func (a *Agent) ChatStream(ctx context.Context, userMessage string) <-chan StreamEvent {
	events := make(chan StreamEvent)

	go func() {
		defer close(events)

		a.messages = append(a.messages, llm.Message{Role: "user", Content: userMessage})

		events <- StreamEvent{Type: "start"}

		for i := 0; i < a.maxIterations; i++ {
			// Use streaming generation
			chunks, err := a.provider.GenerateStream(ctx, a.messages)
			if err != nil {
				events <- StreamEvent{Type: "error", Error: err}
				return
			}

			var fullResponse string
			chunkCount := 0
			for chunk := range chunks {
				chunkCount++
				if chunk.Error != nil {
					events <- StreamEvent{Type: "error", Error: chunk.Error}
					return
				}

				if chunk.Done {
					fullResponse = chunk.Text
					if os.Getenv("ZCODE_DEBUG") != "" {
						fmt.Fprintf(os.Stderr, "[DEBUG] Stream done, chunks=%d, fullResponse_len=%d\n", chunkCount, len(fullResponse))
					}
				} else {
					// Stream the chunk to UI
					events <- StreamEvent{Type: "chunk", Text: chunk.Text}
				}
			}

			// Try to parse as tool calls (supports multiple)
			toolCalls, err := tools.ParseToolCalls(fullResponse)
			// Debug: Log parse result when ZCODE_DEBUG is set
			if os.Getenv("ZCODE_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[DEBUG] ParseToolCalls: found=%d, err=%v, response_len=%d\n", len(toolCalls), err, len(fullResponse))
				if len(fullResponse) < 500 {
					fmt.Fprintf(os.Stderr, "[DEBUG] fullResponse: %q\n", fullResponse)
				}
			}
			if err == nil && len(toolCalls) > 0 {
				if os.Getenv("ZCODE_DEBUG") != "" {
					fmt.Fprintf(os.Stderr, "[DEBUG] About to execute %d tool calls\n", len(toolCalls))
				}
				// Notify about batch start if multiple tools
				if len(toolCalls) > 1 {
					events <- StreamEvent{
						Type:      "tool_batch_start",
						BatchSize: len(toolCalls),
					}
				}

				// Execute tool calls and stream results
				var xmlResults []struct {
					ID     string
					Name   string
					Result tools.ToolResult
				}

				for _, tc := range toolCalls {
					if os.Getenv("ZCODE_DEBUG") != "" {
						fmt.Fprintf(os.Stderr, "[DEBUG] Executing tool: %s (id=%s, args=%v)\n", tc.Name, tc.ID, tc.Arguments)
					}
					// Format args for display
					argsStr := formatArgs(tc.Name, tc.Arguments)

					// Notify about tool start
					events <- StreamEvent{
						Type:     "tool_start",
						ToolID:   tc.ID,
						ToolName: tc.Name,
						ToolArgs: argsStr,
					}

					// Execute tool
					toolResult := a.registry.Execute(ctx, tc)
					if os.Getenv("ZCODE_DEBUG") != "" {
						fmt.Fprintf(os.Stderr, "[DEBUG] Tool result: success=%v, output_len=%d, err=%s\n", toolResult.Success, len(toolResult.Output), toolResult.Error)
					}

					// Notify about tool result
					events <- StreamEvent{
						Type:       "tool_result",
						ToolID:     tc.ID,
						ToolName:   tc.Name,
						ToolResult: toolResult.Output,
						ToolError:  !toolResult.Success,
					}

					// Collect results for XML formatting
					xmlResults = append(xmlResults, struct {
						ID     string
						Name   string
						Result tools.ToolResult
					}{
						ID:     tc.ID,
						Name:   tc.Name,
						Result: toolResult,
					})
				}

				// Notify about batch end if multiple tools
				if len(toolCalls) > 1 {
					events <- StreamEvent{
						Type:      "tool_batch_end",
						BatchSize: len(toolCalls),
					}
				}

				// Add tool interaction to history using XML format
				a.messages = append(a.messages,
					llm.Message{Role: "assistant", Content: fullResponse},
					llm.Message{Role: "user", Content: tools.FormatToolResults(xmlResults)},
				)

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
