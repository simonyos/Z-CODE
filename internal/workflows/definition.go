package workflows

// WorkflowDefinition represents a multi-step workflow loaded from YAML
type WorkflowDefinition struct {
	// Name is the unique identifier for the workflow
	Name string `yaml:"name"`

	// Description explains what the workflow does
	Description string `yaml:"description"`

	// Steps defines the sequence of agent executions
	Steps []WorkflowStep `yaml:"steps"`

	// FilePath is the source file this definition was loaded from
	FilePath string `yaml:"-"`

	// IsGlobal indicates if this workflow was loaded from global config
	IsGlobal bool `yaml:"-"`
}

// WorkflowStep defines a single step in a workflow
type WorkflowStep struct {
	// Name identifies this step (for referencing in conditions)
	Name string `yaml:"name"`

	// Agent is the name of the agent to execute
	Agent string `yaml:"agent"`

	// Input is the context key to read input from
	// The value will be prepended to the user prompt
	Input string `yaml:"input"`

	// Output is the context key to store the result in
	Output string `yaml:"output"`

	// Prompt overrides the user prompt for this step
	// Supports template variables like {user_input}, {step_name.output}
	Prompt string `yaml:"prompt"`

	// Condition is an expression that must be true to execute this step
	// Example: "review_results.has_issues == true"
	Condition string `yaml:"condition"`

	// LoopUntil is an expression that must be true to stop looping
	// The step will repeat until this condition is met
	LoopUntil string `yaml:"loop_until"`

	// MaxLoops limits the number of times a step can loop
	// Default is 1 (no looping) if LoopUntil is empty
	MaxLoops int `yaml:"max_loops"`

	// OnSuccess is the step name to jump to on success
	// Empty means continue to next step
	OnSuccess string `yaml:"on_success"`

	// OnFailure is the step name to jump to on failure
	// Empty means abort the workflow
	OnFailure string `yaml:"on_failure"`
}

// StepResult contains the outcome of executing a workflow step
type StepResult struct {
	StepName  string
	Agent     string
	Success   bool
	Output    string
	Error     string
	LoopCount int
}

// WorkflowResult contains the final outcome of a workflow
type WorkflowResult struct {
	WorkflowName string
	Success      bool
	StepResults  []StepResult
	FinalOutput  string
	Error        string
}

// Validate checks if the workflow definition is valid
func (d *WorkflowDefinition) Validate() error {
	if d.Name == "" {
		return ErrMissingName
	}
	if len(d.Steps) == 0 {
		return ErrNoSteps
	}
	for i, step := range d.Steps {
		if step.Agent == "" {
			return &StepError{Index: i, Err: ErrMissingAgent}
		}
	}
	return nil
}

// GetStep returns a step by name
func (d *WorkflowDefinition) GetStep(name string) (*WorkflowStep, bool) {
	for i := range d.Steps {
		if d.Steps[i].Name == name {
			return &d.Steps[i], true
		}
	}
	return nil, false
}

// StepError wraps an error with step index information
type StepError struct {
	Index int
	Err   error
}

func (e *StepError) Error() string {
	return e.Err.Error()
}

func (e *StepError) Unwrap() error {
	return e.Err
}
