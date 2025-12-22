package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMaskKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "short key",
			key:      "abc",
			expected: "****",
		},
		{
			name:     "exactly 8 chars",
			key:      "12345678",
			expected: "****",
		},
		{
			name:     "long key",
			key:      "sk-1234567890abcdef",
			expected: "sk-1...cdef",
		},
		{
			name:     "empty key",
			key:      "",
			expected: "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskKey(tt.key)
			if result != tt.expected {
				t.Errorf("maskKey(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestConfigLoadSave(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir, err := os.MkdirTemp("", "zcode-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config paths for testing
	oldConfigDir := configDir
	oldConfigFile := configFile
	configDir = tmpDir
	configFile = filepath.Join(tmpDir, "config.json")
	current = nil // Reset cached config
	defer func() {
		configDir = oldConfigDir
		configFile = oldConfigFile
		current = nil
	}()

	// Test loading non-existent config (should return defaults)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DefaultProvider != "claude" {
		t.Errorf("default provider = %q, want %q", cfg.DefaultProvider, "claude")
	}

	// Test saving config
	cfg.OpenAIKey = "test-key-12345"
	cfg.DefaultModel = "gpt-4o"
	err = Save(cfg)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Reset cache and reload
	current = nil
	cfg2, err := Load()
	if err != nil {
		t.Fatalf("Load() after save error = %v", err)
	}
	if cfg2.OpenAIKey != "test-key-12345" {
		t.Errorf("OpenAIKey = %q, want %q", cfg2.OpenAIKey, "test-key-12345")
	}
	if cfg2.DefaultModel != "gpt-4o" {
		t.Errorf("DefaultModel = %q, want %q", cfg2.DefaultModel, "gpt-4o")
	}
}

func TestConfigSet(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir, err := os.MkdirTemp("", "zcode-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config paths for testing
	oldConfigDir := configDir
	oldConfigFile := configFile
	configDir = tmpDir
	configFile = filepath.Join(tmpDir, "config.json")
	current = nil
	defer func() {
		configDir = oldConfigDir
		configFile = oldConfigFile
		current = nil
	}()

	tests := []struct {
		key   string
		value string
		check func(*Config) bool
	}{
		{
			key:   "openai",
			value: "sk-test123",
			check: func(c *Config) bool { return c.OpenAIKey == "sk-test123" },
		},
		{
			key:   "provider",
			value: "openai",
			check: func(c *Config) bool { return c.DefaultProvider == "openai" },
		},
		{
			key:   "model",
			value: "gpt-4-turbo",
			check: func(c *Config) bool { return c.DefaultModel == "gpt-4-turbo" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := Set(tt.key, tt.value)
			if err != nil {
				t.Fatalf("Set(%q, %q) error = %v", tt.key, tt.value, err)
			}

			cfg := Get()
			if !tt.check(cfg) {
				t.Errorf("Set(%q, %q) did not update config correctly", tt.key, tt.value)
			}
		})
	}

	// Test unknown key
	err = Set("unknown_key", "value")
	if err == nil {
		t.Error("Set() with unknown key should return error")
	}
}

func TestConfigDelete(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir, err := os.MkdirTemp("", "zcode-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config paths for testing
	oldConfigDir := configDir
	oldConfigFile := configFile
	configDir = tmpDir
	configFile = filepath.Join(tmpDir, "config.json")
	current = nil
	defer func() {
		configDir = oldConfigDir
		configFile = oldConfigFile
		current = nil
	}()

	// Set a value first
	err = Set("openai", "sk-test123")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Delete the value
	err = Delete("openai")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	cfg := Get()
	if cfg.OpenAIKey != "" {
		t.Errorf("OpenAIKey = %q after delete, want empty", cfg.OpenAIKey)
	}

	// Test unknown key
	err = Delete("unknown_key")
	if err == nil {
		t.Error("Delete() with unknown key should return error")
	}
}

func TestGetOpenAIKeyFromEnv(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir, err := os.MkdirTemp("", "zcode-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config paths for testing
	oldConfigDir := configDir
	oldConfigFile := configFile
	configDir = tmpDir
	configFile = filepath.Join(tmpDir, "config.json")
	current = nil
	defer func() {
		configDir = oldConfigDir
		configFile = oldConfigFile
		current = nil
	}()

	// Set env var
	oldEnv := os.Getenv("OPENAI_API_KEY")
	os.Setenv("OPENAI_API_KEY", "env-test-key")
	defer os.Setenv("OPENAI_API_KEY", oldEnv)

	// Should return env var when config is empty
	key := GetOpenAIKey()
	if key != "env-test-key" {
		t.Errorf("GetOpenAIKey() = %q, want %q", key, "env-test-key")
	}

	// Set config value - should take precedence
	if err := Set("openai", "config-test-key"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	key = GetOpenAIKey()
	if key != "config-test-key" {
		t.Errorf("GetOpenAIKey() with config = %q, want %q", key, "config-test-key")
	}
}

func TestConfigPath(t *testing.T) {
	path := ConfigPath()
	if path == "" {
		t.Error("ConfigPath() returned empty string")
	}
}
