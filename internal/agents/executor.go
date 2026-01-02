package agents

import (
	"context"
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
	registry := e.buildRegistry(def)
	systemPrompt := e.buildSystemPrompt(def, registry)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	result := &ExecuteResult{
		ToolCalls: []ToolExecution{},
	}

	maxIterations := def.GetMaxIterations()
	for i := 0; i < maxIterations; i++ {
		response, err := e.provider.Generate(ctx, messages)
		if err != nil {
			return nil, err
		}

		// Check for handoff instruction
		if handoff := ParseHandoff(response); handoff != nil {
			result.Handoff = handoff
			result.Response = response
			return result, nil
		}

		// Try to parse as tool calls
		toolCalls, err := tools.ParseToolCalls(response)
		if err == nil && len(toolCalls) > 0 {
			// Execute tool calls
			execResults := e.executeToolCalls(ctx, registry, toolCalls)
			result.ToolCalls = append(result.ToolCalls, execResults...)

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

			messages = append(messages,
				llm.Message{Role: "assistant", Content: response},
				llm.Message{Role: "user", Content: tools.FormatToolResults(xmlResults)},
			)
			continue
		}

		// Not a tool call - final response
		result.Response = response
		return result, nil
	}

	return nil, fmt.Errorf("max iterations (%d) reached", maxIterations)
}

// ExecuteStream runs a custom agent with streaming output
func (e *Executor) ExecuteStream(ctx context.Context, def *AgentDefinition, userPrompt string) <-chan StreamEvent {
	events := make(chan StreamEvent)

	go func() {
		defer close(events)

		registry := e.buildRegistry(def)
		systemPrompt := e.buildSystemPrompt(def, registry)

		messages := []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		}

		events <- StreamEvent{Type: "start"}

		maxIterations := def.GetMaxIterations()
		for i := 0; i < maxIterations; i++ {
			chunks, err := e.provider.GenerateStream(ctx, messages)
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
					events <- StreamEvent{Type: "chunk", Text: chunk.Text}
				}
			}

			// Check for handoff
			if handoff := ParseHandoff(fullResponse); handoff != nil {
				events <- StreamEvent{Type: "handoff", Handoff: handoff}
				events <- StreamEvent{Type: "done", FinalResponse: fullResponse}
				return
			}

			// Try to parse as tool calls
			toolCalls, err := tools.ParseToolCalls(fullResponse)
			if err == nil && len(toolCalls) > 0 {
				if len(toolCalls) > 1 {
					events <- StreamEvent{Type: "tool_batch_start", BatchSize: len(toolCalls)}
				}

				var xmlResults []struct {
					ID     string
					Name   string
					Result tools.ToolResult
				}

				for _, tc := range toolCalls {
					events <- StreamEvent{
						Type:     "tool_start",
						ToolID:   tc.ID,
						ToolName: tc.Name,
						ToolArgs: formatArgs(tc.Arguments),
					}

					toolResult := registry.Execute(ctx, tc)

					events <- StreamEvent{
						Type:       "tool_result",
						ToolID:     tc.ID,
						ToolName:   tc.Name,
						ToolResult: toolResult.Output,
						ToolError:  !toolResult.Success,
					}

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

				if len(toolCalls) > 1 {
					events <- StreamEvent{Type: "tool_batch_end", BatchSize: len(toolCalls)}
				}

				messages = append(messages,
					llm.Message{Role: "assistant", Content: fullResponse},
					llm.Message{Role: "user", Content: tools.FormatToolResults(xmlResults)},
				)
				continue
			}

			// Not a tool call - final response
			events <- StreamEvent{Type: "done", FinalResponse: fullResponse}
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
func (e *Executor) buildSystemPrompt(def *AgentDefinition, registry *tools.Registry) string {
	var sb strings.Builder

	// Start with the agent's custom system prompt
	sb.WriteString(def.SystemPrompt)
	sb.WriteString("\n\n")

	// Add current working directory
	cwd, _ := os.Getwd()
	sb.WriteString(fmt.Sprintf("Current working directory: %s\n\n", cwd))

	// Add available tools section
	sb.WriteString("AVAILABLE TOOLS:\n")
	for i, toolDef := range registry.List() {
		sb.WriteString(fmt.Sprintf("%d. %s - %s\n", i+1, toolDef.Name, toolDef.Description))
	}
	sb.WriteString("\n")

	// Add tool calling format
	sb.WriteString("TOOL CALLING FORMAT:\n")
	sb.WriteString("Use this XML format when calling tools:\n")
	sb.WriteString("```xml\n")
	sb.WriteString("<tool_call>\n")
	sb.WriteString("  <id>call_1</id>\n")
	sb.WriteString("  <name>tool_name</name>\n")
	sb.WriteString("  <parameters>\n")
	sb.WriteString("    <param>value</param>\n")
	sb.WriteString("  </parameters>\n")
	sb.WriteString("</tool_call>\n")
	sb.WriteString("```\n\n")

	// Add handoff instructions if enabled
	if def.HandoffTo != "" {
		sb.WriteString("HANDOFF:\n")
		sb.WriteString(fmt.Sprintf("When you need to hand off to another agent, use this format:\n"))
		sb.WriteString("```xml\n")
		sb.WriteString(fmt.Sprintf("<handoff agent=\"%s\" reason=\"Your reason here\">\n", def.HandoffTo))
		sb.WriteString("  <context key=\"key_name\">value</context>\n")
		sb.WriteString("</handoff>\n")
		sb.WriteString("```\n")
	}

	return sb.String()
}

// executeToolCalls executes multiple tool calls
func (e *Executor) executeToolCalls(ctx context.Context, registry *tools.Registry, toolCalls []tools.ToolCall) []ToolExecution {
	results := make([]ToolExecution, len(toolCalls))

	for i, tc := range toolCalls {
		toolResult := registry.Execute(ctx, tc)

		results[i] = ToolExecution{
			ID:     tc.ID,
			Name:   tc.Name,
			Args:   formatArgs(tc.Arguments),
			Result: toolResult.Output,
			Error:  toolResult.Error,
		}
	}

	return results
}

// formatArgs formats tool arguments for display
func formatArgs(args map[string]any) string {
	parts := make([]string, 0, len(args))
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, ", ")
}
