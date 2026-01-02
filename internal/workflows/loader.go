package workflows

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Loader handles discovery and parsing of workflow definitions from YAML files
type Loader struct {
	paths      []string
	globalPath string // The known global config path
}

// NewLoader creates a new workflow loader with the given search paths
func NewLoader(paths []string) *Loader {
	globalPath := ""
	if home, err := os.UserHomeDir(); err == nil {
		globalPath = filepath.Join(home, ".config", "zcode", "workflows")
	}
	return &Loader{paths: paths, globalPath: globalPath}
}

// LoadAll discovers and loads all workflow definitions from all configured paths
func (l *Loader) LoadAll() ([]*WorkflowDefinition, error) {
	var workflows []*WorkflowDefinition

	for _, basePath := range l.paths {
		// Determine if this is the global path by comparing canonical paths
		isGlobal := l.globalPath != "" && basePath == l.globalPath

		// Check if directory exists
		info, err := os.Stat(basePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Skip non-existent directories
			}
			return nil, fmt.Errorf("error accessing %s: %w", basePath, err)
		}
		if !info.IsDir() {
			continue
		}

		// Find all .yaml and .yml files in the directory
		entries, err := os.ReadDir(basePath)
		if err != nil {
			return nil, fmt.Errorf("error reading directory %s: %w", basePath, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
				continue
			}

			filePath := filepath.Join(basePath, name)
			workflow, err := l.LoadFromFile(filePath)
			if err != nil {
				// Log but don't fail on individual file errors
				fmt.Fprintf(os.Stderr, "Warning: failed to load workflow from %s: %v\n", filePath, err)
				continue
			}

			workflow.IsGlobal = isGlobal
			workflows = append(workflows, workflow)
		}
	}

	return workflows, nil
}

// LoadFromFile parses a single YAML workflow file
func (l *Loader) LoadFromFile(filePath string) (*WorkflowDefinition, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	var workflow WorkflowDefinition
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		return nil, fmt.Errorf("error parsing YAML: %w", err)
	}

	workflow.FilePath = filePath

	if err := workflow.Validate(); err != nil {
		return nil, err
	}

	return &workflow, nil
}

// DefaultPaths returns the default workflow search paths
func DefaultWorkflowPaths() []string {
	paths := []string{}

	// Project-local path
	cwd, err := os.Getwd()
	if err == nil {
		paths = append(paths, filepath.Join(cwd, ".zcode", "workflows"))
	}

	// Global config path
	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".config", "zcode", "workflows"))
	}

	return paths
}

// Registry manages loaded workflows
type Registry struct {
	mu        sync.RWMutex
	workflows map[string]*WorkflowDefinition
	loader    *Loader
}

// NewRegistry creates a new workflow registry with default paths
func NewRegistry() *Registry {
	return &Registry{
		workflows: make(map[string]*WorkflowDefinition),
		loader:    NewLoader(DefaultWorkflowPaths()),
	}
}

// NewRegistryWithPaths creates a new workflow registry with custom paths
func NewRegistryWithPaths(paths []string) *Registry {
	return &Registry{
		workflows: make(map[string]*WorkflowDefinition),
		loader:    NewLoader(paths),
	}
}

// Refresh reloads all workflows from disk
func (r *Registry) Refresh() error {
	workflows, err := r.loader.LoadAll()
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear existing workflows
	r.workflows = make(map[string]*WorkflowDefinition)

	// Add newly loaded workflows
	for _, workflow := range workflows {
		r.workflows[workflow.Name] = workflow
	}

	return nil
}

// Get returns a workflow by name
func (r *Registry) Get(name string) (*WorkflowDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	workflow, ok := r.workflows[name]
	return workflow, ok
}

// List returns all loaded workflows
func (r *Registry) List() []*WorkflowDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	workflows := make([]*WorkflowDefinition, 0, len(r.workflows))
	for _, workflow := range r.workflows {
		workflows = append(workflows, workflow)
	}
	return workflows
}

// Count returns the number of loaded workflows
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.workflows)
}

// Names returns the names of all registered workflows
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.workflows))
	for name := range r.workflows {
		names = append(names, name)
	}
	return names
}
