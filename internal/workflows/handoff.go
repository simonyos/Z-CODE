package workflows

import (
	"context"

	"github.com/simonyos/Z-CODE/internal/agents"
	"github.com/simonyos/Z-CODE/internal/llm"
	"github.com/simonyos/Z-CODE/internal/tools"
)

// HandoffManager coordinates agent handoffs within workflows
type HandoffManager struct {
	agentRegistry *agents.Registry
	executor      *agents.Executor
	maxChainDepth int
}

// NewHandoffManager creates a new handoff manager
func NewHandoffManager(
	agentReg *agents.Registry,
	provider llm.Provider,
	confirmFn tools.ConfirmFunc,
) *HandoffManager {
	return &HandoffManager{
		agentRegistry: agentReg,
		executor:      agents.NewExecutor(provider, confirmFn),
		maxChainDepth: 10, // Default max chain depth
	}
}

// SetMaxChainDepth sets the maximum depth of handoff chains
func (hm *HandoffManager) SetMaxChainDepth(depth int) {
	if depth > 0 {
		hm.maxChainDepth = depth
	}
}

// HandoffChain represents a sequence of agent handoffs
type HandoffChain struct {
	Steps    []HandoffStep
	MaxDepth int
}

// HandoffStep records a single handoff in the chain
type HandoffStep struct {
	FromAgent string
	ToAgent   string
	Reason    string
	Context   map[string]any
	Result    *agents.ExecuteResult
}

// ProcessHandoff executes a handoff from one agent to another
func (hm *HandoffManager) ProcessHandoff(
	ctx context.Context,
	instruction *agents.HandoffInstruction,
	wfCtx *Context,
) (*StepResult, error) {
	result := &StepResult{
		StepName: "handoff_" + instruction.TargetAgent,
		Agent:    instruction.TargetAgent,
	}

	// Get the target agent
	agentDef, ok := hm.agentRegistry.Get(instruction.TargetAgent)
	if !ok {
		result.Success = false
		result.Error = "agent not found: " + instruction.TargetAgent
		return result, ErrAgentNotFound
	}

	// Build prompt from handoff context
	prompt := hm.buildHandoffPrompt(instruction)

	// Execute the target agent
	execResult, err := hm.executor.Execute(ctx, agentDef, prompt)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result, err
	}

	result.Success = true
	result.Output = execResult.Response

	return result, nil
}

// ProcessHandoffChain executes a chain of handoffs until completion or max depth
func (hm *HandoffManager) ProcessHandoffChain(
	ctx context.Context,
	initialInstruction *agents.HandoffInstruction,
	wfCtx *Context,
) (*HandoffChain, error) {
	chain := &HandoffChain{
		Steps:    []HandoffStep{},
		MaxDepth: hm.maxChainDepth,
	}

	currentInstruction := initialInstruction
	previousAgent := ""

	for len(chain.Steps) < hm.maxChainDepth {
		select {
		case <-ctx.Done():
			return chain, ErrWorkflowAborted
		default:
		}

		if currentInstruction == nil {
			break
		}

		// Get the target agent
		agentDef, ok := hm.agentRegistry.Get(currentInstruction.TargetAgent)
		if !ok {
			return chain, ErrAgentNotFound
		}

		// Build prompt from handoff context
		prompt := hm.buildHandoffPrompt(currentInstruction)

		// Execute the target agent
		execResult, err := hm.executor.Execute(ctx, agentDef, prompt)

		step := HandoffStep{
			FromAgent: previousAgent,
			ToAgent:   currentInstruction.TargetAgent,
			Reason:    currentInstruction.Reason,
			Context:   currentInstruction.Context,
			Result:    execResult,
		}
		chain.Steps = append(chain.Steps, step)

		if err != nil {
			return chain, err
		}

		// Store result in workflow context
		wfCtx.Set("handoff_"+currentInstruction.TargetAgent, execResult.Response)

		// Check if the agent requested another handoff
		if execResult.Handoff != nil {
			previousAgent = currentInstruction.TargetAgent
			currentInstruction = execResult.Handoff
		} else {
			break // No more handoffs
		}
	}

	return chain, nil
}

// buildHandoffPrompt creates a prompt for the target agent from handoff context
func (hm *HandoffManager) buildHandoffPrompt(instruction *agents.HandoffInstruction) string {
	var prompt string

	if instruction.Reason != "" {
		prompt = "Handoff reason: " + instruction.Reason + "\n\n"
	}

	// Add context values
	for key, value := range instruction.Context {
		prompt += key + ":\n" + agents.ValueToString(value) + "\n\n"
	}

	if prompt == "" {
		prompt = "Continue from the previous agent's work."
	}

	return prompt
}

