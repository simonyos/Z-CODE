package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/simonyos/Z-CODE/internal/llm"
	"github.com/simonyos/Z-CODE/internal/tools"
)

// Executor handles execution of custom agents
type Executor struct {
	provider  llm.Provider
	confirmFn tools.ConfirmFunc
	allTools  map[string]tools.Tool
}

// NewExecutor creates a new agent executor
func NewExecutor(provider llm.Provider, confirmFn tools.ConfirmFunc) *Executor {
	// Build a map of all available tools
	allTools := make(map[string]tools.Tool)

	// Create instances of all tools
	toolList := []tools.Tool{
		tools.NewReadFileTool(),
		tools.NewListDirTool(),
		tools.NewWriteFileTool(confirmFn),
		tools.NewEditTool(confirmFn),
		tools.NewBashTool(confirmFn),
		tools.NewGlobTool(),
		tools.NewGrepTool(),
	}

	for _, t := range toolList {
		allTools[t.Definition().Name] = t
	}

	return &Executor{
		provider:  provider,
		confirmFn: confirmFn,
		allTools:  allTools,
	}
}

// ExecuteResult contains the result of executing a custom agent
type ExecuteResult struct {
	Response  string
	ToolCalls []ToolExecution
	Handoff   *HandoffInstruction
}

// ToolExecution records a tool call and its result
type ToolExecution struct {
	ID     string
	Name   string
	Args   string
	Result string
	Error  string
}

// Execute runs a custom agent with the given prompt
func (e *Executor) Execute(ctx context.Context, def *AgentDefinition, userPrompt string) (*ExecuteResult, error) {
	toolProvider, ok := e.provider.(llm.ToolProvider)
	if !ok {
		return nil, fmt.Errorf("provider does not support native tool calling")
	}

	registry := e.buildRegistry(def)
	systemPrompt := e.buildSystemPrompt(def, registry)
	openAITools := registry.GetOpenAIToolDefinitions()

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	result := &ExecuteResult{
		ToolCalls: []ToolExecution{},
	}

	maxIterations := def.GetMaxIterations()
	for i := 0; i < maxIterations; i++ {
		resp, err := toolProvider.GenerateWithTools(ctx, messages, openAITools)
		if err != nil {
			return nil, err
		}

		// Check for handoff instruction
		if handoff := ParseHandoff(resp.Content); handoff != nil {
			result.Handoff = handoff
			result.Response = resp.Content
			return result, nil
		}

		// Check for tool calls
		if len(resp.ToolCalls) > 0 {
			// Execute tool calls
			execResults := e.executeNativeToolCalls(ctx, registry, resp.ToolCalls)
			result.ToolCalls = append(result.ToolCalls, execResults...)

			// Add assistant message with tool calls
			messages = append(messages, llm.Message{
				Role:      "assistant",
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})

			// Add tool result messages with name
			for _, exec := range execResults {
				resultContent := exec.Result
				if exec.Error != "" {
					resultContent = "Error: " + exec.Error
				}
				messages = append(messages, llm.Message{
					Role:       "tool",
					Content:    resultContent,
					Name:       exec.Name,
					ToolCallID: exec.ID,
				})
			}
			continue
		}

		// No tool calls - final response
		result.Response = resp.Content
		return result, nil
	}

	return nil, fmt.Errorf("max iterations (%d) reached", maxIterations)
}

// ExecuteStream runs a custom agent with streaming output
func (e *Executor) ExecuteStream(ctx context.Context, def *AgentDefinition, userPrompt string) <-chan StreamEvent {
	events := make(chan StreamEvent)

	go func() {
		defer close(events)

		toolProvider, ok := e.provider.(llm.ToolProvider)
		if !ok {
			events <- StreamEvent{Type: "error", Error: fmt.Errorf("provider does not support native tool calling")}
			return
		}

		registry := e.buildRegistry(def)
		systemPrompt := e.buildSystemPrompt(def, registry)
		openAITools := registry.GetOpenAIToolDefinitions()

		messages := []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		}

		events <- StreamEvent{Type: "start"}

		maxIterations := def.GetMaxIterations()
		for i := 0; i < maxIterations; i++ {
			chunks, err := toolProvider.GenerateStreamWithTools(ctx, messages, openAITools)
			if err != nil {
				events <- StreamEvent{Type: "error", Error: err}
				return
			}

			var fullContent string
			var toolCalls []llm.OpenAIToolCall
			for chunk := range chunks {
				if chunk.Error != nil {
					events <- StreamEvent{Type: "error", Error: chunk.Error}
					return
				}
				if chunk.Done {
					fullContent = chunk.Text
					toolCalls = chunk.ToolCalls
				} else {
					events <- StreamEvent{Type: "chunk", Text: chunk.Text}
				}
			}

			// Check for handoff
			if handoff := ParseHandoff(fullContent); handoff != nil {
				events <- StreamEvent{Type: "handoff", Handoff: handoff}
				events <- StreamEvent{Type: "done", FinalResponse: fullContent}
				return
			}

			// Check for tool calls
			if len(toolCalls) > 0 {
				if len(toolCalls) > 1 {
					events <- StreamEvent{Type: "tool_batch_start", BatchSize: len(toolCalls)}
				}

				var execResults []ToolExecution
				for _, tc := range toolCalls {
					events <- StreamEvent{
						Type:     "tool_start",
						ToolID:   tc.ID,
						ToolName: tc.Function.Name,
						ToolArgs: tc.Function.Arguments,
					}

					toolResult := registry.Execute(ctx, tools.ToolCall{
						ID:        tc.ID,
						Name:      tc.Function.Name,
						Arguments: parseToolArgs(tc.Function.Arguments),
					})

					events <- StreamEvent{
						Type:       "tool_result",
						ToolID:     tc.ID,
						ToolName:   tc.Function.Name,
						ToolResult: toolResult.Output,
						ToolError:  !toolResult.Success,
					}

					execResults = append(execResults, ToolExecution{
						ID:     tc.ID,
						Name:   tc.Function.Name,
						Args:   tc.Function.Arguments,
						Result: toolResult.Output,
						Error:  toolResult.Error,
					})
				}

				if len(toolCalls) > 1 {
					events <- StreamEvent{Type: "tool_batch_end", BatchSize: len(toolCalls)}
				}

				// Add assistant message with tool calls
				messages = append(messages, llm.Message{
					Role:      "assistant",
					Content:   fullContent,
					ToolCalls: toolCalls,
				})

				// Add tool result messages with name
				for _, exec := range execResults {
					resultContent := exec.Result
					if exec.Error != "" {
						resultContent = "Error: " + exec.Error
					}
					messages = append(messages, llm.Message{
						Role:       "tool",
						Content:    resultContent,
						Name:       exec.Name,
						ToolCallID: exec.ID,
					})
				}
				continue
			}

			// No tool calls - final response
			events <- StreamEvent{Type: "done", FinalResponse: fullContent}
			return
		}

		events <- StreamEvent{Type: "error", Error: fmt.Errorf("max iterations reached")}
	}()

	return events
}

// StreamEvent represents events during streaming execution
type StreamEvent struct {
	Type          string
	Text          string
	ToolID        string
	ToolName      string
	ToolArgs      string
	ToolResult    string
	ToolError     bool
	BatchSize     int
	FinalResponse string
	Handoff       *HandoffInstruction
	Error         error
}

// buildRegistry creates a tool registry for the agent
func (e *Executor) buildRegistry(def *AgentDefinition) *tools.Registry {
	registry := tools.NewRegistry()

	if len(def.Tools) == 0 {
		// No restrictions - register all tools
		for _, tool := range e.allTools {
			registry.Register(tool)
		}
	} else {
		// Register only allowed tools
		for _, name := range def.Tools {
			if tool, ok := e.allTools[name]; ok {
				registry.Register(tool)
			}
		}
	}

	return registry
}

// buildSystemPrompt creates the system prompt for the agent
// Note: Tool definitions are passed separately via the native tool calling API.
func (e *Executor) buildSystemPrompt(def *AgentDefinition, registry *tools.Registry) string {
	var sb strings.Builder

	// Start with the agent's custom system prompt
	sb.WriteString(def.SystemPrompt)
	sb.WriteString("\n\n")

	// Add current working directory
	cwd, _ := os.Getwd()
	sb.WriteString(fmt.Sprintf("Current working directory: %s\n\n", cwd))

	// Add handoff instructions if enabled
	if def.HandoffTo != "" {
		sb.WriteString("HANDOFF:\n")
		sb.WriteString("When you need to hand off to another agent, use this format:\n")
		sb.WriteString("```xml\n")
		sb.WriteString(fmt.Sprintf("<handoff agent=\"%s\" reason=\"Your reason here\">\n", def.HandoffTo))
		sb.WriteString("  <context key=\"key_name\">value</context>\n")
		sb.WriteString("</handoff>\n")
		sb.WriteString("```\n")
	}

	return sb.String()
}

// executeNativeToolCalls executes multiple OpenAI-format tool calls
func (e *Executor) executeNativeToolCalls(ctx context.Context, registry *tools.Registry, toolCalls []llm.OpenAIToolCall) []ToolExecution {
	results := make([]ToolExecution, len(toolCalls))

	for i, tc := range toolCalls {
		toolResult := registry.Execute(ctx, tools.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: parseToolArgs(tc.Function.Arguments),
		})

		results[i] = ToolExecution{
			ID:     tc.ID,
			Name:   tc.Function.Name,
			Args:   tc.Function.Arguments,
			Result: toolResult.Output,
			Error:  toolResult.Error,
		}
	}

	return results
}

// parseToolArgs parses JSON arguments into a map
func parseToolArgs(argsJSON string) map[string]any {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		if os.Getenv("ZCODE_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG parseToolArgs] failed to parse: %v, input: %q\n", err, argsJSON)
		}
		return make(map[string]any)
	}
	return args
}
