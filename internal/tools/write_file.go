package tools

import (
	"context"
	"fmt"
	"os"
)

// ConfirmFunc is a function that asks for user confirmation
type ConfirmFunc func(prompt string) bool

// WriteFileTool writes content to a file
type WriteFileTool struct {
	BaseTool
	ConfirmFn ConfirmFunc
}

// NewWriteFileTool creates a new write file tool
func NewWriteFileTool(confirmFn ConfirmFunc) *WriteFileTool {
	return &WriteFileTool{
		ConfirmFn: confirmFn,
		BaseTool: BaseTool{
			Def: ToolDefinition{
				Name:        "write_file",
				Description: "Write content to a file at the specified path",
				Parameters: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"path": {
							Type:        "string",
							Description: "The path to the file to write",
						},
						"content": {
							Type:        "string",
							Description: "The content to write to the file",
						},
					},
					Required: []string{"path", "content"},
				},
			},
		},
	}
}

// Execute writes content to the file
func (t *WriteFileTool) Execute(ctx context.Context, args map[string]any) ToolResult {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)

	// Ask for confirmation if a confirm function is provided
	if t.ConfirmFn != nil {
		prompt := fmt.Sprintf("Write to file: %s (%d bytes)", path, len(content))
		if !t.ConfirmFn(prompt) {
			return ToolResult{Success: false, Error: "user denied write permission"}
		}
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}

	return ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path),
	}
}
