package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/simonyos/Z-CODE/internal/llm"
	"github.com/simonyos/Z-CODE/internal/tools"
)

// ToolExecution records a single tool call and its result
type ToolExecution struct {
	Name   string
	Args   string // Formatted args string for display
	Result string
	Error  string
}

// ChatResult contains the response and any tool executions
type ChatResult struct {
	Response  string
	ToolCalls []ToolExecution
}

// StreamEvent represents events during streaming chat
type StreamEvent struct {
	Type string // "start", "chunk", "tool_start", "tool_result", "done", "error"

	// For chunk events
	Text string

	// For tool events
	ToolName   string
	ToolArgs   string
	ToolResult string
	ToolError  bool

	// For done event
	FinalResponse string

	// For error event
	Error error
}

// EventHandler receives callbacks during agent execution
type EventHandler interface {
	OnThinking()
	OnToolUse(name string, args map[string]any)
	OnToolResult(name string, result tools.ToolResult)
}

// Agent orchestrates the LLM and tools
type Agent struct {
	provider llm.Provider
	registry *tools.Registry
	messages []llm.Message
	handler  EventHandler
}

// New creates a new agent with the given provider
func New(provider llm.Provider, confirmFn tools.ConfirmFunc) *Agent {
	reg := tools.NewRegistry()

	// Register default tools
	reg.Register(tools.NewReadFileTool())
	reg.Register(tools.NewListDirTool())
	reg.Register(tools.NewWriteFileTool(confirmFn))
	reg.Register(tools.NewBashTool(confirmFn))

	return &Agent{
		provider: provider,
		registry: reg,
		messages: []llm.Message{
			{Role: "system", Content: reg.BuildSystemPrompt()},
		},
	}
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

	maxIterations := 10
	for i := 0; i < maxIterations; i++ {
		if a.handler != nil {
			a.handler.OnThinking()
		}

		response, err := a.provider.Generate(ctx, a.messages)
		if err != nil {
			return nil, err
		}

		// Try to parse as tool call
		toolCall, err := tools.ParseToolCall(response)
		if err == nil && toolCall != nil {
			if a.handler != nil {
				a.handler.OnToolUse(toolCall.Name, toolCall.Arguments)
			}

			toolResult := a.registry.Execute(ctx, *toolCall)

			if a.handler != nil {
				a.handler.OnToolResult(toolCall.Name, toolResult)
			}

			// Format args for display
			argsStr := formatArgs(toolCall.Name, toolCall.Arguments)

			// Record the tool execution
			exec := ToolExecution{
				Name:   toolCall.Name,
				Args:   argsStr,
				Result: toolResult.Output,
				Error:  toolResult.Error,
			}
			result.ToolCalls = append(result.ToolCalls, exec)

			// Add tool interaction to history
			a.messages = append(a.messages,
				llm.Message{Role: "assistant", Content: response},
				llm.Message{Role: "user", Content: fmt.Sprintf("Tool result: %s", toolResult.Output)},
			)

			if !toolResult.Success && toolResult.Error != "" {
				a.messages[len(a.messages)-1].Content = fmt.Sprintf("Tool error: %s", toolResult.Error)
			}

			continue
		}

		// Not a tool call - final response
		a.messages = append(a.messages, llm.Message{Role: "assistant", Content: response})
		result.Response = response
		return result, nil
	}

	return nil, fmt.Errorf("max iterations reached")
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
	case "list_dir":
		if path, ok := args["path"].(string); ok {
			return path
		}
		return "."
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

// ChatStream sends a message and streams the response through a channel
func (a *Agent) ChatStream(ctx context.Context, userMessage string) <-chan StreamEvent {
	events := make(chan StreamEvent)

	go func() {
		defer close(events)

		a.messages = append(a.messages, llm.Message{Role: "user", Content: userMessage})

		events <- StreamEvent{Type: "start"}

		maxIterations := 10
		for i := 0; i < maxIterations; i++ {
			// Use streaming generation
			chunks, err := a.provider.GenerateStream(ctx, a.messages)
			if err != nil {
				events <- StreamEvent{Type: "error", Error: err}
				return
			}

			var fullResponse string
			for chunk := range chunks {
				if chunk.Error != nil {
					events <- StreamEvent{Type: "error", Error: chunk.Error}
					return
				}

				if chunk.Done {
					fullResponse = chunk.Text
				} else {
					// Stream the chunk to UI
					events <- StreamEvent{Type: "chunk", Text: chunk.Text}
				}
			}

			// Try to parse as tool call
			toolCall, err := tools.ParseToolCall(fullResponse)
			if err == nil && toolCall != nil {
				// Format args for display
				argsStr := formatArgs(toolCall.Name, toolCall.Arguments)

				// Notify about tool start
				events <- StreamEvent{
					Type:     "tool_start",
					ToolName: toolCall.Name,
					ToolArgs: argsStr,
				}

				// Execute tool
				toolResult := a.registry.Execute(ctx, *toolCall)

				// Notify about tool result
				events <- StreamEvent{
					Type:       "tool_result",
					ToolName:   toolCall.Name,
					ToolResult: toolResult.Output,
					ToolError:  !toolResult.Success,
				}

				// Add tool interaction to history
				a.messages = append(a.messages,
					llm.Message{Role: "assistant", Content: fullResponse},
					llm.Message{Role: "user", Content: fmt.Sprintf("Tool result: %s", toolResult.Output)},
				)

				if !toolResult.Success && toolResult.Error != "" {
					a.messages[len(a.messages)-1].Content = fmt.Sprintf("Tool error: %s", toolResult.Error)
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
