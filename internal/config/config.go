package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all application configuration
type Config struct {
	// API Keys
	OpenAIKey      string `json:"openai_api_key,omitempty"`
	AnthropicKey   string `json:"anthropic_api_key,omitempty"`
	OpenRouterKey  string `json:"openrouter_api_key,omitempty"`

	// Defaults
	DefaultProvider string `json:"default_provider,omitempty"`
	DefaultModel    string `json:"default_model,omitempty"`
}

var (
	configDir  string
	configFile string
	current    *Config
)

func init() {
	// Use ~/.config/zcode for config
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	configDir = filepath.Join(home, ".config", "zcode")
	configFile = filepath.Join(configDir, "config.json")
}

// Load reads the config from disk
func Load() (*Config, error) {
	if current != nil {
		return current, nil
	}

	current = &Config{
		DefaultProvider: "claude",
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return current, nil // Return default config
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if err := json.Unmarshal(data, current); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return current, nil
}

// Save writes the config to disk
func Save(cfg *Config) error {
	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	current = cfg
	return nil
}

// Get returns the current config, loading if necessary
func Get() *Config {
	if current == nil {
		_, _ = Load()
	}
	return current
}

// Set updates a config value by key
func Set(key, value string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	switch key {
	case "openai_api_key", "openai":
		cfg.OpenAIKey = value
	case "anthropic_api_key", "anthropic":
		cfg.AnthropicKey = value
	case "openrouter_api_key", "openrouter":
		cfg.OpenRouterKey = value
	case "default_provider", "provider":
		cfg.DefaultProvider = value
	case "default_model", "model":
		cfg.DefaultModel = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	return Save(cfg)
}

// GetOpenAIKey returns the OpenAI API key (config or env)
func GetOpenAIKey() string {
	cfg := Get()
	if cfg.OpenAIKey != "" {
		return cfg.OpenAIKey
	}
	return os.Getenv("OPENAI_API_KEY")
}

// GetAnthropicKey returns the Anthropic API key (config or env)
func GetAnthropicKey() string {
	cfg := Get()
	if cfg.AnthropicKey != "" {
		return cfg.AnthropicKey
	}
	return os.Getenv("ANTHROPIC_API_KEY")
}

// GetOpenRouterKey returns the OpenRouter API key (config or env)
func GetOpenRouterKey() string {
	cfg := Get()
	if cfg.OpenRouterKey != "" {
		return cfg.OpenRouterKey
	}
	return os.Getenv("OPENROUTER_API_KEY")
}

// ConfigPath returns the path to the config file
func ConfigPath() string {
	return configFile
}

// ListKeys returns configured keys (masked for display)
func ListKeys() map[string]string {
	cfg := Get()
	result := make(map[string]string)

	if cfg.OpenAIKey != "" {
		result["openai_api_key"] = maskKey(cfg.OpenAIKey)
	} else if os.Getenv("OPENAI_API_KEY") != "" {
		result["openai_api_key"] = maskKey(os.Getenv("OPENAI_API_KEY")) + " (env)"
	}

	if cfg.AnthropicKey != "" {
		result["anthropic_api_key"] = maskKey(cfg.AnthropicKey)
	} else if os.Getenv("ANTHROPIC_API_KEY") != "" {
		result["anthropic_api_key"] = maskKey(os.Getenv("ANTHROPIC_API_KEY")) + " (env)"
	}

	if cfg.OpenRouterKey != "" {
		result["openrouter_api_key"] = maskKey(cfg.OpenRouterKey)
	} else if os.Getenv("OPENROUTER_API_KEY") != "" {
		result["openrouter_api_key"] = maskKey(os.Getenv("OPENROUTER_API_KEY")) + " (env)"
	}

	if cfg.DefaultProvider != "" {
		result["default_provider"] = cfg.DefaultProvider
	}

	if cfg.DefaultModel != "" {
		result["default_model"] = cfg.DefaultModel
	}

	return result
}

// maskKey shows only first 4 and last 4 characters
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// Delete removes a config value
func Delete(key string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	switch key {
	case "openai_api_key", "openai":
		cfg.OpenAIKey = ""
	case "anthropic_api_key", "anthropic":
		cfg.AnthropicKey = ""
	case "openrouter_api_key", "openrouter":
		cfg.OpenRouterKey = ""
	case "default_provider", "provider":
		cfg.DefaultProvider = ""
	case "default_model", "model":
		cfg.DefaultModel = ""
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	return Save(cfg)
}

// GetAgentPaths returns paths to search for custom agent definitions
// Returns both project-local (.zcode/agents/) and global (~/.config/zcode/agents/) paths
func GetAgentPaths() []string {
	paths := []string{}

	// Project-local path
	cwd, err := os.Getwd()
	if err == nil {
		paths = append(paths, filepath.Join(cwd, ".zcode", "agents"))
	}

	// Global config path
	paths = append(paths, filepath.Join(configDir, "agents"))

	return paths
}

// GetWorkflowPaths returns paths to search for workflow definitions
// Returns both project-local (.zcode/workflows/) and global (~/.config/zcode/workflows/) paths
func GetWorkflowPaths() []string {
	paths := []string{}

	// Project-local path
	cwd, err := os.Getwd()
	if err == nil {
		paths = append(paths, filepath.Join(cwd, ".zcode", "workflows"))
	}

	// Global config path
	paths = append(paths, filepath.Join(configDir, "workflows"))

	return paths
}

// GetSkillPaths returns paths to search for skill definitions
// Returns both project-local (.zcode/skills/) and global (~/.config/zcode/skills/) paths
func GetSkillPaths() []string {
	paths := []string{}

	// Project-local path
	cwd, err := os.Getwd()
	if err == nil {
		paths = append(paths, filepath.Join(cwd, ".zcode", "skills"))
	}

	// Global config path
	paths = append(paths, filepath.Join(configDir, "skills"))

	return paths
}
