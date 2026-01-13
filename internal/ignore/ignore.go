// Package ignore provides .zcodeignore pattern matching for Z-CODE
// Similar to .gitignore but for blocking tool access to certain paths
package ignore

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Matcher checks if paths should be ignored based on .zcodeignore patterns
type Matcher struct {
	patterns  []pattern
	root      string
	statCache map[string]bool // Cache for isDir lookups to avoid repeated os.Stat calls
}

type pattern struct {
	pattern  string
	negation bool // patterns starting with ! are negations
	dirOnly  bool // patterns ending with / only match directories
}

// NewMatcher creates a new ignore matcher for the given root directory
// It looks for .zcodeignore in the root and all parent directories
func NewMatcher(root string) (*Matcher, error) {
	m := &Matcher{
		root:      root,
		patterns:  []pattern{},
		statCache: make(map[string]bool),
	}

	// Load patterns from .zcodeignore files (from root up to filesystem root)
	dir := root
	for {
		ignoreFile := filepath.Join(dir, ".zcodeignore")
		if err := m.loadFile(ignoreFile); err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached filesystem root
		}
		dir = parent
	}

	// Add default patterns (always ignored)
	m.addDefaultPatterns()

	return m, nil
}

// loadFile loads patterns from a single .zcodeignore file
func (m *Matcher) loadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		m.addPattern(line)
	}

	return scanner.Err()
}

// addPattern adds a single pattern to the matcher
func (m *Matcher) addPattern(line string) {
	p := pattern{pattern: line}

	// Check for negation
	if strings.HasPrefix(line, "!") {
		p.negation = true
		p.pattern = strings.TrimPrefix(line, "!")
	}

	// Check for directory-only match
	if strings.HasSuffix(p.pattern, "/") {
		p.dirOnly = true
		p.pattern = strings.TrimSuffix(p.pattern, "/")
	}

	m.patterns = append(m.patterns, p)
}

// addDefaultPatterns adds patterns that are always ignored
func (m *Matcher) addDefaultPatterns() {
	defaults := []string{
		".git/",
		".svn/",
		".hg/",
		"node_modules/",
		"__pycache__/",
		".env",
		".env.local",
		".env.*.local",
		"*.pyc",
		".DS_Store",
		"Thumbs.db",
		// Secrets and credentials
		"*.pem",
		"*.key",
		"*_rsa",
		"*_dsa",
		"*_ecdsa",
		"*_ed25519",
		"*.p12",
		"*.pfx",
		"credentials.json",
		"service-account*.json",
	}

	for _, d := range defaults {
		m.addPattern(d)
	}
}

// ShouldIgnore checks if a path should be ignored
// The path should be relative to the root directory
func (m *Matcher) ShouldIgnore(path string) bool {
	// Normalize path separators
	path = filepath.ToSlash(path)

	// Check if it's a directory (with caching for performance)
	isDir := m.isDirectory(path)

	// Check patterns in order (later patterns override earlier ones)
	ignored := false
	for _, p := range m.patterns {
		if m.matchPattern(p, path, isDir) {
			ignored = !p.negation
		}
	}

	return ignored
}

// isDirectory checks if a path is a directory, with caching
func (m *Matcher) isDirectory(path string) bool {
	// Check cache first
	if isDir, ok := m.statCache[path]; ok {
		return isDir
	}

	// Stat the file
	fullPath := filepath.Join(m.root, path)
	info, err := os.Stat(fullPath)
	isDir := err == nil && info.IsDir()

	// Cache the result
	m.statCache[path] = isDir

	return isDir
}

// ClearCache clears the stat cache (useful after file operations)
func (m *Matcher) ClearCache() {
	m.statCache = make(map[string]bool)
}

// matchPattern checks if a path matches a single pattern
func (m *Matcher) matchPattern(p pattern, path string, isDir bool) bool {
	// Directory-only patterns don't match files
	if p.dirOnly && !isDir {
		return false
	}

	pattern := p.pattern

	// Handle patterns with leading /
	if strings.HasPrefix(pattern, "/") {
		// Anchored to root
		pattern = strings.TrimPrefix(pattern, "/")
		return m.matchGlob(pattern, path)
	}

	// Handle patterns with /
	if strings.Contains(pattern, "/") {
		// Match from root or any subdirectory
		if m.matchGlob(pattern, path) {
			return true
		}
		// Also try matching as a suffix
		parts := strings.Split(path, "/")
		for i := range parts {
			subpath := strings.Join(parts[i:], "/")
			if m.matchGlob(pattern, subpath) {
				return true
			}
		}
		return false
	}

	// Simple pattern - match basename or full path
	base := filepath.Base(path)
	if m.matchGlob(pattern, base) {
		return true
	}

	// Also try matching against each path component
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if m.matchGlob(pattern, part) {
			return true
		}
	}

	return false
}

// matchGlob performs glob-style pattern matching
func (m *Matcher) matchGlob(pattern, name string) bool {
	// Handle ** (match any number of directories)
	if strings.Contains(pattern, "**") {
		return m.matchDoublestar(pattern, name)
	}

	// Use filepath.Match for simple glob patterns
	matched, _ := filepath.Match(pattern, name)
	return matched
}

// matchDoublestar handles ** patterns
func (m *Matcher) matchDoublestar(pattern, path string) bool {
	// Split pattern by **
	parts := strings.Split(pattern, "**")

	if len(parts) == 2 {
		prefix := parts[0]
		suffix := parts[1]

		// Remove leading/trailing slashes from suffix
		suffix = strings.TrimPrefix(suffix, "/")

		// Check if prefix matches start of path
		if prefix != "" && !strings.HasPrefix(path, strings.TrimSuffix(prefix, "/")) {
			return false
		}

		// Check if suffix matches end of path
		if suffix != "" {
			pathParts := strings.Split(path, "/")
			for i := range pathParts {
				candidate := strings.Join(pathParts[i:], "/")
				if matched, _ := filepath.Match(suffix, candidate); matched {
					return true
				}
				// Also check just the filename
				if matched, _ := filepath.Match(suffix, pathParts[len(pathParts)-1]); matched {
					return true
				}
			}
			return false
		}

		return true
	}

	// Fallback: simple match
	matched, _ := filepath.Match(pattern, path)
	return matched
}

// ValidatePath checks if a path is allowed for tool access
// Returns an error if the path should be blocked
func (m *Matcher) ValidatePath(path string) error {
	// Make path relative to root if it's absolute
	if filepath.IsAbs(path) {
		relPath, err := filepath.Rel(m.root, path)
		if err != nil {
			// Security: deny access if we can't determine relative path
			// This prevents path traversal attacks
			return &PathResolutionError{Path: path, Err: err}
		}
		// Security: deny access if path escapes root (e.g., "../../../etc/passwd")
		if strings.HasPrefix(relPath, "..") {
			return &PathResolutionError{Path: path, Err: fmt.Errorf("path escapes root directory")}
		}
		path = relPath
	}

	if m.ShouldIgnore(path) {
		return &IgnoredPathError{Path: path}
	}

	return nil
}

// PathResolutionError is returned when a path cannot be safely resolved
type PathResolutionError struct {
	Path string
	Err  error
}

func (e *PathResolutionError) Error() string {
	return fmt.Sprintf("cannot resolve path %q: %v", e.Path, e.Err)
}

// IsPathResolutionError checks if an error is a PathResolutionError
func IsPathResolutionError(err error) bool {
	_, ok := err.(*PathResolutionError)
	return ok
}

// IgnoredPathError is returned when a path is blocked by .zcodeignore
type IgnoredPathError struct {
	Path string
}

func (e *IgnoredPathError) Error() string {
	return "path is blocked by .zcodeignore: " + e.Path
}

// IsIgnoredPathError checks if an error is an IgnoredPathError
func IsIgnoredPathError(err error) bool {
	_, ok := err.(*IgnoredPathError)
	return ok
}

// DefaultMatcher returns a matcher for the current working directory
func DefaultMatcher() (*Matcher, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return NewMatcher(cwd)
}
