// Package agent - Task State management for Z-CODE
// Provides centralized state tracking for agent execution
package agent

import (
	"context"
	"sync"
	"time"
)

// TaskState manages the state of an agent task execution
// Similar to Cline's TaskState for tracking execution flow
type TaskState struct {
	mu sync.RWMutex

	// Task identification
	ID        string
	StartedAt time.Time

	// Abort/cancellation state
	Abort         bool
	AbortReason   string
	CancelContext context.CancelFunc

	// Tool execution state
	DidRejectTool     bool   // User rejected a tool execution
	RejectedToolName  string // Name of the rejected tool
	DidAlreadyUseTool bool   // For sequential tool execution mode

	// Iteration tracking
	CurrentIteration int
	MaxIterations    int

	// Message state
	UserMessageContent []ContentBlock
	LastToolUseID      string

	// Tool tracking
	ToolUseIDMap map[string]string // Maps transformed IDs to original IDs

	// Hooks state (for pre/post tool execution)
	ActiveHookExecution *HookExecution
}

// ContentBlock represents a content block in messages
type ContentBlock struct {
	Type    string `json:"type"` // "text", "tool_use", "tool_result"
	Text    string `json:"text,omitempty"`
	ToolID  string `json:"tool_id,omitempty"`
	Content string `json:"content,omitempty"`
}

// HookExecution tracks an active hook execution
type HookExecution struct {
	HookName  string
	ToolName  string
	StartedAt time.Time
	Cancelled bool
}

// NewTaskState creates a new task state with defaults
func NewTaskState(id string, maxIterations int) *TaskState {
	return &TaskState{
		ID:                 id,
		StartedAt:          time.Now(),
		MaxIterations:      maxIterations,
		ToolUseIDMap:       make(map[string]string),
		UserMessageContent: make([]ContentBlock, 0),
	}
}

// RequestAbort sets the abort flag with a reason
func (ts *TaskState) RequestAbort(reason string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.Abort = true
	ts.AbortReason = reason

	if ts.CancelContext != nil {
		ts.CancelContext()
	}
}

// IsAborted checks if the task has been aborted
func (ts *TaskState) IsAborted() bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.Abort
}

// GetAbortReason returns the abort reason
func (ts *TaskState) GetAbortReason() string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.AbortReason
}

// RejectTool marks that a tool was rejected by the user
func (ts *TaskState) RejectTool(toolName string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.DidRejectTool = true
	ts.RejectedToolName = toolName
}

// WasToolRejected checks if any tool was rejected
func (ts *TaskState) WasToolRejected() bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.DidRejectTool
}

// ClearToolRejection clears the tool rejection state
func (ts *TaskState) ClearToolRejection() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.DidRejectTool = false
	ts.RejectedToolName = ""
}

// IncrementIteration increments and returns the current iteration count
func (ts *TaskState) IncrementIteration() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.CurrentIteration++
	return ts.CurrentIteration
}

// HasReachedMaxIterations checks if max iterations have been reached
func (ts *TaskState) HasReachedMaxIterations() bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.CurrentIteration >= ts.MaxIterations
}

// AddUserContent adds a content block to the user message
func (ts *TaskState) AddUserContent(block ContentBlock) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.UserMessageContent = append(ts.UserMessageContent, block)
}

// ClearUserContent clears all user message content
func (ts *TaskState) ClearUserContent() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.UserMessageContent = make([]ContentBlock, 0)
}

// GetUserContent returns a copy of the user message content
func (ts *TaskState) GetUserContent() []ContentBlock {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	result := make([]ContentBlock, len(ts.UserMessageContent))
	copy(result, ts.UserMessageContent)
	return result
}

// MapToolUseID maps a transformed tool use ID to its original
func (ts *TaskState) MapToolUseID(transformed, original string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.ToolUseIDMap[transformed] = original
}

// GetOriginalToolUseID returns the original tool use ID for a transformed one
func (ts *TaskState) GetOriginalToolUseID(transformed string) (string, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	original, ok := ts.ToolUseIDMap[transformed]
	return original, ok
}

// SetActiveHook sets the currently executing hook
func (ts *TaskState) SetActiveHook(hookName, toolName string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.ActiveHookExecution = &HookExecution{
		HookName:  hookName,
		ToolName:  toolName,
		StartedAt: time.Now(),
	}
}

// ClearActiveHook clears the active hook execution
func (ts *TaskState) ClearActiveHook() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.ActiveHookExecution = nil
}

// GetActiveHook returns the currently executing hook
func (ts *TaskState) GetActiveHook() *HookExecution {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if ts.ActiveHookExecution == nil {
		return nil
	}

	// Return a copy
	hook := *ts.ActiveHookExecution
	return &hook
}

// CancelActiveHook marks the active hook as cancelled
func (ts *TaskState) CancelActiveHook() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.ActiveHookExecution != nil {
		ts.ActiveHookExecution.Cancelled = true
	}
}

// Reset resets the task state for a new execution
func (ts *TaskState) Reset() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.Abort = false
	ts.AbortReason = ""
	ts.DidRejectTool = false
	ts.RejectedToolName = ""
	ts.DidAlreadyUseTool = false
	ts.CurrentIteration = 0
	ts.UserMessageContent = make([]ContentBlock, 0)
	ts.LastToolUseID = ""
	ts.ToolUseIDMap = make(map[string]string)
	ts.ActiveHookExecution = nil
}

// Duration returns how long the task has been running
func (ts *TaskState) Duration() time.Duration {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return time.Since(ts.StartedAt)
}

// Summary returns a summary of the current state
func (ts *TaskState) Summary() map[string]interface{} {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	return map[string]interface{}{
		"id":                ts.ID,
		"started_at":        ts.StartedAt,
		"duration":          time.Since(ts.StartedAt).String(),
		"aborted":           ts.Abort,
		"abort_reason":      ts.AbortReason,
		"tool_rejected":     ts.DidRejectTool,
		"current_iteration": ts.CurrentIteration,
		"max_iterations":    ts.MaxIterations,
		"content_blocks":    len(ts.UserMessageContent),
	}
}
