package skills

import (
	"sync"
)

// Registry manages skill discovery and lookup
type Registry struct {
	mu     sync.RWMutex
	skills map[string]*SkillDefinition
	loader *Loader
}

// NewRegistry creates a new skill registry with the given loader
func NewRegistry(loader *Loader) *Registry {
	return &Registry{
		skills: make(map[string]*SkillDefinition),
		loader: loader,
	}
}

// Refresh reloads all skills from disk
func (r *Registry) Refresh() error {
	skills, err := r.loader.LoadAll()
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.skills = make(map[string]*SkillDefinition)
	for _, skill := range skills {
		r.skills[skill.Name] = skill
	}

	return nil
}

// Get returns a skill by name
func (r *Registry) Get(name string) (*SkillDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, ok := r.skills[name]
	return skill, ok
}

// List returns all registered skills
func (r *Registry) List() []*SkillDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skills := make([]*SkillDefinition, 0, len(r.skills))
	for _, skill := range r.skills {
		skills = append(skills, skill)
	}
	return skills
}

// ListByTag returns skills that have the given tag
func (r *Registry) ListByTag(tag string) []*SkillDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var skills []*SkillDefinition
	for _, skill := range r.skills {
		for _, t := range skill.Tags {
			if t == tag {
				skills = append(skills, skill)
				break
			}
		}
	}
	return skills
}

// Count returns the number of registered skills
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.skills)
}
