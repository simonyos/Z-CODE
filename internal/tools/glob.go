package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GlobTool searches for files matching a glob pattern
type GlobTool struct {
	BaseTool
}

// NewGlobTool creates a new glob file search tool
func NewGlobTool() *GlobTool {
	return &GlobTool{
		BaseTool: BaseTool{
			Def: ToolDefinition{
				Name:        "glob",
				Description: "Find files matching a glob pattern. Supports patterns like '**/*.go', 'src/**/*.ts', '*.json'. Returns matching file paths.",
				Parameters: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"pattern": {
							Type:        "string",
							Description: "The glob pattern to match files (e.g., '**/*.go', 'src/*.ts')",
						},
						"path": {
							Type:        "string",
							Description: "The directory to search in (defaults to current directory)",
						},
					},
					Required: []string{"pattern"},
				},
			},
		},
	}
}

// Execute searches for files matching the pattern
func (t *GlobTool) Execute(ctx context.Context, args map[string]any) ToolResult {
	pattern, _ := args["pattern"].(string)
	basePath, _ := args["path"].(string)

	if basePath == "" {
		basePath = "."
	}

	// Expand to absolute path
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("invalid path: %v", err)}
	}

	// Check if directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("path not found: %v", err)}
	}
	if !info.IsDir() {
		return ToolResult{Success: false, Error: "path is not a directory"}
	}

	var matches []string

	// Handle ** pattern (recursive)
	if strings.Contains(pattern, "**") {
		matches, err = globRecursive(absPath, pattern)
	} else {
		// Simple glob
		fullPattern := filepath.Join(absPath, pattern)
		matches, err = filepath.Glob(fullPattern)
	}

	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("glob error: %v", err)}
	}

	// Sort matches
	sort.Strings(matches)

	// Convert to relative paths for cleaner output
	relMatches := make([]string, 0, len(matches))
	for _, match := range matches {
		rel, err := filepath.Rel(absPath, match)
		if err != nil {
			rel = match
		}
		relMatches = append(relMatches, rel)
	}

	if len(relMatches) == 0 {
		return ToolResult{
			Success: true,
			Output:  "No files found matching pattern: " + pattern,
		}
	}

	// Limit output if too many matches
	maxMatches := 100
	output := strings.Join(relMatches, "\n")
	if len(relMatches) > maxMatches {
		output = strings.Join(relMatches[:maxMatches], "\n")
		output += fmt.Sprintf("\n... and %d more files", len(relMatches)-maxMatches)
	}

	return ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Found %d files:\n%s", len(relMatches), output),
	}
}

// globRecursive handles ** patterns for recursive matching
func globRecursive(basePath, pattern string) ([]string, error) {
	var matches []string

	// Split pattern by **
	parts := strings.SplitN(pattern, "**", 2)
	prefix := strings.TrimSuffix(parts[0], string(filepath.Separator))
	suffix := ""
	if len(parts) > 1 {
		suffix = strings.TrimPrefix(parts[1], string(filepath.Separator))
	}

	// Start path
	startPath := basePath
	if prefix != "" {
		startPath = filepath.Join(basePath, prefix)
	}

	// Walk the directory tree
	// Note: Permission errors and other file access issues are silently skipped
	// to provide best-effort results rather than failing on inaccessible files
	err := filepath.Walk(startPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors (permission denied, broken symlinks, etc.)
		}

		// Skip hidden directories
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		// Check if file matches the suffix pattern
		if suffix == "" {
			matches = append(matches, path)
			return nil
		}

		// Match against suffix pattern
		matched, err := filepath.Match(suffix, info.Name())
		if err != nil {
			return nil
		}

		// Also try matching with the relative path from startPath
		relPath, _ := filepath.Rel(startPath, path)
		matchedPath, _ := filepath.Match(suffix, relPath)

		if matched || matchedPath {
			matches = append(matches, path)
		}

		return nil
	})

	return matches, err
}
