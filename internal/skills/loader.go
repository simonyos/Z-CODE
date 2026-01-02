package skills

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Loader handles discovery and parsing of skill definitions
type Loader struct {
	paths      []string
	globalPath string // The known global config path
}

// NewLoader creates a loader that searches the given paths
func NewLoader(paths []string) *Loader {
	globalPath := ""
	if home, err := os.UserHomeDir(); err == nil {
		globalPath = filepath.Join(home, ".config", "zcode", "skills")
	}
	return &Loader{paths: paths, globalPath: globalPath}
}

// LoadAll loads all skill definitions from configured paths
func (l *Loader) LoadAll() ([]*SkillDefinition, error) {
	var skills []*SkillDefinition

	for _, basePath := range l.paths {
		// Determine if this is the global path by comparing canonical paths
		isGlobal := l.globalPath != "" && basePath == l.globalPath

		entries, err := os.ReadDir(basePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if !strings.HasSuffix(name, ".md") {
				continue
			}

			filePath := filepath.Join(basePath, name)
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			skill, err := ParseSkillMarkdown(string(content))
			if err != nil {
				continue
			}

			skill.FilePath = filePath
			skill.IsGlobal = isGlobal
			skills = append(skills, skill)
		}
	}

	return skills, nil
}

// ParseSkillMarkdown parses a markdown file with YAML frontmatter
func ParseSkillMarkdown(content string) (*SkillDefinition, error) {
	frontmatter, body, err := parseFrontmatter(content)
	if err != nil {
		return nil, err
	}

	var skill SkillDefinition
	if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
		return nil, err
	}

	skill.Prompt = strings.TrimSpace(body)

	if skill.Name == "" {
		return nil, ErrMissingName
	}

	return &skill, nil
}

// parseFrontmatter extracts YAML frontmatter from markdown content
func parseFrontmatter(content string) (frontmatter, body string, err error) {
	content = strings.TrimSpace(content)

	if !strings.HasPrefix(content, "---") {
		return "", "", ErrMissingFrontmatter
	}

	// Find the closing ---
	rest := content[3:]
	endIdx := strings.Index(rest, "\n---")
	if endIdx == -1 {
		return "", "", ErrMissingFrontmatter
	}

	frontmatter = strings.TrimSpace(rest[:endIdx])
	body = strings.TrimSpace(rest[endIdx+4:])

	return frontmatter, body, nil
}
