package materialize_test

import (
	"testing"

	"github.com/davidarce/devrune/internal/materialize"
)

// TestLoadBuiltinAgents_AllAgentsLoad verifies that all four built-in agent YAML
// files can be loaded, parsed, and pass validation without error.
func TestLoadBuiltinAgents_AllAgentsLoad(t *testing.T) {
	agents, err := materialize.LoadBuiltinAgents()
	if err != nil {
		t.Fatalf("LoadBuiltinAgents() failed: %v", err)
	}

	if len(agents) == 0 {
		t.Fatal("LoadBuiltinAgents() returned no agents")
	}

	// We expect exactly 4 built-in agents.
	if len(agents) != 4 {
		t.Errorf("LoadBuiltinAgents() returned %d agents, want 4", len(agents))
	}
}

// TestLoadBuiltinAgents_AgentsByName verifies each expected agent name is present.
func TestLoadBuiltinAgents_AgentsByName(t *testing.T) {
	agents, err := materialize.LoadBuiltinAgents()
	if err != nil {
		t.Fatalf("LoadBuiltinAgents() failed: %v", err)
	}

	byName := make(map[string]bool, len(agents))
	for _, a := range agents {
		byName[a.Name] = true
	}

	for _, want := range []string{"claude", "factory", "opencode", "copilot"} {
		if !byName[want] {
			t.Errorf("built-in agent %q not found after load", want)
		}
	}
}

// TestLoadBuiltinAgents_MCPConfig_Claude verifies the claude agent exposes the
// expected MCP schema values after YAML parsing.
func TestLoadBuiltinAgents_MCPConfig_Claude(t *testing.T) {
	agents, err := materialize.LoadBuiltinAgents()
	if err != nil {
		t.Fatalf("LoadBuiltinAgents() failed: %v", err)
	}

	var found bool
	for _, a := range agents {
		if a.Name != "claude" {
			continue
		}
		found = true

		if a.MCP == nil {
			t.Fatal("claude agent: MCP field should not be nil")
		}
		if a.MCP.FilePath != "../.mcp.json" {
			t.Errorf("claude MCP.FilePath = %q, want %q", a.MCP.FilePath, "../.mcp.json")
		}
		if a.MCP.RootKey != "mcpServers" {
			t.Errorf("claude MCP.RootKey = %q, want %q", a.MCP.RootKey, "mcpServers")
		}
		if a.MCP.EnvKey != "env" {
			t.Errorf("claude MCP.EnvKey = %q, want %q", a.MCP.EnvKey, "env")
		}
		if a.MCP.EnvVarStyle != "${VAR}" {
			t.Errorf("claude MCP.EnvVarStyle = %q, want %q", a.MCP.EnvVarStyle, "${VAR}")
		}
	}

	if !found {
		t.Error("claude agent not found in built-in agents")
	}
}

// TestLoadBuiltinAgents_MCPConfig_Factory verifies the factory agent exposes the
// expected MCP schema values after YAML parsing.
func TestLoadBuiltinAgents_MCPConfig_Factory(t *testing.T) {
	agents, err := materialize.LoadBuiltinAgents()
	if err != nil {
		t.Fatalf("LoadBuiltinAgents() failed: %v", err)
	}

	var found bool
	for _, a := range agents {
		if a.Name != "factory" {
			continue
		}
		found = true

		if a.MCP == nil {
			t.Fatal("factory agent: MCP field should not be nil")
		}
		if a.MCP.FilePath != "mcp.json" {
			t.Errorf("factory MCP.FilePath = %q, want %q", a.MCP.FilePath, "mcp.json")
		}
		if a.MCP.RootKey != "mcpServers" {
			t.Errorf("factory MCP.RootKey = %q, want %q", a.MCP.RootKey, "mcpServers")
		}
		if a.MCP.EnvKey != "env" {
			t.Errorf("factory MCP.EnvKey = %q, want %q", a.MCP.EnvKey, "env")
		}
		if a.MCP.EnvVarStyle != "${VAR}" {
			t.Errorf("factory MCP.EnvVarStyle = %q, want %q", a.MCP.EnvVarStyle, "${VAR}")
		}
	}

	if !found {
		t.Error("factory agent not found in built-in agents")
	}
}

// TestLoadBuiltinAgents_MCPConfig_OpenCode verifies the opencode agent exposes the
// expected MCP schema values after YAML parsing.
func TestLoadBuiltinAgents_MCPConfig_OpenCode(t *testing.T) {
	agents, err := materialize.LoadBuiltinAgents()
	if err != nil {
		t.Fatalf("LoadBuiltinAgents() failed: %v", err)
	}

	var found bool
	for _, a := range agents {
		if a.Name != "opencode" {
			continue
		}
		found = true

		if a.MCP == nil {
			t.Fatal("opencode agent: MCP field should not be nil")
		}
		if a.MCP.FilePath != "opencode.json" {
			t.Errorf("opencode MCP.FilePath = %q, want %q", a.MCP.FilePath, "opencode.json")
		}
		if a.MCP.RootKey != "mcp" {
			t.Errorf("opencode MCP.RootKey = %q, want %q", a.MCP.RootKey, "mcp")
		}
		if a.MCP.EnvKey != "environment" {
			t.Errorf("opencode MCP.EnvKey = %q, want %q", a.MCP.EnvKey, "environment")
		}
		if a.MCP.EnvVarStyle != "{env:VAR}" {
			t.Errorf("opencode MCP.EnvVarStyle = %q, want %q", a.MCP.EnvVarStyle, "{env:VAR}")
		}
	}

	if !found {
		t.Error("opencode agent not found in built-in agents")
	}
}

// TestLoadBuiltinAgents_MCPConfig_Copilot verifies the copilot agent exposes the
// expected MCP schema values after YAML parsing.
func TestLoadBuiltinAgents_MCPConfig_Copilot(t *testing.T) {
	agents, err := materialize.LoadBuiltinAgents()
	if err != nil {
		t.Fatalf("LoadBuiltinAgents() failed: %v", err)
	}

	var found bool
	for _, a := range agents {
		if a.Name != "copilot" {
			continue
		}
		found = true

		if a.MCP == nil {
			t.Fatal("copilot agent: MCP field should not be nil")
		}
		if a.MCP.FilePath != "../.vscode/mcp.json" {
			t.Errorf("copilot MCP.FilePath = %q, want %q", a.MCP.FilePath, "../.vscode/mcp.json")
		}
		if a.MCP.RootKey != "servers" {
			t.Errorf("copilot MCP.RootKey = %q, want %q", a.MCP.RootKey, "servers")
		}
		if a.MCP.EnvKey != "env" {
			t.Errorf("copilot MCP.EnvKey = %q, want %q", a.MCP.EnvKey, "env")
		}
		if a.MCP.EnvVarStyle != "${env:VAR}" {
			t.Errorf("copilot MCP.EnvVarStyle = %q, want %q", a.MCP.EnvVarStyle, "${env:VAR}")
		}
	}

	if !found {
		t.Error("copilot agent not found in built-in agents")
	}
}

// TestLoadBuiltinAgents_ValidationPasses verifies that every loaded built-in agent
// passes its own Validate() method — i.e., all required fields are present.
func TestLoadBuiltinAgents_ValidationPasses(t *testing.T) {
	agents, err := materialize.LoadBuiltinAgents()
	if err != nil {
		t.Fatalf("LoadBuiltinAgents() failed: %v", err)
	}

	for _, a := range agents {
		if err := a.Validate(); err != nil {
			t.Errorf("agent %q: Validate() failed: %v", a.Name, err)
		}
	}
}

// TestLoadDefaultRegistry_AllAgentsRegistered verifies that all four built-in
// agent renderers are registered and accessible via the default registry.
func TestLoadDefaultRegistry_AllAgentsRegistered(t *testing.T) {
	registry, err := materialize.LoadDefaultRegistry()
	if err != nil {
		t.Fatalf("LoadDefaultRegistry() failed: %v", err)
	}

	for _, name := range []string{"claude", "factory", "opencode", "copilot"} {
		if _, ok := registry[name]; !ok {
			t.Errorf("registry missing agent %q", name)
		}
	}
}
