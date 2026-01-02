package workflows

import "errors"

var (
	// ErrMissingName is returned when a workflow has no name
	ErrMissingName = errors.New("workflow missing required 'name' field")

	// ErrNoSteps is returned when a workflow has no steps
	ErrNoSteps = errors.New("workflow must have at least one step")

	// ErrMissingAgent is returned when a step has no agent specified
	ErrMissingAgent = errors.New("step missing required 'agent' field")

	// ErrWorkflowNotFound is returned when a workflow is not in the registry
	ErrWorkflowNotFound = errors.New("workflow not found")

	// ErrAgentNotFound is returned when a step references an unknown agent
	ErrAgentNotFound = errors.New("agent not found")

	// ErrStepNotFound is returned when a referenced step doesn't exist
	ErrStepNotFound = errors.New("step not found")

	// ErrMaxLoopsExceeded is returned when a step exceeds its loop limit
	ErrMaxLoopsExceeded = errors.New("maximum loop iterations exceeded")

	// ErrInvalidCondition is returned when a condition expression is invalid
	ErrInvalidCondition = errors.New("invalid condition expression")

	// ErrWorkflowAborted is returned when a workflow is cancelled
	ErrWorkflowAborted = errors.New("workflow aborted")
)
