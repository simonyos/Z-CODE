package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// EditTool performs surgical string replacement in files
type EditTool struct {
	BaseTool
	ConfirmFn ConfirmFunc
}

// NewEditTool creates a new edit file tool
func NewEditTool(confirmFn ConfirmFunc) *EditTool {
	return &EditTool{
		ConfirmFn: confirmFn,
		BaseTool: BaseTool{
			Def: ToolDefinition{
				Name:        "edit_file",
				Description: "Make a surgical text replacement in a file. The old_string must match exactly and be unique in the file. Use this instead of write_file for modifying existing files.",
				Parameters: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"path": {
							Type:        "string",
							Description: "The path to the file to edit",
						},
						"old_string": {
							Type:        "string",
							Description: "The exact text to find and replace (must be unique in file)",
						},
						"new_string": {
							Type:        "string",
							Description: "The text to replace old_string with",
						},
					},
					Required: []string{"path", "old_string", "new_string"},
				},
			},
		},
	}
}

// Execute performs the surgical edit
func (t *EditTool) Execute(ctx context.Context, args map[string]any) ToolResult {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return ToolResult{Success: false, Error: "missing or invalid 'path' parameter"}
	}
	oldString, ok := args["old_string"].(string)
	if !ok {
		return ToolResult{Success: false, Error: "missing or invalid 'old_string' parameter"}
	}
	newString, ok := args["new_string"].(string)
	if !ok {
		return ToolResult{Success: false, Error: "missing or invalid 'new_string' parameter"}
	}

	// Get file info to preserve permissions
	fileInfo, err := os.Stat(path)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("failed to stat file: %v", err)}
	}
	fileMode := fileInfo.Mode()

	// Read the file
	content, err := os.ReadFile(path)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("failed to read file: %v", err)}
	}

	fileContent := string(content)

	// Check if old_string exists in file
	count := strings.Count(fileContent, oldString)
	if count == 0 {
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("old_string not found in file. Make sure you're using the exact text from the file."),
		}
	}

	// Check if old_string is unique
	if count > 1 {
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("old_string appears %d times in file. It must be unique. Add more surrounding context to make it unique.", count),
		}
	}

	// Check if old_string equals new_string
	if oldString == newString {
		return ToolResult{
			Success: false,
			Error:   "old_string and new_string are identical. No changes needed.",
		}
	}

	// Ask for confirmation if a confirm function is provided
	if t.ConfirmFn != nil {
		// Create a simple diff preview
		preview := createDiffPreview(oldString, newString)
		prompt := fmt.Sprintf("Edit file %s:\n%s", path, preview)
		if !t.ConfirmFn(prompt) {
			return ToolResult{Success: false, Error: "user denied edit permission"}
		}
	}

	// Perform the replacement
	newContent := strings.Replace(fileContent, oldString, newString, 1)

	// Write back to file with original permissions
	err = os.WriteFile(path, []byte(newContent), fileMode)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("failed to write file: %v", err)}
	}

	// Calculate lines changed
	oldLines := strings.Count(oldString, "\n") + 1
	newLines := strings.Count(newString, "\n") + 1

	return ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Successfully edited %s: replaced %d lines with %d lines", path, oldLines, newLines),
	}
}

// createDiffPreview creates a simple diff-like preview
func createDiffPreview(oldString, newString string) string {
	var sb strings.Builder

	oldLines := strings.Split(oldString, "\n")
	newLines := strings.Split(newString, "\n")

	// Show what's being removed (first few lines)
	maxLines := 5
	sb.WriteString("- ")
	for i, line := range oldLines {
		if i >= maxLines {
			sb.WriteString(fmt.Sprintf("\n  ... (%d more lines)", len(oldLines)-maxLines))
			break
		}
		if i > 0 {
			sb.WriteString("\n- ")
		}
		sb.WriteString(line)
	}

	sb.WriteString("\n+ ")
	for i, line := range newLines {
		if i >= maxLines {
			sb.WriteString(fmt.Sprintf("\n  ... (%d more lines)", len(newLines)-maxLines))
			break
		}
		if i > 0 {
			sb.WriteString("\n+ ")
		}
		sb.WriteString(line)
	}

	return sb.String()
}
