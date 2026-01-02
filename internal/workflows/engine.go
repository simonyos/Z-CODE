package workflows

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/simonyos/Z-CODE/internal/agents"
	"github.com/simonyos/Z-CODE/internal/llm"
	"github.com/simonyos/Z-CODE/internal/tools"
)

// Engine executes workflows
type Engine struct {
	agentRegistry    *agents.Registry
	workflowRegistry *Registry
	executor         *agents.Executor
}

// NewEngine creates a new workflow engine
func NewEngine(
	agentReg *agents.Registry,
	workflowReg *Registry,
	provider llm.Provider,
	confirmFn tools.ConfirmFunc,
) *Engine {
	return &Engine{
		agentRegistry:    agentReg,
		workflowRegistry: workflowReg,
		executor:         agents.NewExecutor(provider, confirmFn),
	}
}

// Execute runs a workflow by name
func (e *Engine) Execute(ctx context.Context, workflowName string, initialPrompt string) (*WorkflowResult, error) {
	workflow, ok := e.workflowRegistry.Get(workflowName)
	if !ok {
		return nil, ErrWorkflowNotFound
	}

	wfCtx := NewContext()
	wfCtx.Set("user_input", initialPrompt)

	result := &WorkflowResult{
		WorkflowName: workflowName,
		StepResults:  []StepResult{},
	}

	// Execute steps in order
	stepIndex := 0
	for stepIndex < len(workflow.Steps) {
		select {
		case <-ctx.Done():
			result.Success = false
			result.Error = ErrWorkflowAborted.Error()
			return result, ErrWorkflowAborted
		default:
		}

		step := workflow.Steps[stepIndex]

		// Check condition
		if step.Condition != "" {
			condMet, err := e.evaluateCondition(step.Condition, wfCtx)
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("condition evaluation failed: %v", err)
				return result, err
			}
			if !condMet {
				stepIndex++
				continue
			}
		}

		// Execute the step (with looping support)
		stepResult, err := e.executeStepWithLooping(ctx, &step, wfCtx, initialPrompt)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			result.StepResults = append(result.StepResults, *stepResult)

			// Handle failure routing
			if step.OnFailure != "" {
				nextIdx := e.findStepIndex(workflow, step.OnFailure)
				if nextIdx >= 0 {
					stepIndex = nextIdx
					continue
				}
			}
			return result, err
		}

		result.StepResults = append(result.StepResults, *stepResult)

		// Store result in context
		if step.Output != "" {
			wfCtx.Set(step.Output, stepResult.Output)
		}
		wfCtx.SetResult(step.Name, *stepResult)

		// Handle success routing
		if step.OnSuccess != "" {
			nextIdx := e.findStepIndex(workflow, step.OnSuccess)
			if nextIdx >= 0 {
				stepIndex = nextIdx
				continue
			}
		}

		stepIndex++
	}

	result.Success = true
	if len(result.StepResults) > 0 {
		result.FinalOutput = result.StepResults[len(result.StepResults)-1].Output
	}

	return result, nil
}

// executeStepWithLooping executes a step, handling loop_until conditions
func (e *Engine) executeStepWithLooping(
	ctx context.Context,
	step *WorkflowStep,
	wfCtx *Context,
	initialPrompt string,
) (*StepResult, error) {
	maxLoops := step.MaxLoops
	if maxLoops <= 0 {
		maxLoops = 1 // Default: no looping
	}
	if step.LoopUntil != "" && maxLoops == 1 {
		maxLoops = 10 // Default max loops when loop_until is set
	}

	var lastResult *StepResult

	for loopCount := 1; loopCount <= maxLoops; loopCount++ {
		result, err := e.executeStep(ctx, step, wfCtx, initialPrompt)
		result.LoopCount = loopCount
		lastResult = result

		if err != nil {
			return result, err
		}

		// Check loop_until condition
		if step.LoopUntil != "" {
			// Store intermediate result for condition evaluation
			wfCtx.SetResult(step.Name, *result)

			condMet, err := e.evaluateCondition(step.LoopUntil, wfCtx)
			if err != nil {
				return result, fmt.Errorf("loop condition evaluation failed: %w", err)
			}
			if condMet {
				return result, nil // Condition met, stop looping
			}
		} else {
			return result, nil // No loop condition, execute once
		}
	}

	// Max loops exceeded
	if lastResult != nil {
		lastResult.Success = false
		lastResult.Error = ErrMaxLoopsExceeded.Error()
	}
	return lastResult, ErrMaxLoopsExceeded
}

// executeStep executes a single workflow step
func (e *Engine) executeStep(
	ctx context.Context,
	step *WorkflowStep,
	wfCtx *Context,
	initialPrompt string,
) (*StepResult, error) {
	result := &StepResult{
		StepName: step.Name,
		Agent:    step.Agent,
	}

	// Get the agent definition
	agentDef, ok := e.agentRegistry.Get(step.Agent)
	if !ok {
		result.Success = false
		result.Error = fmt.Sprintf("agent not found: %s", step.Agent)
		return result, ErrAgentNotFound
	}

	// Build the prompt
	prompt := e.buildPrompt(step, wfCtx, initialPrompt)

	// Execute the agent
	execResult, err := e.executor.Execute(ctx, agentDef, prompt)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result, err
	}

	result.Success = true
	result.Output = execResult.Response

	// Handle handoff if requested
	if execResult.Handoff != nil {
		// Store handoff info in context
		wfCtx.Set(step.Name+"_handoff", execResult.Handoff)
	}

	return result, nil
}

// buildPrompt constructs the prompt for a step
func (e *Engine) buildPrompt(step *WorkflowStep, wfCtx *Context, initialPrompt string) string {
	var prompt string

	if step.Prompt != "" {
		// Use step's custom prompt with template substitution
		prompt = e.substituteTemplates(step.Prompt, wfCtx, initialPrompt)
	} else {
		prompt = initialPrompt
	}

	// Prepend input from context if specified
	if step.Input != "" {
		inputValue := wfCtx.GetString(step.Input)
		if inputValue != "" {
			prompt = fmt.Sprintf("Context from previous step:\n%s\n\nTask:\n%s", inputValue, prompt)
		}
	}

	return prompt
}

// substituteTemplates replaces template variables in a string
// Supports: {user_input}, {step_name}, {step_name.output}, etc.
func (e *Engine) substituteTemplates(template string, wfCtx *Context, initialPrompt string) string {
	result := template

	// Replace {user_input}
	result = strings.ReplaceAll(result, "{user_input}", initialPrompt)

	// Replace {key} and {key.field} patterns
	contextMap := wfCtx.ToMap()
	pattern := regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*)?)\}`)

	result = pattern.ReplaceAllStringFunc(result, func(match string) string {
		key := match[1 : len(match)-1] // Remove { and }

		// Handle nested keys like "step_name.output"
		parts := strings.SplitN(key, ".", 2)
		value, ok := contextMap[parts[0]]
		if !ok {
			return match // Keep original if not found
		}

		if len(parts) == 1 {
			return fmt.Sprintf("%v", value)
		}

		// Handle nested access
		if nested, ok := value.(map[string]any); ok {
			if nestedVal, ok := nested[parts[1]]; ok {
				return fmt.Sprintf("%v", nestedVal)
			}
		}

		return match // Keep original if nested key not found
	})

	return result
}

// evaluateCondition evaluates a simple condition expression
// Supports: "key == value", "key != value", "key.field == value"
func (e *Engine) evaluateCondition(condition string, wfCtx *Context) (bool, error) {
	condition = strings.TrimSpace(condition)

	// Handle "true" and "false" literals
	if condition == "true" {
		return true, nil
	}
	if condition == "false" {
		return false, nil
	}

	// Parse comparison operators
	var left, right, op string

	if strings.Contains(condition, "==") {
		parts := strings.SplitN(condition, "==", 2)
		left, right, op = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), "=="
	} else if strings.Contains(condition, "!=") {
		parts := strings.SplitN(condition, "!=", 2)
		left, right, op = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), "!="
	} else {
		// Treat as existence check
		value := e.resolveValue(condition, wfCtx)
		return value != nil && value != "" && value != false, nil
	}

	leftVal := e.resolveValue(left, wfCtx)
	rightVal := e.resolveValue(right, wfCtx)

	switch op {
	case "==":
		return fmt.Sprintf("%v", leftVal) == fmt.Sprintf("%v", rightVal), nil
	case "!=":
		return fmt.Sprintf("%v", leftVal) != fmt.Sprintf("%v", rightVal), nil
	}

	return false, ErrInvalidCondition
}

// resolveValue resolves a value from the context or returns the literal
func (e *Engine) resolveValue(expr string, wfCtx *Context) any {
	expr = strings.TrimSpace(expr)

	// Handle quoted strings
	if (strings.HasPrefix(expr, "\"") && strings.HasSuffix(expr, "\"")) ||
		(strings.HasPrefix(expr, "'") && strings.HasSuffix(expr, "'")) {
		return expr[1 : len(expr)-1]
	}

	// Handle boolean literals
	if expr == "true" {
		return true
	}
	if expr == "false" {
		return false
	}

	// Handle context lookups
	contextMap := wfCtx.ToMap()
	parts := strings.SplitN(expr, ".", 2)

	value, ok := contextMap[parts[0]]
	if !ok {
		return expr // Return as literal if not found
	}

	if len(parts) == 1 {
		return value
	}

	// Handle nested access
	if nested, ok := value.(map[string]any); ok {
		if nestedVal, ok := nested[parts[1]]; ok {
			return nestedVal
		}
	}

	return nil
}

// findStepIndex returns the index of a step by name, or -1 if not found
func (e *Engine) findStepIndex(workflow *WorkflowDefinition, stepName string) int {
	for i, step := range workflow.Steps {
		if step.Name == stepName {
			return i
		}
	}
	return -1
}

// StreamEvent represents events during workflow streaming execution
type StreamEvent struct {
	Type          string // "workflow_start", "step_start", "step_done", "workflow_done", "error"
	WorkflowName  string
	StepName      string
	AgentName     string
	StepResult    *StepResult
	WorkflowResult *WorkflowResult
	Error         error
}

// ExecuteStream runs a workflow with streaming events
func (e *Engine) ExecuteStream(ctx context.Context, workflowName string, initialPrompt string) <-chan StreamEvent {
	events := make(chan StreamEvent)

	go func() {
		defer close(events)

		events <- StreamEvent{Type: "workflow_start", WorkflowName: workflowName}

		result, err := e.Execute(ctx, workflowName, initialPrompt)
		if err != nil {
			events <- StreamEvent{Type: "error", Error: err, WorkflowResult: result}
			return
		}

		events <- StreamEvent{Type: "workflow_done", WorkflowResult: result}
	}()

	return events
}
