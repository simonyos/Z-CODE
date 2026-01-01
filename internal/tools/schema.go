package tools

// JSONSchema represents OpenAI-style function parameters
type JSONSchema struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description,omitempty"`
	Properties  map[string]*JSONSchema `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Enum        []string               `json:"enum,omitempty"`
}

// ToolDefinition is the structured tool definition (like OpenAI)
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  *JSONSchema `json:"parameters"`
}

// ToolCall represents a parsed tool invocation.
// Note: JSON tags are retained for potential future API serialization,
// but tool calls are currently parsed from XML format (see registry.go).
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ToolResult represents the output of a tool execution
type ToolResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}
