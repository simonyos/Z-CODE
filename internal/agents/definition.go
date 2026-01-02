package agents

import "strings"

// AgentDefinition represents a custom agent loaded from a markdown file
type AgentDefinition struct {
	// Name is the unique identifier for the agent (used in slash commands)
	Name string `yaml:"name"`

	// Description is a brief explanation of what the agent does
	Description string `yaml:"description"`

	// SystemPrompt is the markdown content after the frontmatter
	// This defines the agent's behavior and instructions
	SystemPrompt string `yaml:"-"`

	// Tools is the list of tool names this agent can use
	// Empty means all tools are available
	Tools []string `yaml:"tools"`

	// MaxIterations is the maximum number of LLM calls per conversation
	// Default is 10 if not specified
	MaxIterations int `yaml:"max_iterations"`

	// HandoffTo is the default agent to hand off to when this agent completes
	// Empty means no automatic handoff
	HandoffTo string `yaml:"handoff_to"`

	// FilePath is the source file this definition was loaded from
	FilePath string `yaml:"-"`

	// IsGlobal indicates if this agent was loaded from global config
	// (as opposed to project-local .zcode/agents/)
	IsGlobal bool `yaml:"-"`
}

// HandoffInstruction represents a request to transfer control to another agent
type HandoffInstruction struct {
	// TargetAgent is the name of the agent to hand off to
	TargetAgent string

	// Reason explains why the handoff is occurring
	Reason string

	// Context contains data to pass to the target agent
	Context map[string]any
}

// Validate checks if the agent definition is valid
func (d *AgentDefinition) Validate() error {
	if d.Name == "" {
		return ErrMissingName
	}
	// Check for reserved names (case-insensitive)
	if ReservedNames[strings.ToLower(d.Name)] {
		return ErrReservedName
	}
	if d.SystemPrompt == "" {
		return ErrMissingSystemPrompt
	}
	return nil
}

// HasRestrictedTools returns true if the agent has a limited tool set
func (d *AgentDefinition) HasRestrictedTools() bool {
	return len(d.Tools) > 0
}

// GetMaxIterations returns the max iterations, defaulting to 10
func (d *AgentDefinition) GetMaxIterations() int {
	if d.MaxIterations <= 0 {
		return 10
	}
	return d.MaxIterations
}
