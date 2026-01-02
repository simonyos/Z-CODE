package workflows

import (
	"sync"
	"time"
)

// Context holds shared state between agents in a workflow
type Context struct {
	mu      sync.RWMutex
	values  map[string]any
	results map[string]StepResult
	history []ContextEvent
}

// ContextEvent records a change to the context
type ContextEvent struct {
	Timestamp time.Time
	StepName  string
	Action    string // "set", "get", "result"
	Key       string
	Value     any
}

// NewContext creates a new workflow context
func NewContext() *Context {
	return &Context{
		values:  make(map[string]any),
		results: make(map[string]StepResult),
		history: []ContextEvent{},
	}
}

// Set stores a value in the context
func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.values[key] = value
	c.history = append(c.history, ContextEvent{
		Timestamp: time.Now(),
		Action:    "set",
		Key:       key,
		Value:     value,
	})
}

// Get retrieves a value from the context
func (c *Context) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	value, ok := c.values[key]
	return value, ok
}

// GetString retrieves a string value from the context
func (c *Context) GetString(key string) string {
	value, ok := c.Get(key)
	if !ok {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

// SetResult records the result of a workflow step
func (c *Context) SetResult(stepName string, result StepResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.results[stepName] = result
	c.history = append(c.history, ContextEvent{
		Timestamp: time.Now(),
		StepName:  stepName,
		Action:    "result",
		Value:     result,
	})
}

// GetResult retrieves the result of a workflow step
func (c *Context) GetResult(stepName string) (StepResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result, ok := c.results[stepName]
	return result, ok
}

// AllResults returns all step results
func (c *Context) AllResults() []StepResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make([]StepResult, 0, len(c.results))
	for _, r := range c.results {
		results = append(results, r)
	}
	return results
}

// ToMap exports the context values as a map
// This is useful for template rendering and condition evaluation
func (c *Context) ToMap() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m := make(map[string]any, len(c.values)+len(c.results))

	// Copy values
	for k, v := range c.values {
		m[k] = v
	}

	// Add step results as nested maps
	for name, result := range c.results {
		m[name] = map[string]any{
			"success":    result.Success,
			"output":     result.Output,
			"error":      result.Error,
			"loop_count": result.LoopCount,
		}
	}

	return m
}

// History returns all context events
func (c *Context) History() []ContextEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid race conditions
	h := make([]ContextEvent, len(c.history))
	copy(h, c.history)
	return h
}

// Clear removes all values and results
func (c *Context) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.values = make(map[string]any)
	c.results = make(map[string]StepResult)
	c.history = []ContextEvent{}
}
