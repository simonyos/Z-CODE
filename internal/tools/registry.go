package tools

import (
	"context"
	"fmt"

	"github.com/simonyos/Z-CODE/internal/llm"
	"github.com/simonyos/Z-CODE/internal/prompts"
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
// Uses the new Cline-style prompt system with modular components.
func (r *Registry) BuildSystemPrompt() string {
	return prompts.BuildSystemPrompt()
}

// BuildSystemPromptWithRules generates the system prompt with custom user rules.
func (r *Registry) BuildSystemPromptWithRules(customRules string) string {
	return prompts.BuildSystemPromptWithRules(customRules)
}
