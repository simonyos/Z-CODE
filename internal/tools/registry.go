package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Registry manages tool registration and execution
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry
func (r *Registry) Register(tool Tool) {
	def := tool.Definition()
	r.tools[def.Name] = tool
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tool definitions
func (r *Registry) List() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	return defs
}

// Execute runs a tool by name with arguments
func (r *Registry) Execute(ctx context.Context, call ToolCall) ToolResult {
	tool, ok := r.Get(call.Name)
	if !ok {
		return ToolResult{Success: false, Error: fmt.Sprintf("unknown tool: %s", call.Name)}
	}

	if err := tool.Validate(call.Arguments); err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}

	return tool.Execute(ctx, call.Arguments)
}

// BuildSystemPrompt generates the tools section of system prompt
func (r *Registry) BuildSystemPrompt() string {
	cwd, _ := os.Getwd()

	var sb strings.Builder
	sb.WriteString("You are a helpful coding assistant.\n\n")
	sb.WriteString(fmt.Sprintf("Current working directory: %s\n\n", cwd))
	sb.WriteString("You have access to these tools. When you need to use one, respond with ONLY a JSON object:\n\n")
	sb.WriteString("TOOLS:\n")

	for i, def := range r.List() {
		sb.WriteString(fmt.Sprintf("%d. %s - %s\n", i+1, def.Name, def.Description))

		// Generate example usage from schema
		example := r.generateExample(def)
		sb.WriteString(fmt.Sprintf("   Usage: %s\n\n", example))
	}

	sb.WriteString("RULES:\n")
	sb.WriteString("- If you need information, use a tool first\n")
	sb.WriteString("- Output ONLY the JSON object when using a tool, nothing else\n")
	sb.WriteString("- After getting tool results, provide your final answer\n")
	sb.WriteString("- Be concise and helpful\n")

	return sb.String()
}

func (r *Registry) generateExample(def ToolDefinition) string {
	example := map[string]any{"tool": def.Name}
	if def.Parameters != nil && def.Parameters.Properties != nil {
		for name, prop := range def.Parameters.Properties {
			switch prop.Type {
			case "string":
				example[name] = fmt.Sprintf("<%s>", name)
			case "integer", "number":
				example[name] = 0
			case "boolean":
				example[name] = true
			}
		}
	}
	bytes, _ := json.Marshal(example)
	return string(bytes)
}

// ParseToolCall attempts to parse text as a tool call
// It extracts JSON from the response even if there's surrounding text
func ParseToolCall(text string) (*ToolCall, error) {
	text = strings.TrimSpace(text)

	// Try to find JSON object in the text
	startIdx := strings.Index(text, "{")
	if startIdx == -1 {
		return nil, fmt.Errorf("no JSON object found")
	}

	// Find matching closing brace
	depth := 0
	endIdx := -1
	for i := startIdx; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				endIdx = i
				break
			}
		}
		if endIdx != -1 {
			break
		}
	}

	if endIdx == -1 {
		return nil, fmt.Errorf("no complete JSON object found")
	}

	jsonStr := text[startIdx : endIdx+1]

	var raw map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, err
	}

	toolName, ok := raw["tool"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'tool' field")
	}

	delete(raw, "tool")
	return &ToolCall{Name: toolName, Arguments: raw}, nil
}
