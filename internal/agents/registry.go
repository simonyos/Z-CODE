package agents

import (
	"sync"
)

// Registry manages loaded custom agents
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*AgentDefinition
	loader *Loader
}

// NewRegistry creates a new agent registry with default paths
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*AgentDefinition),
		loader: NewLoader(DefaultPaths()),
	}
}

// NewRegistryWithPaths creates a new agent registry with custom paths
func NewRegistryWithPaths(paths []string) *Registry {
	return &Registry{
		agents: make(map[string]*AgentDefinition),
		loader: NewLoader(paths),
	}
}

// Refresh reloads all agents from disk
func (r *Registry) Refresh() error {
	agents, err := r.loader.LoadAll()
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear existing agents
	r.agents = make(map[string]*AgentDefinition)

	// Add newly loaded agents
	for _, agent := range agents {
		r.agents[agent.Name] = agent
	}

	return nil
}

// Get returns an agent by name
func (r *Registry) Get(name string) (*AgentDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, ok := r.agents[name]
	return agent, ok
}

// List returns all loaded agents
func (r *Registry) List() []*AgentDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]*AgentDefinition, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}
	return agents
}

// Count returns the number of loaded agents
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

// Register manually adds an agent to the registry
// This is useful for testing or programmatically defined agents
func (r *Registry) Register(agent *AgentDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[agent.Name] = agent
}

// Unregister removes an agent from the registry
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.agents, name)
}

// Names returns the names of all registered agents
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}
