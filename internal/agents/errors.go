package agents

import "errors"

var (
	// ErrMissingName is returned when an agent definition has no name
	ErrMissingName = errors.New("agent definition missing required 'name' field")

	// ErrMissingSystemPrompt is returned when an agent has no system prompt
	ErrMissingSystemPrompt = errors.New("agent definition missing system prompt (markdown body)")

	// ErrAgentNotFound is returned when an agent is not in the registry
	ErrAgentNotFound = errors.New("agent not found")

	// ErrInvalidFrontmatter is returned when YAML frontmatter parsing fails
	ErrInvalidFrontmatter = errors.New("invalid YAML frontmatter")

	// ErrNoFrontmatter is returned when a markdown file has no frontmatter
	ErrNoFrontmatter = errors.New("markdown file missing YAML frontmatter")

	// ErrReservedName is returned when an agent uses a reserved command name
	ErrReservedName = errors.New("agent name conflicts with built-in command")
)

// ReservedNames contains names that cannot be used for custom agents
// because they conflict with built-in slash commands
var ReservedNames = map[string]bool{
	"help":      true,
	"clear":     true,
	"reset":     true,
	"tools":     true,
	"config":    true,
	"agents":    true,
	"skills":    true,
	"workflows": true,
	"quit":      true,
	"exit":      true,
	"q":         true,
}
