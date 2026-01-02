package skills

import (
	"context"
	"strings"

	"github.com/simonyos/Z-CODE/internal/agent"
	"github.com/simonyos/Z-CODE/internal/llm"
	"github.com/simonyos/Z-CODE/internal/tools"
)

// Executor handles skill execution using the base agent
type Executor struct {
	provider  llm.Provider
	confirmFn tools.ConfirmFunc
}

// NewExecutor creates a new skill executor
func NewExecutor(provider llm.Provider, confirmFn tools.ConfirmFunc) *Executor {
	return &Executor{
		provider:  provider,
		confirmFn: confirmFn,
	}
}

// ExecuteResult contains the result of a skill execution
type ExecuteResult struct {
	Response string
	Success  bool
}

// StreamEvent represents a streaming event during skill execution
type StreamEvent struct {
	Type    StreamEventType
	Content string
	Error   error
}

// StreamEventType identifies the type of stream event
type StreamEventType int

const (
	StreamEventText StreamEventType = iota
	StreamEventToolCall
	StreamEventToolResult
	StreamEventDone
	StreamEventError
)

// Execute runs a skill with the given input and returns the result
func (e *Executor) Execute(ctx context.Context, skill *SkillDefinition, userInput string, variables map[string]string) (*ExecuteResult, error) {
	invocation := &SkillInvocation{
		Skill:     skill,
		UserInput: userInput,
		Variables: variables,
	}

	prompt := invocation.Expand()

	// Create a base agent with default settings
	baseAgent := agent.New(e.provider, e.confirmFn)

	result, err := baseAgent.Chat(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return &ExecuteResult{
		Response: result.Response,
		Success:  true,
	}, nil
}

// ExecuteStream runs a skill and streams events back
func (e *Executor) ExecuteStream(ctx context.Context, skill *SkillDefinition, userInput string, variables map[string]string) <-chan StreamEvent {
	events := make(chan StreamEvent, 100)

	go func() {
		defer close(events)

		invocation := &SkillInvocation{
			Skill:     skill,
			UserInput: userInput,
			Variables: variables,
		}

		prompt := invocation.Expand()

		// Create a base agent with default settings
		baseAgent := agent.New(e.provider, e.confirmFn)

		// Use streaming chat
		agentEvents := baseAgent.ChatStream(ctx, prompt)

		for evt := range agentEvents {
			switch evt.Type {
			case "chunk":
				events <- StreamEvent{
					Type:    StreamEventText,
					Content: evt.Text,
				}
			case "tool_start":
				events <- StreamEvent{
					Type:    StreamEventToolCall,
					Content: evt.ToolName + ": " + evt.ToolArgs,
				}
			case "tool_result":
				events <- StreamEvent{
					Type:    StreamEventToolResult,
					Content: evt.ToolResult,
				}
			case "done":
				events <- StreamEvent{
					Type:    StreamEventDone,
					Content: evt.FinalResponse,
				}
			case "error":
				events <- StreamEvent{
					Type:    StreamEventError,
					Error:   evt.Error,
				}
			}
		}
	}()

	return events
}

// ParseSkillCommand parses a skill invocation from user input
// Format: /skill-name arg1 arg2 or /skill-name key=value key2=value2
func ParseSkillCommand(input string) (skillName, userInput string, variables map[string]string) {
	variables = make(map[string]string)

	if !strings.HasPrefix(input, "/") {
		return "", input, variables
	}

	parts := strings.SplitN(input, " ", 2)
	skillName = strings.TrimPrefix(parts[0], "/")

	if len(parts) < 2 {
		return skillName, "", variables
	}

	remaining := parts[1]

	// Try to parse key=value pairs
	words := strings.Fields(remaining)
	var nonVarParts []string

	for _, word := range words {
		if idx := strings.Index(word, "="); idx > 0 {
			key := word[:idx]
			value := word[idx+1:]
			variables[key] = value
		} else {
			nonVarParts = append(nonVarParts, word)
		}
	}

	userInput = strings.Join(nonVarParts, " ")
	return skillName, userInput, variables
}
