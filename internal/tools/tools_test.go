package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	// Note: Tool definitions are now passed via native tool calling API, not in the system prompt
	if !strings.Contains(prompt, "CODING GUIDELINES:") {
		t.Error("BuildSystemPrompt() should contain 'CODING GUIDELINES:'")
	}
	if !strings.Contains(prompt, "WORKFLOW:") {
		t.Error("BuildSystemPrompt() should contain 'WORKFLOW:'")
	}
}

func TestEditTool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcode-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Always confirm
	confirmFn := func(prompt string) bool { return true }
	tool := NewEditTool(confirmFn)
	ctx := context.Background()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.go")
	originalContent := `package main

func main() {
	fmt.Println("Hello")
}
`
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test successful edit
	result := tool.Execute(ctx, map[string]any{
		"path":       testFile,
		"old_string": `fmt.Println("Hello")`,
		"new_string": `fmt.Println("Hello, World!")`,
	})
	if !result.Success {
		t.Errorf("Execute() success = false, error = %s", result.Error)
	}

	// Verify file was modified
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !strings.Contains(string(data), `fmt.Println("Hello, World!")`) {
		t.Errorf("file should contain new string, got: %s", string(data))
	}

	// Test old_string not found
	result = tool.Execute(ctx, map[string]any{
		"path":       testFile,
		"old_string": "nonexistent string",
		"new_string": "replacement",
	})
	if result.Success {
		t.Error("Execute() should fail when old_string not found")
	}
	if !strings.Contains(result.Error, "not found") {
		t.Errorf("error should mention 'not found', got: %s", result.Error)
	}

	// Test denied confirmation
	denyFn := func(prompt string) bool { return false }
	denyTool := NewEditTool(denyFn)
	result = denyTool.Execute(ctx, map[string]any{
		"path":       testFile,
		"old_string": "Hello",
		"new_string": "Hi",
	})
	if result.Success {
		t.Error("Execute() should fail when confirmation is denied")
	}
}

func TestEditTool_ContentPreservation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcode-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	confirmFn := func(prompt string) bool { return true }
	tool := NewEditTool(confirmFn)
	ctx := context.Background()

	// Create a test file with content before and after the target string
	testFile := filepath.Join(tmpDir, "preserve.go")
	originalContent := `package main

import "fmt"

// This is the first section
func first() {
	fmt.Println("first")
}

// Target function to modify
func target() {
	fmt.Println("original")
}

// This is the last section
func last() {
	fmt.Println("last")
}
`
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Edit just the target function
	result := tool.Execute(ctx, map[string]any{
		"path":       testFile,
		"old_string": `fmt.Println("original")`,
		"new_string": `fmt.Println("modified")`,
	})
	if !result.Success {
		t.Errorf("Execute() success = false, error = %s", result.Error)
	}

	// Read back and verify all content is preserved
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	content := string(data)

	// Check that unchanged sections are preserved
	if !strings.Contains(content, `fmt.Println("first")`) {
		t.Error("first function should be preserved")
	}
	if !strings.Contains(content, `fmt.Println("last")`) {
		t.Error("last function should be preserved")
	}
	if !strings.Contains(content, "// This is the first section") {
		t.Error("first comment should be preserved")
	}
	if !strings.Contains(content, "// This is the last section") {
		t.Error("last comment should be preserved")
	}
	if !strings.Contains(content, `fmt.Println("modified")`) {
		t.Error("target function should be modified")
	}
	if strings.Contains(content, `fmt.Println("original")`) {
		t.Error("original text should be replaced")
	}
}

func TestEditTool_MissingParameters(t *testing.T) {
	confirmFn := func(prompt string) bool { return true }
	tool := NewEditTool(confirmFn)
	ctx := context.Background()

	// Test missing path
	result := tool.Execute(ctx, map[string]any{
		"old_string": "foo",
		"new_string": "bar",
	})
	if result.Success {
		t.Error("Execute() should fail when path is missing")
	}
	if !strings.Contains(result.Error, "path") {
		t.Errorf("error should mention 'path', got: %s", result.Error)
	}

	// Test missing old_string
	result = tool.Execute(ctx, map[string]any{
		"path":       "/tmp/test.txt",
		"new_string": "bar",
	})
	if result.Success {
		t.Error("Execute() should fail when old_string is missing")
	}
	if !strings.Contains(result.Error, "old_string") {
		t.Errorf("error should mention 'old_string', got: %s", result.Error)
	}

	// Test missing new_string
	result = tool.Execute(ctx, map[string]any{
		"path":       "/tmp/test.txt",
		"old_string": "foo",
	})
	if result.Success {
		t.Error("Execute() should fail when new_string is missing")
	}
	if !strings.Contains(result.Error, "new_string") {
		t.Errorf("error should mention 'new_string', got: %s", result.Error)
	}
}

func TestEditTool_NonUnique(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcode-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	confirmFn := func(prompt string) bool { return true }
	tool := NewEditTool(confirmFn)
	ctx := context.Background()

	// Create file with duplicate strings
	testFile := filepath.Join(tmpDir, "dup.go")
	content := `func test() {
	hello()
	hello()
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test that non-unique old_string fails
	result := tool.Execute(ctx, map[string]any{
		"path":       testFile,
		"old_string": "hello()",
		"new_string": "world()",
	})
	if result.Success {
		t.Error("Execute() should fail when old_string is not unique")
	}
	if !strings.Contains(result.Error, "not unique") && !strings.Contains(result.Error, "2 times") {
		t.Errorf("error should mention non-uniqueness, got: %s", result.Error)
	}
}

func TestGlobTool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcode-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	if err := os.WriteFile(filepath.Join(tmpDir, "file1.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create file1.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create file2.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to create test.txt: %v", err)
	}

	// Create subdirectory with files
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.go"), []byte("package sub"), 0644); err != nil {
		t.Fatalf("failed to create nested.go: %v", err)
	}

	tool := NewGlobTool()
	ctx := context.Background()

	// Test simple glob pattern
	result := tool.Execute(ctx, map[string]any{
		"pattern": "*.go",
		"path":    tmpDir,
	})
	if !result.Success {
		t.Errorf("Execute() success = false, error = %s", result.Error)
	}
	if !strings.Contains(result.Output, "file1.go") {
		t.Errorf("output should contain file1.go, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "file2.go") {
		t.Errorf("output should contain file2.go, got: %s", result.Output)
	}
	if strings.Contains(result.Output, "test.txt") {
		t.Error("output should not contain test.txt for *.go pattern")
	}

	// Test recursive pattern
	result = tool.Execute(ctx, map[string]any{
		"pattern": "**/*.go",
		"path":    tmpDir,
	})
	if !result.Success {
		t.Errorf("Execute() success = false, error = %s", result.Error)
	}
	if !strings.Contains(result.Output, "nested.go") {
		t.Errorf("recursive pattern should find nested.go, got: %s", result.Output)
	}

	// Test no matches
	result = tool.Execute(ctx, map[string]any{
		"pattern": "*.xyz",
		"path":    tmpDir,
	})
	if !result.Success {
		t.Errorf("Execute() with no matches should succeed, error = %s", result.Error)
	}
	if !strings.Contains(result.Output, "No files") {
		t.Errorf("output should indicate no matches, got: %s", result.Output)
	}
}

func TestGrepTool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcode-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files with content
	file1 := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(file1, []byte(`package main

func main() {
	fmt.Println("Hello World")
}
`), 0644); err != nil {
		t.Fatalf("failed to create main.go: %v", err)
	}

	file2 := filepath.Join(tmpDir, "util.go")
	if err := os.WriteFile(file2, []byte(`package main

func helper() {
	fmt.Println("Helper function")
}
`), 0644); err != nil {
		t.Fatalf("failed to create util.go: %v", err)
	}

	tool := NewGrepTool()
	ctx := context.Background()

	// Test simple pattern
	result := tool.Execute(ctx, map[string]any{
		"pattern": "Println",
		"path":    tmpDir,
	})
	if !result.Success {
		t.Errorf("Execute() success = false, error = %s", result.Error)
	}
	if !strings.Contains(result.Output, "main.go") {
		t.Errorf("output should contain main.go, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "util.go") {
		t.Errorf("output should contain util.go, got: %s", result.Output)
	}

	// Test with glob filter
	result = tool.Execute(ctx, map[string]any{
		"pattern": "Println",
		"path":    tmpDir,
		"glob":    "main.go",
	})
	if !result.Success {
		t.Errorf("Execute() success = false, error = %s", result.Error)
	}
	if !strings.Contains(result.Output, "main.go") {
		t.Errorf("output should contain main.go, got: %s", result.Output)
	}
	if strings.Contains(result.Output, "util.go") {
		t.Error("output should not contain util.go when filtering by main.go")
	}

	// Test no matches
	result = tool.Execute(ctx, map[string]any{
		"pattern": "nonexistent_pattern",
		"path":    tmpDir,
	})
	if !result.Success {
		t.Errorf("Execute() with no matches should succeed, error = %s", result.Error)
	}
	if !strings.Contains(result.Output, "No matches") {
		t.Errorf("output should indicate no matches, got: %s", result.Output)
	}

	// Test regex pattern
	result = tool.Execute(ctx, map[string]any{
		"pattern": "func\\s+\\w+",
		"path":    tmpDir,
	})
	if !result.Success {
		t.Errorf("Execute() with regex should succeed, error = %s", result.Error)
	}
	if !strings.Contains(result.Output, "func main") || !strings.Contains(result.Output, "func helper") {
		t.Errorf("output should contain function matches, got: %s", result.Output)
	}

	// Test case insensitive search
	result = tool.Execute(ctx, map[string]any{
		"pattern":          "hello",
		"path":             tmpDir,
		"case_insensitive": true,
	})
	if !result.Success {
		t.Errorf("Execute() case insensitive should succeed, error = %s", result.Error)
	}
	if !strings.Contains(result.Output, "Hello World") {
		t.Errorf("case insensitive should match Hello, got: %s", result.Output)
	}
}

func TestGrepTool_SingleFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcode-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("line one\nline two\nline three\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := NewGrepTool()
	ctx := context.Background()

	// Test grep on single file
	result := tool.Execute(ctx, map[string]any{
		"pattern": "two",
		"path":    testFile,
	})
	if !result.Success {
		t.Errorf("Execute() success = false, error = %s", result.Error)
	}
	if !strings.Contains(result.Output, "line two") {
		t.Errorf("output should contain 'line two', got: %s", result.Output)
	}
	if !strings.Contains(result.Output, ":2:") {
		t.Errorf("output should contain line number ':2:', got: %s", result.Output)
	}
}
