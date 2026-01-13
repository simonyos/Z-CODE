package swarm

import (
	"strings"
	"testing"
)

func TestParseRoleMarkdown(t *testing.T) {
	content := `# Test Role (TEST)

**Description:** A test role for testing.

## Capabilities

- Capability one
- Capability two
- Capability three

## Permissions

- Can message ORCH
- Can do things

## System Prompt

You are a test role. Your job is to:

1. Do testing
2. Be tested
3. Test things

Always be thorough.
`

	def, err := parseRoleMarkdown(content)
	if err != nil {
		t.Fatalf("parseRoleMarkdown failed: %v", err)
	}

	if def.Name != "Test Role" {
		t.Errorf("Expected name 'Test Role', got '%s'", def.Name)
	}

	if def.Role != "TEST" {
		t.Errorf("Expected role 'TEST', got '%s'", def.Role)
	}

	if def.Description != "A test role for testing." {
		t.Errorf("Expected description 'A test role for testing.', got '%s'", def.Description)
	}

	if len(def.Capabilities) != 3 {
		t.Errorf("Expected 3 capabilities, got %d", len(def.Capabilities))
	}

	if len(def.Permissions) != 2 {
		t.Errorf("Expected 2 permissions, got %d", len(def.Permissions))
	}

	if !strings.Contains(def.SystemPrompt, "You are a test role") {
		t.Errorf("System prompt should contain 'You are a test role', got '%s'", def.SystemPrompt)
	}
}

func TestMapRoleName(t *testing.T) {
	tests := []struct {
		input    string
		expected Role
	}{
		{"ORCH", RoleOrchestrator},
		{"ORCHESTRATOR", RoleOrchestrator},
		{"SA", RoleSA},
		{"SOLUTION_ARCHITECT", RoleSA},
		{"BE_DEV", RoleBEDev},
		{"BACKEND", RoleBEDev},
		{"FE_DEV", RoleFEDev},
		{"FRONTEND", RoleFEDev},
		{"QA", RoleQA},
		{"DEVOPS", RoleDevOps},
		{"DBA", RoleDBA},
		{"SEC", RoleSecurity},
		{"SECURITY", RoleSecurity},
		{"HUMAN", RoleHuman},
	}

	for _, tc := range tests {
		result := mapRoleName(tc.input)
		if result != tc.expected {
			t.Errorf("mapRoleName(%s) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestLoadEmbeddedRoles(t *testing.T) {
	loader := NewRoleLoader("")
	err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	roles := loader.GetAll()
	if len(roles) == 0 {
		t.Fatal("No roles loaded from embedded files")
	}

	// Check that key roles are present
	expectedRoles := []Role{
		RoleOrchestrator,
		RoleSA,
		RoleBEDev,
		RoleFEDev,
		RoleQA,
	}

	for _, role := range expectedRoles {
		def := loader.Get(role)
		if def == nil {
			t.Errorf("Expected role %s to be loaded", role)
			continue
		}

		if def.Name == "" {
			t.Errorf("Role %s has empty name", role)
		}

		if def.SystemPrompt == "" {
			t.Errorf("Role %s has empty system prompt", role)
		}
	}
}

func TestDefaultRolesLoadsFromEmbedded(t *testing.T) {
	roles := DefaultRoles()
	if len(roles) == 0 {
		t.Fatal("DefaultRoles returned no roles")
	}

	// Check orchestrator is loaded correctly
	orch := roles[RoleOrchestrator]
	if orch == nil {
		t.Fatal("Orchestrator role not found")
	}

	if !orch.CanInitiate {
		t.Error("Orchestrator should be able to initiate")
	}

	if !orch.CanApprove {
		t.Error("Orchestrator should be able to approve")
	}
}
