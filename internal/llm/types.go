package llm

import (
	"context"
	"strings"
)

// OpenAI-compatible tool calling types

// OpenAITool represents a tool definition in OpenAI format
type OpenAITool struct {
	Type     string         `json:"type"` // "function"
	Function OpenAIFunction `json:"function"`
}

// OpenAIFunction represents a function definition
type OpenAIFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// OpenAIToolCall represents a tool call from the model
type OpenAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string
	} `json:"function"`
}

// ToolMessage represents a tool result message to send back
type ToolMessage struct {
	Role       string `json:"role"` // "tool"
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id"`
}

// ToolCallResponse contains the model's response with tool calls
type ToolCallResponse struct {
	Content   string           // Text content (may be empty if only tool calls)
	ToolCalls []OpenAIToolCall // Tool calls requested by the model
	Done      bool             // Whether the model is done (no more tool calls)
}

// ToolProvider is an optional interface for providers that support native tool calling
type ToolProvider interface {
	Provider
	// GenerateWithTools sends a request with tool definitions and returns tool calls
	GenerateWithTools(ctx context.Context, messages []Message, tools []OpenAITool) (*ToolCallResponse, error)
	// GenerateStreamWithTools streams a response with tool call support
	GenerateStreamWithTools(ctx context.Context, messages []Message, tools []OpenAITool) (<-chan ToolStreamChunk, error)
}

// ToolStreamChunk represents a streaming chunk that may contain tool calls
type ToolStreamChunk struct {
	Text      string           // Text content delta
	ToolCalls []OpenAIToolCall // Tool calls (accumulated)
	Done      bool             // Whether streaming is complete
	Error     error            // Any error that occurred
}

// ToolRequestMessage is the message format for tool calling API requests.
// Uses *string for Content to allow null values for assistant messages with tool calls.
type ToolRequestMessage struct {
	Role       string           `json:"role"`
	Content    *string          `json:"content"`                // Pointer to allow null
	Name       string           `json:"name,omitempty"`         // Tool name for tool result messages
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

// ConvertMessagesToToolFormat converts internal Message slice to ToolRequestMessage slice
// for use with the OpenAI-compatible tool calling API.
func ConvertMessagesToToolFormat(messages []Message) []ToolRequestMessage {
	result := make([]ToolRequestMessage, 0, len(messages))
	for _, msg := range messages {
		tm := ToolRequestMessage{
			Role:       msg.Role,
			Name:       msg.Name,
			ToolCalls:  msg.ToolCalls,
			ToolCallID: msg.ToolCallID,
		}
		// For assistant messages with tool calls, content should be null if empty
		// For all other messages, content should be set (even if empty string)
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 && msg.Content == "" {
			tm.Content = nil
		} else {
			tm.Content = &msg.Content
		}
		result = append(result, tm)
	}
	return result
}

// ToolCallDelta represents a partial tool call received during streaming
type ToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function,omitempty"`
}

// ToolCallAccumulator accumulates tool call deltas during streaming
type ToolCallAccumulator struct {
	toolCalls map[int]*OpenAIToolCall
}

// NewToolCallAccumulator creates a new accumulator
func NewToolCallAccumulator() *ToolCallAccumulator {
	return &ToolCallAccumulator{
		toolCalls: make(map[int]*OpenAIToolCall),
	}
}

// AddDelta processes a tool call delta and accumulates it
func (a *ToolCallAccumulator) AddDelta(delta ToolCallDelta) {
	tc, exists := a.toolCalls[delta.Index]
	if !exists {
		tcType := delta.Type
		if tcType == "" {
			tcType = "function" // Default to function type
		}
		tc = &OpenAIToolCall{
			ID:   delta.ID,
			Type: tcType,
		}
		tc.Function.Name = delta.Function.Name
		a.toolCalls[delta.Index] = tc
	} else {
		// Update ID and Type if they come in later deltas
		if delta.ID != "" {
			tc.ID = delta.ID
		}
		if delta.Type != "" {
			tc.Type = delta.Type
		}
		if delta.Function.Name != "" {
			tc.Function.Name = delta.Function.Name
		}
	}
	// Accumulate arguments
	tc.Function.Arguments += delta.Function.Arguments
}

// GetToolCalls returns the accumulated tool calls in order
func (a *ToolCallAccumulator) GetToolCalls() []OpenAIToolCall {
	var toolCalls []OpenAIToolCall
	for i := 0; i < len(a.toolCalls); i++ {
		if tc, ok := a.toolCalls[i]; ok {
			toolCalls = append(toolCalls, *tc)
		}
	}
	return toolCalls
}

// ParseSSELine parses a Server-Sent Events line and returns the data payload.
// Returns empty string if line is not a data line or is the [DONE] marker.
func ParseSSELine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "data: ") {
		return ""
	}
	data := strings.TrimPrefix(line, "data: ")
	if data == "[DONE]" {
		return ""
	}
	return data
}
