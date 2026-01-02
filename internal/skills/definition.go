// Package skills provides a lightweight prompt template system.
// Skills are simpler than agents - they're reusable prompt snippets
// that can be invoked directly or composed into agent prompts.
package skills

// SkillDefinition represents a reusable prompt template.
// Skills are lighter-weight than agents - they don't have their own
// tool restrictions or iteration limits.
type SkillDefinition struct {
	// Name is the unique identifier for this skill (used as slash command)
	Name string `yaml:"name"`

	// Description is shown in /skills listing
	Description string `yaml:"description"`

	// Prompt is the template text that gets expanded
	// Supports {user_input} and {variable_name} placeholders
	Prompt string `yaml:"-"`

	// Variables defines named placeholders that can be referenced
	// Example: variables: ["file_path", "language"]
	Variables []string `yaml:"variables"`

	// Tags for categorization and discovery
	Tags []string `yaml:"tags"`

	// FilePath is the source file (populated by loader)
	FilePath string `yaml:"-"`

	// IsGlobal indicates if loaded from global config
	IsGlobal bool `yaml:"-"`
}

// SkillInvocation represents a skill being used with specific values
type SkillInvocation struct {
	Skill     *SkillDefinition
	UserInput string
	Variables map[string]string
}

// Expand returns the skill prompt with all placeholders replaced
func (si *SkillInvocation) Expand() string {
	result := si.Skill.Prompt

	// Replace {user_input} placeholder
	result = replacePlaceholder(result, "user_input", si.UserInput)

	// Replace named variable placeholders
	for name, value := range si.Variables {
		result = replacePlaceholder(result, name, value)
	}

	return result
}

// replacePlaceholder replaces {name} with value in text
func replacePlaceholder(text, name, value string) string {
	placeholder := "{" + name + "}"
	result := ""
	i := 0
	for i < len(text) {
		if i+len(placeholder) <= len(text) && text[i:i+len(placeholder)] == placeholder {
			result += value
			i += len(placeholder)
		} else {
			result += string(text[i])
			i++
		}
	}
	return result
}
