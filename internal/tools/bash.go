package tools

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// BashTool executes shell commands
type BashTool struct {
	BaseTool
	ConfirmFn ConfirmFunc
	Timeout   time.Duration
}

// NewBashTool creates a new bash command tool
func NewBashTool(confirmFn ConfirmFunc) *BashTool {
	return &BashTool{
		ConfirmFn: confirmFn,
		Timeout:   30 * time.Second,
		BaseTool: BaseTool{
			Def: ToolDefinition{
				Name:        "run_command",
				Description: "Execute a shell command and return the output",
				Parameters: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"command": {
							Type:        "string",
							Description: "The shell command to execute",
						},
					},
					Required: []string{"command"},
				},
			},
		},
	}
}

// Execute runs the shell command
func (t *BashTool) Execute(ctx context.Context, args map[string]any) ToolResult {
	command, _ := args["command"].(string)

	// Ask for confirmation if a confirm function is provided
	if t.ConfirmFn != nil {
		prompt := fmt.Sprintf("Run command: %s", command)
		if !t.ConfirmFn(prompt) {
			return ToolResult{Success: false, Error: "user denied command execution"}
		}
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()

	if execCtx.Err() == context.DeadlineExceeded {
		return ToolResult{Success: false, Error: "command timed out"}
	}

	if err != nil {
		return ToolResult{
			Success: false,
			Output:  string(output),
			Error:   err.Error(),
		}
	}

	result := string(output)
	if result == "" {
		result = "(no output)"
	}

	return ToolResult{Success: true, Output: result}
}
