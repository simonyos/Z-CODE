package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseToolCall(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantArgs  map[string]any
		wantError bool
	}{
		{
			name:     "simple tool call",
			input:    `{"tool": "read_file", "path": "/tmp/test.txt"}`,
			wantName: "read_file",
			wantArgs: map[string]any{"path": "/tmp/test.txt"},
		},
		{
			name:     "tool call with surrounding text",
			input:    `Let me read that file for you. {"tool": "read_file", "path": "/tmp/test.txt"} Done.`,
			wantName: "read_file",
			wantArgs: map[string]any{"path": "/tmp/test.txt"},
		},
		{
			name:     "tool call with multiple args",
			input:    `{"tool": "write_file", "path": "/tmp/out.txt", "content": "hello"}`,
			wantName: "write_file",
			wantArgs: map[string]any{"path": "/tmp/out.txt", "content": "hello"},
		},
		{
			name:      "no JSON",
			input:     "Just a plain text response",
			wantError: true,
		},
		{
			name:      "JSON without tool field",
			input:     `{"path": "/tmp/test.txt"}`,
			wantError: true,
		},
		{
			name:      "incomplete JSON",
			input:     `{"tool": "read_file", "path":`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			call, err := ParseToolCall(tt.input)

			if tt.wantError {
				if err == nil {
					t.Error("ParseToolCall() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseToolCall() error = %v", err)
			}

			if call.Name != tt.wantName {
				t.Errorf("ParseToolCall() name = %q, want %q", call.Name, tt.wantName)
			}

			for k, v := range tt.wantArgs {
				if call.Arguments[k] != v {
					t.Errorf("ParseToolCall() args[%q] = %v, want %v", k, call.Arguments[k], v)
				}
			}
		})
	}
}

func TestBaseTool_Validate(t *testing.T) {
	tool := &BaseTool{
		Def: ToolDefinition{
			Name: "test_tool",
			Parameters: &JSONSchema{
				Type:     "object",
				Required: []string{"path", "content"},
			},
		},
	}

	tests := []struct {
		name      string
		args      map[string]any
		wantError bool
	}{
		{
			name:      "all required present",
			args:      map[string]any{"path": "/tmp", "content": "test"},
			wantError: false,
		},
		{
			name:      "missing required",
			args:      map[string]any{"path": "/tmp"},
			wantError: true,
		},
		{
			name:      "empty args",
			args:      map[string]any{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.Validate(tt.args)
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError = %v", err, tt.wantError)
			}
		})
	}
}

func TestBaseTool_ValidateNoParams(t *testing.T) {
	tool := &BaseTool{
		Def: ToolDefinition{
			Name:       "no_params_tool",
			Parameters: nil,
		},
	}

	err := tool.Validate(map[string]any{})
	if err != nil {
		t.Errorf("Validate() with nil params should not error, got %v", err)
	}
}

func TestReadFileTool(t *testing.T) {
	// Create a temp file
	tmpFile, err := os.CreateTemp("", "zcode-test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "Hello, Z-Code!"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	tool := NewReadFileTool()
	ctx := context.Background()

	// Test successful read
	result := tool.Execute(ctx, map[string]any{"path": tmpFile.Name()})
	if !result.Success {
		t.Errorf("Execute() success = false, error = %s", result.Error)
	}
	if result.Output != content {
		t.Errorf("Execute() output = %q, want %q", result.Output, content)
	}

	// Test reading non-existent file
	result = tool.Execute(ctx, map[string]any{"path": "/nonexistent/file.txt"})
	if result.Success {
		t.Error("Execute() on non-existent file should fail")
	}
	if result.Error == "" {
		t.Error("Execute() on non-existent file should have error message")
	}
}

func TestReadFileTool_Definition(t *testing.T) {
	tool := NewReadFileTool()
	def := tool.Definition()

	if def.Name != "read_file" {
		t.Errorf("Definition().Name = %q, want %q", def.Name, "read_file")
	}
	if def.Parameters == nil {
		t.Fatal("Definition().Parameters is nil")
	}
	if _, ok := def.Parameters.Properties["path"]; !ok {
		t.Error("Definition().Parameters should have 'path' property")
	}
}

func TestListDirTool(t *testing.T) {
	// Create a temp directory with files
	tmpDir, err := os.MkdirTemp("", "zcode-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create files and subdirectory
	if err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file1.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create file2.go: %v", err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	tool := NewListDirTool()
	ctx := context.Background()

	// Test listing directory
	result := tool.Execute(ctx, map[string]any{"path": tmpDir})
	if !result.Success {
		t.Errorf("Execute() success = false, error = %s", result.Error)
	}

	// Check that all entries are present
	if !strings.Contains(result.Output, "file1.txt") {
		t.Error("Execute() output should contain 'file1.txt'")
	}
	if !strings.Contains(result.Output, "file2.go") {
		t.Error("Execute() output should contain 'file2.go'")
	}
	if !strings.Contains(result.Output, "subdir/") {
		t.Error("Execute() output should contain 'subdir/' (with trailing slash)")
	}

	// Test with empty path (defaults to current directory)
	result = tool.Execute(ctx, map[string]any{})
	if !result.Success {
		t.Errorf("Execute() with empty path should use current directory, got error: %s", result.Error)
	}

	// Test non-existent directory
	result = tool.Execute(ctx, map[string]any{"path": "/nonexistent/dir"})
	if result.Success {
		t.Error("Execute() on non-existent dir should fail")
	}
}

func TestWriteFileTool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcode-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Always confirm
	confirmFn := func(prompt string) bool { return true }
	tool := NewWriteFileTool(confirmFn)
	ctx := context.Background()

	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello, Z-Code!"

	// Test writing file
	result := tool.Execute(ctx, map[string]any{
		"path":    testFile,
		"content": content,
	})
	if !result.Success {
		t.Errorf("Execute() success = false, error = %s", result.Error)
	}

	// Verify file contents
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content = %q, want %q", string(data), content)
	}

	// Test denied confirmation
	denyFn := func(prompt string) bool { return false }
	denyTool := NewWriteFileTool(denyFn)
	result = denyTool.Execute(ctx, map[string]any{
		"path":    filepath.Join(tmpDir, "denied.txt"),
		"content": "should not write",
	})
	if result.Success {
		t.Error("Execute() should fail when confirmation is denied")
	}
	if !strings.Contains(result.Error, "denied") {
		t.Errorf("Execute() error should mention denial, got: %s", result.Error)
	}
}

func TestWriteFileTool_NoConfirm(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcode-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// No confirmation function (nil)
	tool := NewWriteFileTool(nil)
	ctx := context.Background()

	testFile := filepath.Join(tmpDir, "test.txt")
	result := tool.Execute(ctx, map[string]any{
		"path":    testFile,
		"content": "test content",
	})
	if !result.Success {
		t.Errorf("Execute() with nil confirmFn should succeed, got error: %s", result.Error)
	}
}

func TestBashTool(t *testing.T) {
	// Always confirm
	confirmFn := func(prompt string) bool { return true }
	tool := NewBashTool(confirmFn)
	ctx := context.Background()

	// Test simple command
	result := tool.Execute(ctx, map[string]any{"command": "echo 'hello world'"})
	if !result.Success {
		t.Errorf("Execute() success = false, error = %s", result.Error)
	}
	if !strings.Contains(result.Output, "hello world") {
		t.Errorf("Execute() output = %q, want to contain 'hello world'", result.Output)
	}

	// Test command with exit code
	result = tool.Execute(ctx, map[string]any{"command": "exit 1"})
	if result.Success {
		t.Error("Execute() with exit 1 should fail")
	}

	// Test denied confirmation
	denyFn := func(prompt string) bool { return false }
	denyTool := NewBashTool(denyFn)
	result = denyTool.Execute(ctx, map[string]any{"command": "echo test"})
	if result.Success {
		t.Error("Execute() should fail when confirmation is denied")
	}
}

func TestBashTool_NoOutput(t *testing.T) {
	confirmFn := func(prompt string) bool { return true }
	tool := NewBashTool(confirmFn)
	ctx := context.Background()

	// Command with no output
	result := tool.Execute(ctx, map[string]any{"command": "true"})
	if !result.Success {
		t.Errorf("Execute() success = false, error = %s", result.Error)
	}
	if result.Output != "(no output)" {
		t.Errorf("Execute() output = %q, want %q", result.Output, "(no output)")
	}
}

func TestRegistry(t *testing.T) {
	reg := NewRegistry()

	// Register tools
	reg.Register(NewReadFileTool())
	reg.Register(NewListDirTool())

	// Test Get
	tool, ok := reg.Get("read_file")
	if !ok {
		t.Error("Get() read_file should return true")
	}
	if tool == nil {
		t.Error("Get() read_file should return tool")
	}

	// Test Get non-existent
	_, ok = reg.Get("nonexistent")
	if ok {
		t.Error("Get() nonexistent should return false")
	}

	// Test List
	defs := reg.List()
	if len(defs) != 2 {
		t.Errorf("List() len = %d, want 2", len(defs))
	}
}

func TestRegistry_Execute(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewListDirTool())
	ctx := context.Background()

	// Test successful execution
	result := reg.Execute(ctx, ToolCall{
		Name:      "list_dir",
		Arguments: map[string]any{"path": "."},
	})
	if !result.Success {
		t.Errorf("Execute() success = false, error = %s", result.Error)
	}

	// Test unknown tool
	result = reg.Execute(ctx, ToolCall{
		Name:      "unknown_tool",
		Arguments: map[string]any{},
	})
	if result.Success {
		t.Error("Execute() unknown tool should fail")
	}
	if !strings.Contains(result.Error, "unknown tool") {
		t.Errorf("Execute() error = %q, should contain 'unknown tool'", result.Error)
	}
}

func TestRegistry_BuildSystemPrompt(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewReadFileTool())

	prompt := reg.BuildSystemPrompt()

	// Check that prompt contains expected elements
	if !strings.Contains(prompt, "read_file") {
		t.Error("BuildSystemPrompt() should contain 'read_file'")
	}
	if !strings.Contains(prompt, "TOOLS:") {
		t.Error("BuildSystemPrompt() should contain 'TOOLS:'")
	}
	if !strings.Contains(prompt, "RULES:") {
		t.Error("BuildSystemPrompt() should contain 'RULES:'")
	}
}
