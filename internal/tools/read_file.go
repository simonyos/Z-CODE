package tools

import (
	"context"
	"os"
)

// ReadFileTool reads the contents of a file
type ReadFileTool struct {
	BaseTool
}

// NewReadFileTool creates a new read file tool
func NewReadFileTool() *ReadFileTool {
	return &ReadFileTool{
		BaseTool: BaseTool{
			Def: ToolDefinition{
				Name:        "read_file",
				Description: "Read the contents of a file at the specified path",
				Parameters: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"path": {
							Type:        "string",
							Description: "The path to the file to read",
						},
					},
					Required: []string{"path"},
				},
			},
		},
	}
}

// Execute reads the file and returns its contents
func (t *ReadFileTool) Execute(ctx context.Context, args map[string]any) ToolResult {
	path, _ := args["path"].(string)

	content, err := os.ReadFile(path)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}

	return ToolResult{Success: true, Output: string(content)}
}
