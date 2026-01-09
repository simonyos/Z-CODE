package swarm

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SwarmConfig contains global swarm configuration.
type SwarmConfig struct {
	NATS          NATSConfig          `json:"nats" yaml:"nats"`
	Defaults      DefaultsConfig      `json:"defaults" yaml:"defaults"`
	Roles         map[Role]RoleConfig `json:"roles,omitempty" yaml:"roles,omitempty"`
	CustomRolesDir string             `json:"custom_roles_dir,omitempty" yaml:"custom_roles_dir,omitempty"` // Path to custom role definitions
}

// DefaultsConfig contains default settings for swarm sessions.
type DefaultsConfig struct {
	AutoPilot       bool `json:"auto_pilot" yaml:"auto_pilot"`
	RequireApproval bool `json:"require_approval" yaml:"require_approval"`
	HistoryLimit    int  `json:"history_limit" yaml:"history_limit"`
}

// RoleConfig contains per-role configuration overrides.
type RoleConfig struct {
	LLMProvider string `json:"llm_provider,omitempty" yaml:"llm_provider,omitempty"`
	LLMModel    string `json:"llm_model,omitempty" yaml:"llm_model,omitempty"`
	MaxTokens   int    `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
}

// DefaultSwarmConfig returns the default swarm configuration.
func DefaultSwarmConfig() *SwarmConfig {
	return &SwarmConfig{
		NATS: DefaultNATSConfig(),
		Defaults: DefaultsConfig{
			AutoPilot:       true,
			RequireApproval: false,
			HistoryLimit:    100,
		},
		Roles: make(map[Role]RoleConfig),
	}
}

// ConfigPath returns the default configuration file path.
func ConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "zcode", "swarm.json")
}

// LoadConfig loads the swarm configuration from file.
func LoadConfig() (*SwarmConfig, error) {
	path := ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultSwarmConfig(), nil
		}
		return nil, err
	}

	var config SwarmConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveConfig saves the swarm configuration to file.
func SaveConfig(config *SwarmConfig) error {
	path := ConfigPath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetRoleConfig returns the configuration for a specific role.
func (c *SwarmConfig) GetRoleConfig(role Role) RoleConfig {
	if config, exists := c.Roles[role]; exists {
		return config
	}
	return RoleConfig{}
}

// SetRoleConfig sets the configuration for a specific role.
func (c *SwarmConfig) SetRoleConfig(role Role, config RoleConfig) {
	if c.Roles == nil {
		c.Roles = make(map[Role]RoleConfig)
	}
	c.Roles[role] = config
}

// LoadRoleDefinitions loads role definitions, preferring custom directory if configured.
func (c *SwarmConfig) LoadRoleDefinitions() (map[Role]*RoleDefinition, error) {
	return LoadRoles(c.CustomRolesDir)
}

// CustomRolesPath returns the default path for custom roles.
func CustomRolesPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "zcode", "roles")
}
