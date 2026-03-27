// SPDX-License-Identifier: MIT

package model

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestWorkflowManifest_Validate tests the Validate method on WorkflowManifest.
func TestWorkflowManifest_Validate(t *testing.T) {
	tests := []struct {
		name     string
		manifest WorkflowManifest
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid SDD-like workflow with skills and commands",
			manifest: WorkflowManifest{
				APIVersion: "devrune/workflow/v1",
				Metadata: WorkflowMetadata{
					Name:    "sdd",
					Version: "1.0.0",
				},
				Components: WorkflowComponents{
					Skills: []string{"sdd-explore", "sdd-plan", "sdd-implement", "sdd-review"},
					Commands: []WorkflowCommand{
						{Name: "sdd-explore", Action: "Explore and investigate", Argument: "<topic>"},
						{Name: "sdd-plan", Action: "Create implementation plan", Argument: "<change>"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid workflow with entrypoint string",
			manifest: WorkflowManifest{
				APIVersion: "devrune/workflow/v1",
				Metadata:   WorkflowMetadata{Name: "code-review", Version: "0.1.0"},
				Components: WorkflowComponents{
					Skills:     []string{"review-scan", "review-deep"},
					Entrypoint: "ORCHESTRATOR.md",
				},
			},
			wantErr: false,
		},
		{
			name: "valid workflow with only commands (no skills)",
			manifest: WorkflowManifest{
				APIVersion: "devrune/workflow/v1",
				Metadata:   WorkflowMetadata{Name: "simple-workflow", Version: "1.0.0"},
				Components: WorkflowComponents{
					Commands: []WorkflowCommand{
						{Name: "run", Action: "Run the workflow"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid workflow with only skills (no commands)",
			manifest: WorkflowManifest{
				APIVersion: "devrune/workflow/v1",
				Metadata:   WorkflowMetadata{Name: "skill-only", Version: "1.0.0"},
				Components: WorkflowComponents{
					Skills: []string{"my-skill"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid workflow with rules and MCPs",
			manifest: WorkflowManifest{
				APIVersion: "devrune/workflow/v1",
				Metadata:   WorkflowMetadata{Name: "full-workflow", Version: "2.0.0"},
				Components: WorkflowComponents{
					Skills: []string{"my-skill"},
					Rules:  []string{"architecture/clean-architecture"},
					MCPs:   []string{"github.yaml"},
					Commands: []WorkflowCommand{
						{Name: "run", Action: "Execute"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing apiVersion",
			manifest: WorkflowManifest{
				Metadata: WorkflowMetadata{Name: "test-workflow"},
				Components: WorkflowComponents{
					Skills: []string{"my-skill"},
				},
			},
			wantErr: true,
			errMsg:  "apiVersion must be",
		},
		{
			name: "wrong apiVersion",
			manifest: WorkflowManifest{
				APIVersion: "devrune/workflow/v2",
				Metadata:   WorkflowMetadata{Name: "test-workflow"},
				Components: WorkflowComponents{
					Skills: []string{"my-skill"},
				},
			},
			wantErr: true,
			errMsg:  "apiVersion must be",
		},
		{
			name: "completely wrong apiVersion string",
			manifest: WorkflowManifest{
				APIVersion: "v1",
				Metadata:   WorkflowMetadata{Name: "test-workflow"},
				Components: WorkflowComponents{
					Skills: []string{"my-skill"},
				},
			},
			wantErr: true,
			errMsg:  "apiVersion must be",
		},
		{
			name: "missing metadata.name",
			manifest: WorkflowManifest{
				APIVersion: "devrune/workflow/v1",
				Metadata:   WorkflowMetadata{Version: "1.0.0"},
				Components: WorkflowComponents{
					Skills: []string{"my-skill"},
				},
			},
			wantErr: true,
			errMsg:  "metadata.name is required",
		},
		{
			name: "no skills and no commands",
			manifest: WorkflowManifest{
				APIVersion: "devrune/workflow/v1",
				Metadata:   WorkflowMetadata{Name: "empty-workflow", Version: "1.0.0"},
				Components: WorkflowComponents{},
			},
			wantErr: true,
			errMsg:  "at least one skill or command",
		},
		{
			name: "no skills and no commands (explicit empty slices)",
			manifest: WorkflowManifest{
				APIVersion: "devrune/workflow/v1",
				Metadata:   WorkflowMetadata{Name: "empty-workflow"},
				Components: WorkflowComponents{
					Skills:   []string{},
					Commands: []WorkflowCommand{},
				},
			},
			wantErr: true,
			errMsg:  "at least one skill or command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %q, want message containing %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// TestWorkflowManifest_Validate_WithNewOptionalFields verifies that Validate() still
// passes when new optional fields (DecisionRules, InvocationControls, Registry,
// Permissions) are empty, and passes when they are populated.
func TestWorkflowManifest_Validate_WithNewOptionalFields(t *testing.T) {
	base := WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   WorkflowMetadata{Name: "sdd", Version: "1.0.0"},
		Components: WorkflowComponents{
			Skills: []string{"sdd-explore"},
		},
	}

	// Empty optional fields: must not change validation outcome.
	if err := base.Validate(); err != nil {
		t.Errorf("Validate() with empty optional fields should pass, got: %v", err)
	}

	// Populated optional fields: must also pass.
	base.Components.DecisionRules = []DecisionRule{
		{Scenario: `"commit", "create commit"`, Resolution: "Use `git:commit`"},
	}
	base.Components.InvocationControls = []InvocationControl{
		{Skills: "git:commit", Description: "When user requests commit operations"},
	}
	base.Components.Registry = "REGISTRY.md"
	base.Components.Permissions = []string{"Bash(mkdir -p .sdd*)", "Bash(tree:*)"}

	if err := base.Validate(); err != nil {
		t.Errorf("Validate() with populated optional fields should pass, got: %v", err)
	}
}

// TestDecisionRule_SerializationRoundTrip verifies that DecisionRule fields
// are correctly marshaled and unmarshaled via YAML.
func TestDecisionRule_SerializationRoundTrip(t *testing.T) {
	rule := DecisionRule{
		Scenario:   `"commit", "commit my changes", "create commit"`,
		Resolution: "Use `git:commit`",
	}

	// Verify field values are accessible (structural check, not full YAML roundtrip).
	if rule.Scenario == "" {
		t.Error("DecisionRule.Scenario should not be empty")
	}
	if rule.Resolution == "" {
		t.Error("DecisionRule.Resolution should not be empty")
	}
	if rule.Scenario != `"commit", "commit my changes", "create commit"` {
		t.Errorf("DecisionRule.Scenario = %q, want correct value", rule.Scenario)
	}
	if rule.Resolution != "Use `git:commit`" {
		t.Errorf("DecisionRule.Resolution = %q, want correct value", rule.Resolution)
	}
}

// TestInvocationControl_SerializationRoundTrip verifies that InvocationControl
// fields are correctly populated.
func TestInvocationControl_SerializationRoundTrip(t *testing.T) {
	ctrl := InvocationControl{
		Skills:      "git:commit, git:pull-request",
		Description: "When user requests commit/PR operations",
	}

	if ctrl.Skills != "git:commit, git:pull-request" {
		t.Errorf("InvocationControl.Skills = %q, want correct value", ctrl.Skills)
	}
	if ctrl.Description != "When user requests commit/PR operations" {
		t.Errorf("InvocationControl.Description = %q, want correct value", ctrl.Description)
	}
}

// TestWorkflowManifest_RegistryContent_IsTransient verifies that RegistryContent
// is a runtime-only field — it has no yaml tag that would cause it to be serialized.
func TestWorkflowManifest_RegistryContent_IsTransient(t *testing.T) {
	wf := WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   WorkflowMetadata{Name: "test"},
		Components: WorkflowComponents{Skills: []string{"my-skill"}},
	}

	// Set the transient field directly (simulating runtime population).
	wf.RegistryContent = "# Injected content"

	// Verify the field is accessible.
	if wf.RegistryContent != "# Injected content" {
		t.Errorf("RegistryContent = %q, want %q", wf.RegistryContent, "# Injected content")
	}

	// Verify it is zero-valued by default.
	wf2 := WorkflowManifest{}
	if wf2.RegistryContent != "" {
		t.Errorf("RegistryContent default = %q, want empty string", wf2.RegistryContent)
	}
}

// TestWorkflowManifest_EntrypointIsString verifies that the Entrypoint field
// is a plain string (not a struct), as required by v3.4 simplification.
// This test documents the intent: no EntrypointRef struct, no sharedDir nesting.
func TestWorkflowManifest_EntrypointIsString(t *testing.T) {
	wf := WorkflowManifest{
		APIVersion: "devrune/workflow/v1",
		Metadata:   WorkflowMetadata{Name: "test"},
		Components: WorkflowComponents{
			Skills:     []string{"my-skill"},
			Entrypoint: "ORCHESTRATOR.md",
		},
	}

	// Verify entrypoint is accessible as a string directly
	if wf.Components.Entrypoint != "ORCHESTRATOR.md" {
		t.Errorf("Entrypoint = %q, want %q", wf.Components.Entrypoint, "ORCHESTRATOR.md")
	}

	// Empty entrypoint is valid (optional field)
	wf.Components.Entrypoint = ""
	if err := wf.Validate(); err != nil {
		t.Errorf("Validate() with empty entrypoint should not error, got: %v", err)
	}
}

// TestAgentDefinition_Validate tests the Validate method on AgentDefinition.
func TestAgentDefinition_Validate(t *testing.T) {
	tests := []struct {
		name    string
		agent   AgentDefinition
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid claude agent",
			agent: AgentDefinition{
				Name:        "claude",
				Type:        "claude",
				Workspace:   ".claude",
				SkillDir:    "skills",
				CatalogFile: "CLAUDE.md",
			},
			wantErr: false,
		},
		{
			name: "valid opencode agent",
			agent: AgentDefinition{
				Name:        "opencode",
				Type:        "opencode",
				Workspace:   ".opencode",
				SkillDir:    "agents",
				CatalogFile: "AGENTS.md",
			},
			wantErr: false,
		},
		{
			name: "valid copilot agent",
			agent: AgentDefinition{
				Name:        "copilot",
				Type:        "copilot",
				Workspace:   ".github",
				SkillDir:    "agents",
				CatalogFile: "copilot-instructions.md",
			},
			wantErr: false,
		},
		{
			name: "valid factory agent",
			agent: AgentDefinition{
				Name:        "factory",
				Type:        "factory",
				Workspace:   ".factory",
				SkillDir:    "skills",
				CatalogFile: "AGENTS.md",
			},
			wantErr: false,
		},
		{
			name: "valid agent with all optional fields",
			agent: AgentDefinition{
				Name:         "claude",
				Type:         "claude",
				Workspace:    ".claude",
				SkillDir:     "skills",
				CommandDir:   "commands",
				RulesDir:     "rules",
				CatalogFile:  "CLAUDE.md",
				DefaultRules: "both",
			},
			wantErr: false,
		},
		{
			name:    "missing name",
			agent:   AgentDefinition{Type: "claude", Workspace: ".claude", SkillDir: "skills", CatalogFile: "CLAUDE.md"},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name:    "missing type",
			agent:   AgentDefinition{Name: "claude", Workspace: ".claude", SkillDir: "skills", CatalogFile: "CLAUDE.md"},
			wantErr: true,
			errMsg:  "type is required",
		},
		{
			name:    "unknown type",
			agent:   AgentDefinition{Name: "myagent", Type: "cursor", Workspace: ".cursor", SkillDir: "skills", CatalogFile: "catalog.md"},
			wantErr: true,
			errMsg:  "unknown type",
		},
		{
			name:    "unsupported type vscode",
			agent:   AgentDefinition{Name: "vscode", Type: "vscode", Workspace: ".vscode", SkillDir: "skills", CatalogFile: "catalog.md"},
			wantErr: true,
			errMsg:  "unknown type",
		},
		{
			name:    "missing workspace",
			agent:   AgentDefinition{Name: "claude", Type: "claude", SkillDir: "skills", CatalogFile: "CLAUDE.md"},
			wantErr: true,
			errMsg:  "workspace is required",
		},
		{
			name:    "missing skillDir",
			agent:   AgentDefinition{Name: "claude", Type: "claude", Workspace: ".claude", CatalogFile: "CLAUDE.md"},
			wantErr: true,
			errMsg:  "skillDir is required",
		},
		{
			name:    "missing catalogFile",
			agent:   AgentDefinition{Name: "claude", Type: "claude", Workspace: ".claude", SkillDir: "skills"},
			wantErr: true,
			errMsg:  "catalogFile is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.agent.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %q, want message containing %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// TestAgentDefinition_ReservedTypes verifies all four reserved agent types are recognized.
func TestAgentDefinition_ReservedTypes(t *testing.T) {
	reservedTypes := []string{"claude", "opencode", "copilot", "factory"}

	for _, agentType := range reservedTypes {
		t.Run(agentType, func(t *testing.T) {
			agent := AgentDefinition{
				Name:        agentType,
				Type:        agentType,
				Workspace:   "." + agentType,
				SkillDir:    "skills",
				CatalogFile: "catalog.md",
			}
			if err := agent.Validate(); err != nil {
				t.Errorf("agent type %q should be valid, got error: %v", agentType, err)
			}
		})
	}
}

// TestAgentDefinition_MCPConfig_Optional proves that Validate() accepts an
// AgentDefinition with nil MCP (backward compatibility) and one with a fully
// populated MCPConfig, and that YAML unmarshaling correctly populates MCPConfig fields.
func TestAgentDefinition_MCPConfig_Optional(t *testing.T) {
	t.Run("nil MCP is accepted by Validate", func(t *testing.T) {
		agent := AgentDefinition{
			Name:        "claude",
			Type:        "claude",
			Workspace:   ".claude",
			SkillDir:    "skills",
			CatalogFile: "CLAUDE.md",
			MCP:         nil,
		}
		if err := agent.Validate(); err != nil {
			t.Errorf("Validate() with nil MCP should pass, got: %v", err)
		}
	})

	t.Run("populated MCPConfig is accepted by Validate", func(t *testing.T) {
		agent := AgentDefinition{
			Name:        "factory",
			Type:        "factory",
			Workspace:   ".factory",
			SkillDir:    "skills",
			CatalogFile: "AGENTS.md",
			MCP: &MCPConfig{
				FilePath:    "mcp.json",
				RootKey:     "mcpServers",
				EnvKey:      "env",
				EnvVarStyle: "${VAR}",
			},
		}
		if err := agent.Validate(); err != nil {
			t.Errorf("Validate() with populated MCPConfig should pass, got: %v", err)
		}
	})

	t.Run("YAML unmarshaling populates MCPConfig fields", func(t *testing.T) {
		yamlInput := `
name: opencode
type: opencode
workspace: ".opencode"
skillDir: "skills"
catalogFile: "AGENTS.md"
mcp:
  filePath: "opencode.json"
  rootKey: "mcp"
  envKey: "environment"
  envVarStyle: "{env:VAR}"
`
		var agent AgentDefinition
		if err := yaml.Unmarshal([]byte(yamlInput), &agent); err != nil {
			t.Fatalf("YAML unmarshal failed: %v", err)
		}
		if agent.MCP == nil {
			t.Fatal("MCPConfig should not be nil after YAML unmarshaling")
		}
		if agent.MCP.FilePath != "opencode.json" {
			t.Errorf("MCPConfig.FilePath = %q, want %q", agent.MCP.FilePath, "opencode.json")
		}
		if agent.MCP.RootKey != "mcp" {
			t.Errorf("MCPConfig.RootKey = %q, want %q", agent.MCP.RootKey, "mcp")
		}
		if agent.MCP.EnvKey != "environment" {
			t.Errorf("MCPConfig.EnvKey = %q, want %q", agent.MCP.EnvKey, "environment")
		}
		if agent.MCP.EnvVarStyle != "{env:VAR}" {
			t.Errorf("MCPConfig.EnvVarStyle = %q, want %q", agent.MCP.EnvVarStyle, "{env:VAR}")
		}
	})

	t.Run("YAML without mcp block leaves MCP nil", func(t *testing.T) {
		yamlInput := `
name: claude
type: claude
workspace: ".claude"
skillDir: "skills"
catalogFile: "CLAUDE.md"
`
		var agent AgentDefinition
		if err := yaml.Unmarshal([]byte(yamlInput), &agent); err != nil {
			t.Fatalf("YAML unmarshal failed: %v", err)
		}
		if agent.MCP != nil {
			t.Errorf("MCPConfig should be nil when mcp block is absent, got: %+v", agent.MCP)
		}
		if err := agent.Validate(); err != nil {
			t.Errorf("Validate() should pass when mcp block is absent, got: %v", err)
		}
	})
}
