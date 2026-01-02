package skills

import "errors"

var (
	// ErrMissingFrontmatter indicates the skill file lacks YAML frontmatter
	ErrMissingFrontmatter = errors.New("skill file must start with YAML frontmatter (---)")

	// ErrMissingName indicates the skill definition has no name field
	ErrMissingName = errors.New("skill must have a name field")

	// ErrSkillNotFound indicates the requested skill doesn't exist
	ErrSkillNotFound = errors.New("skill not found")
)
