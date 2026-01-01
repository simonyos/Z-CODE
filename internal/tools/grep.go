package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GrepTool searches for content in files
type GrepTool struct {
	BaseTool
}

// GrepMatch represents a single match result
type GrepMatch struct {
	File    string
	Line    int
	Content string
}

// NewGrepTool creates a new grep content search tool
func NewGrepTool() *GrepTool {
	return &GrepTool{
		BaseTool: BaseTool{
			Def: ToolDefinition{
				Name:        "grep",
				Description: "Search for text or regex patterns in files. Returns matching lines with file paths and line numbers.",
				Parameters: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"pattern": {
							Type:        "string",
							Description: "The text or regex pattern to search for",
						},
						"path": {
							Type:        "string",
							Description: "File or directory to search in (defaults to current directory)",
						},
						"glob": {
							Type:        "string",
							Description: "Optional glob pattern to filter files (e.g., '*.go', '*.ts')",
						},
						"case_insensitive": {
							Type:        "boolean",
							Description: "If true, search is case-insensitive",
						},
					},
					Required: []string{"pattern"},
				},
			},
		},
	}
}

// Execute searches for the pattern in files
func (t *GrepTool) Execute(ctx context.Context, args map[string]any) ToolResult {
	pattern, _ := args["pattern"].(string)
	searchPath, _ := args["path"].(string)
	globPattern, _ := args["glob"].(string)
	caseInsensitive, _ := args["case_insensitive"].(bool)

	if searchPath == "" {
		searchPath = "."
	}

	// Compile regex
	regexPattern := pattern
	if caseInsensitive {
		regexPattern = "(?i)" + pattern
	}

	var usedLiteralFallback bool
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		// Fall back to literal string search
		usedLiteralFallback = true
		escaped := regexp.QuoteMeta(pattern)
		if caseInsensitive {
			escaped = "(?i)" + escaped
		}
		re = regexp.MustCompile(escaped)
	}

	// Get absolute path
	absPath, err := filepath.Abs(searchPath)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("invalid path: %v", err)}
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("path not found: %v", err)}
	}

	var matches []GrepMatch
	var warning string

	if info.IsDir() {
		matches, err = grepDirectory(absPath, re, globPattern)
		// Check if this is just a "skipped files" warning (not a hard error)
		if err != nil && strings.Contains(err.Error(), "skipped") {
			warning = err.Error()
			err = nil
		}
	} else {
		matches, err = grepFile(absPath, re)
	}

	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("search error: %v", err)}
	}

	if len(matches) == 0 {
		msg := "No matches found for pattern: " + pattern
		if usedLiteralFallback {
			msg += " (note: pattern was treated as literal text due to invalid regex syntax)"
		}
		return ToolResult{
			Success: true,
			Output:  msg,
		}
	}

	// Format output
	var sb strings.Builder
	if usedLiteralFallback {
		sb.WriteString("Note: pattern was treated as literal text due to invalid regex syntax\n\n")
	}
	sb.WriteString(fmt.Sprintf("Found %d matches:\n\n", len(matches)))

	maxMatches := 50
	for i, match := range matches {
		if i >= maxMatches {
			sb.WriteString(fmt.Sprintf("\n... and %d more matches", len(matches)-maxMatches))
			break
		}
		// Truncate long lines
		content := match.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		sb.WriteString(fmt.Sprintf("%s:%d: %s\n", match.File, match.Line, content))
	}

	if warning != "" {
		sb.WriteString(fmt.Sprintf("\nNote: %s", warning))
	}

	return ToolResult{
		Success: true,
		Output:  sb.String(),
	}
}

// grepDirResult holds matches and metadata from directory grep
type grepDirResult struct {
	matches      []GrepMatch
	skippedCount int
}

// grepDirectory searches all files in a directory
func grepDirectory(dirPath string, re *regexp.Regexp, globPattern string) ([]GrepMatch, error) {
	result := &grepDirResult{}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.skippedCount++
			return nil // Skip errors but track them
		}

		// Skip hidden directories
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			// Skip common non-code directories
			switch info.Name() {
			case "node_modules", "vendor", "__pycache__", ".git", "dist", "build":
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Skip binary files (simple check)
		if isBinaryFile(info.Name()) {
			return nil
		}

		// Apply glob filter if provided
		if globPattern != "" {
			matched, _ := filepath.Match(globPattern, info.Name())
			if !matched {
				return nil
			}
		}

		// Search this file
		matches, err := grepFile(path, re)
		if err != nil {
			result.skippedCount++
			return nil // Skip files we can't read but track them
		}

		// Convert to relative paths
		for i := range matches {
			rel, err := filepath.Rel(dirPath, matches[i].File)
			if err == nil {
				matches[i].File = rel
			}
		}

		result.matches = append(result.matches, matches...)
		return nil
	})

	// If some paths were skipped, wrap the error with additional info
	if result.skippedCount > 0 && err == nil {
		err = fmt.Errorf("skipped %d inaccessible files", result.skippedCount)
	}

	return result.matches, err
}

// grepFile searches a single file.
// Uses a 1MB buffer to handle files with long lines (e.g., minified JS).
func grepFile(filePath string, re *regexp.Regexp) ([]GrepMatch, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var matches []GrepMatch
	scanner := bufio.NewScanner(file)
	// Increase buffer size to 1MB to handle minified files
	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if re.MatchString(line) {
			matches = append(matches, GrepMatch{
				File:    filePath,
				Line:    lineNum,
				Content: strings.TrimSpace(line),
			})
		}
	}

	if err := scanner.Err(); err != nil {
		// Return partial matches with a note about the error
		return matches, fmt.Errorf("scan incomplete: %w", err)
	}

	return matches, nil
}

// isBinaryFile checks if a file is likely binary based on extension
func isBinaryFile(name string) bool {
	binaryExts := []string{
		".exe", ".dll", ".so", ".dylib", ".bin",
		".png", ".jpg", ".jpeg", ".gif", ".ico", ".webp",
		".pdf", ".doc", ".docx", ".xls", ".xlsx",
		".zip", ".tar", ".gz", ".rar", ".7z",
		".mp3", ".mp4", ".avi", ".mov", ".wav",
		".ttf", ".otf", ".woff", ".woff2",
		".pyc", ".class", ".o", ".a",
	}

	ext := strings.ToLower(filepath.Ext(name))
	for _, binExt := range binaryExts {
		if ext == binExt {
			return true
		}
	}
	return false
}
