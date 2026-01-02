package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Loader handles discovery and parsing of agent definitions from markdown files
type Loader struct {
	paths      []string
	globalPath string // The known global config path
}

// NewLoader creates a new agent loader with the given search paths
func NewLoader(paths []string) *Loader {
	globalPath := ""
	if home, err := os.UserHomeDir(); err == nil {
		globalPath = filepath.Join(home, ".config", "zcode", "agents")
	}
	return &Loader{paths: paths, globalPath: globalPath}
}

// LoadAll discovers and loads all agent definitions from all configured paths
func (l *Loader) LoadAll() ([]*AgentDefinition, error) {
	var agents []*AgentDefinition

	for _, basePath := range l.paths {
		// Determine if this is the global path by comparing canonical paths
		isGlobal := l.globalPath != "" && basePath == l.globalPath

		// Check if directory exists
		info, err := os.Stat(basePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Skip non-existent directories
			}
			return nil, fmt.Errorf("error accessing %s: %w", basePath, err)
		}
		if !info.IsDir() {
			continue
		}

		// Find all .md files in the directory
		entries, err := os.ReadDir(basePath)
		if err != nil {
			return nil, fmt.Errorf("error reading directory %s: %w", basePath, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}

			filePath := filepath.Join(basePath, entry.Name())
			agent, err := l.LoadFromFile(filePath)
			if err != nil {
				// Log but don't fail on individual file errors
				fmt.Fprintf(os.Stderr, "Warning: failed to load agent from %s: %v\n", filePath, err)
				continue
			}

			agent.IsGlobal = isGlobal
			agents = append(agents, agent)
		}
	}

	return agents, nil
}

// LoadFromFile parses a single markdown file with YAML frontmatter
func (l *Loader) LoadFromFile(filePath string) (*AgentDefinition, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	agent, err := ParseAgentMarkdown(string(content))
	if err != nil {
		return nil, err
	}

	agent.FilePath = filePath
	return agent, nil
}

// ParseAgentMarkdown parses markdown content with YAML frontmatter into an AgentDefinition
func ParseAgentMarkdown(content string) (*AgentDefinition, error) {
	frontmatter, body, err := parseFrontmatter(content)
	if err != nil {
		return nil, err
	}

	var agent AgentDefinition
	if err := yaml.Unmarshal([]byte(frontmatter), &agent); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFrontmatter, err)
	}

	agent.SystemPrompt = strings.TrimSpace(body)

	if err := agent.Validate(); err != nil {
		return nil, err
	}

	return &agent, nil
}

// parseFrontmatter extracts YAML frontmatter and body from markdown content
// Frontmatter must be enclosed in --- markers at the start of the file
func parseFrontmatter(content string) (frontmatter, body string, err error) {
	content = strings.TrimSpace(content)

	// Must start with ---
	if !strings.HasPrefix(content, "---") {
		return "", "", ErrNoFrontmatter
	}

	// Find the closing ---
	rest := content[3:] // Skip opening ---
	rest = strings.TrimLeft(rest, "\r\n")

	endIdx := strings.Index(rest, "\n---")
	if endIdx == -1 {
		// Try Windows line endings
		endIdx = strings.Index(rest, "\r\n---")
		if endIdx == -1 {
			return "", "", ErrNoFrontmatter
		}
	}

	frontmatter = strings.TrimSpace(rest[:endIdx])
	body = strings.TrimSpace(rest[endIdx+4:]) // Skip \n---

	return frontmatter, body, nil
}

// DefaultPaths returns the default agent search paths
func DefaultPaths() []string {
	paths := []string{}

	// Project-local path
	cwd, err := os.Getwd()
	if err == nil {
		paths = append(paths, filepath.Join(cwd, ".zcode", "agents"))
	}

	// Global config path
	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".config", "zcode", "agents"))
	}

	return paths
}
