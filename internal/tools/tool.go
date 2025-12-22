package tools

import (
	"context"
	"fmt"
)

// Tool is the interface all tools must implement
type Tool interface {
	// Definition returns the structured tool definition
	Definition() ToolDefinition

	// Execute runs the tool with the given arguments
	Execute(ctx context.Context, args map[string]any) ToolResult

	// Validate checks if the arguments are valid
	Validate(args map[string]any) error
}

// BaseTool provides common functionality for tools
type BaseTool struct {
	Def ToolDefinition
}

// Definition returns the tool definition
func (b *BaseTool) Definition() ToolDefinition {
	return b.Def
}

// Validate checks required fields are present
func (b *BaseTool) Validate(args map[string]any) error {
	if b.Def.Parameters == nil {
		return nil
	}
	for _, required := range b.Def.Parameters.Required {
		if _, ok := args[required]; !ok {
			return fmt.Errorf("missing required argument: %s", required)
		}
	}
	return nil
}
