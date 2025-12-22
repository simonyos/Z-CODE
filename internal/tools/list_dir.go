package tools

import (
	"context"
	"os"
	"strings"
)

// ListDirTool lists files in a directory
type ListDirTool struct {
	BaseTool
}

// NewListDirTool creates a new list directory tool
func NewListDirTool() *ListDirTool {
	return &ListDirTool{
		BaseTool: BaseTool{
			Def: ToolDefinition{
				Name:        "list_dir",
				Description: "List files and directories at the specified path",
				Parameters: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"path": {
							Type:        "string",
							Description: "The directory path to list (defaults to current directory)",
						},
					},
					Required: []string{},
				},
			},
		},
	}
}

// Execute lists the directory contents
func (t *ListDirTool) Execute(ctx context.Context, args map[string]any) ToolResult {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		path = "."
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}

	var names []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}

	return ToolResult{Success: true, Output: strings.Join(names, "\n")}
}
