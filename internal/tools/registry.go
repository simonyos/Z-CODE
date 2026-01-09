package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/simonyos/Z-CODE/internal/llm"
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

// GetOpenAIToolDefinitions returns tool definitions in OpenAI-compatible format
func (r *Registry) GetOpenAIToolDefinitions() []llm.OpenAITool {
	result := make([]llm.OpenAITool, 0, len(r.tools))
	for _, t := range r.tools {
		def := t.Definition()
		result = append(result, llm.OpenAITool{
			Type: "function",
			Function: llm.OpenAIFunction{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  jsonSchemaToMap(def.Parameters),
			},
		})
	}
	return result
}

// jsonSchemaToMap converts JSONSchema to map for OpenAI API.
//
// Limitations: This function only handles basic JSON Schema features used by
// the built-in tools. The following features are NOT supported:
//   - items (for array types)
//   - additionalProperties
//   - anyOf, oneOf, allOf
//   - $ref
//   - pattern, format
//   - minimum, maximum, minLength, maxLength
//
// If you need these features, extend this function or use a full JSON Schema library.
func jsonSchemaToMap(schema *JSONSchema) map[string]interface{} {
	if schema == nil {
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	result := map[string]interface{}{
		"type": schema.Type,
	}

	if schema.Description != "" {
		result["description"] = schema.Description
	}

	if len(schema.Properties) > 0 {
		props := make(map[string]interface{})
		for name, prop := range schema.Properties {
			props[name] = jsonSchemaToMap(prop)
		}
		result["properties"] = props
	}

	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}

	if len(schema.Enum) > 0 {
		result["enum"] = schema.Enum
	}

	return result
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

// BuildSystemPrompt generates the system prompt for the agent.
// Tool definitions are passed separately via the native tool calling API.
func (r *Registry) BuildSystemPrompt() string {
	cwd, _ := os.Getwd()

	var sb strings.Builder

	// Core identity and capabilities
	sb.WriteString("You are an expert software engineer and coding assistant.\n\n")
	sb.WriteString(fmt.Sprintf("Current working directory: %s\n\n", cwd))

	// Coding best practices
	sb.WriteString("CODING GUIDELINES:\n")
	sb.WriteString("- Write clean, maintainable code following best practices\n")
	sb.WriteString("- Prefer editing existing files over creating new ones\n")
	sb.WriteString("- Use edit_file for surgical changes instead of rewriting entire files\n")
	sb.WriteString("- Read files before modifying them to understand context\n")
	sb.WriteString("- Use glob and grep to explore the codebase before making changes\n")
	sb.WriteString("- Keep changes minimal and focused on the task\n")
	sb.WriteString("- Avoid excessive or redundant comments, logging, or error handling; include appropriate error checks where needed\n")
	sb.WriteString("- Follow the existing code style and patterns in the project\n")
	sb.WriteString("- Be careful with destructive operations (file writes, shell commands)\n\n")

	// Workflow guidance
	sb.WriteString("WORKFLOW:\n")
	sb.WriteString("1. If you need information, use tools first (read_file, glob, grep)\n")
	sb.WriteString("2. You can call multiple tools in parallel when they're independent\n")
	sb.WriteString("3. After getting tool results, continue working or provide your answer\n")
	sb.WriteString("4. Be concise and helpful in your responses\n")

	return sb.String()
}
