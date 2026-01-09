package swarm

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed roles/*.md
var embeddedRoles embed.FS

// RoleLoader handles loading role definitions from markdown files
type RoleLoader struct {
	rolesDir    string
	definitions map[Role]*RoleDefinition
}

// NewRoleLoader creates a new role loader
func NewRoleLoader(rolesDir string) *RoleLoader {
	return &RoleLoader{
		rolesDir:    rolesDir,
		definitions: make(map[Role]*RoleDefinition),
	}
}

// LoadAll loads all role definitions from the roles directory
// Falls back to embedded roles if directory doesn't exist or files are missing
func (l *RoleLoader) LoadAll() error {
	// Start with embedded defaults
	if err := l.loadEmbedded(); err != nil {
		return fmt.Errorf("failed to load embedded roles: %w", err)
	}

	// Override with custom roles from directory if it exists
	if l.rolesDir != "" {
		if _, err := os.Stat(l.rolesDir); err == nil {
			if err := l.loadFromDir(l.rolesDir); err != nil {
				// Non-fatal: just use embedded
				return nil
			}
		}
	}

	return nil
}

// loadEmbedded loads roles from embedded files
func (l *RoleLoader) loadEmbedded() error {
	entries, err := embeddedRoles.ReadDir("roles")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := embeddedRoles.ReadFile("roles/" + entry.Name())
		if err != nil {
			continue
		}

		def, err := parseRoleMarkdown(string(content))
		if err != nil {
			continue
		}

		if def != nil && def.Role != "" {
			l.definitions[def.Role] = def
		}
	}

	return nil
}

// loadFromDir loads role definitions from a directory
func (l *RoleLoader) loadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		def, err := parseRoleMarkdown(string(content))
		if err != nil {
			continue
		}

		if def != nil && def.Role != "" {
			l.definitions[def.Role] = def
		}
	}

	return nil
}

// Get returns a role definition by role
func (l *RoleLoader) Get(role Role) *RoleDefinition {
	return l.definitions[role]
}

// GetAll returns all loaded role definitions
func (l *RoleLoader) GetAll() map[Role]*RoleDefinition {
	return l.definitions
}

// parseRoleMarkdown parses a markdown role definition file
func parseRoleMarkdown(content string) (*RoleDefinition, error) {
	def := &RoleDefinition{
		Capabilities: []string{},
		Permissions:  []string{},
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	var currentSection string
	var systemPromptLines []string
	inSystemPrompt := false

	// Regex for title line: # Name (ROLE)
	titleRegex := regexp.MustCompile(`^#\s+(.+?)\s+\(([A-Z_]+)\)`)
	// Regex for description: **Description:** text
	descRegex := regexp.MustCompile(`^\*\*Description:\*\*\s*(.+)`)
	// Regex for section headers
	sectionRegex := regexp.MustCompile(`^##\s+(.+)`)
	// Regex for list items
	listItemRegex := regexp.MustCompile(`^-\s+(.+)`)

	for scanner.Scan() {
		line := scanner.Text()

		// Check for title
		if matches := titleRegex.FindStringSubmatch(line); matches != nil {
			def.Name = strings.TrimSpace(matches[1])
			roleStr := strings.TrimSpace(matches[2])
			role, err := ParseRole(roleStr)
			if err != nil {
				// Try mapping common variations
				role = mapRoleName(roleStr)
			}
			def.Role = role
			continue
		}

		// Check for description
		if matches := descRegex.FindStringSubmatch(line); matches != nil {
			def.Description = strings.TrimSpace(matches[1])
			continue
		}

		// Check for section header
		if matches := sectionRegex.FindStringSubmatch(line); matches != nil {
			section := strings.TrimSpace(matches[1])
			currentSection = strings.ToLower(section)

			if currentSection == "system prompt" {
				inSystemPrompt = true
				continue
			} else {
				inSystemPrompt = false
			}
			continue
		}

		// If in system prompt section, collect all lines
		if inSystemPrompt {
			systemPromptLines = append(systemPromptLines, line)
			continue
		}

		// Check for list items in capabilities/permissions sections
		if matches := listItemRegex.FindStringSubmatch(line); matches != nil {
			item := strings.TrimSpace(matches[1])
			switch currentSection {
			case "capabilities":
				def.Capabilities = append(def.Capabilities, item)
			case "permissions":
				def.Permissions = append(def.Permissions, item)
			}
		}
	}

	// Set system prompt
	if len(systemPromptLines) > 0 {
		def.SystemPrompt = strings.TrimSpace(strings.Join(systemPromptLines, "\n"))
	}

	return def, nil
}

// mapRoleName maps common role name variations to Role constants
func mapRoleName(name string) Role {
	name = strings.ToUpper(strings.TrimSpace(name))
	switch name {
	case "ORCH", "ORCHESTRATOR":
		return RoleOrchestrator
	case "SA", "SOLUTION_ARCHITECT", "SOLUTION ARCHITECT":
		return RoleSA
	case "BE_DEV", "BACKEND_DEVELOPER", "BACKEND DEVELOPER", "BACKEND":
		return RoleBEDev
	case "FE_DEV", "FRONTEND_DEVELOPER", "FRONTEND DEVELOPER", "FRONTEND":
		return RoleFEDev
	case "QA", "QUALITY_ASSURANCE", "QUALITY ASSURANCE":
		return RoleQA
	case "DEVOPS", "DEV_OPS":
		return RoleDevOps
	case "DBA", "DATABASE_ADMINISTRATOR", "DATABASE ADMINISTRATOR":
		return RoleDBA
	case "SEC", "SECURITY", "SECURITY_ENGINEER", "SECURITY ENGINEER":
		return RoleSecurity
	case "HUMAN", "HUMAN_OBSERVER", "HUMAN OBSERVER":
		return RoleHuman
	default:
		return Role(name)
	}
}

// LoadRoles is a convenience function to load all roles with optional custom directory
func LoadRoles(customDir string) (map[Role]*RoleDefinition, error) {
	loader := NewRoleLoader(customDir)
	if err := loader.LoadAll(); err != nil {
		return nil, err
	}
	return loader.GetAll(), nil
}
