package model

import "fmt"

// reservedAgentTypes lists the built-in agent renderer types recognized by DevRune.
var reservedAgentTypes = map[string]bool{
	"claude":   true,
	"opencode": true,
	"copilot":  true,
	"factory":  true,
}

// SettingsConfig holds agent-level configuration for generated settings files.
// For Claude, this controls the content of `.claude/settings.json`.
type SettingsConfig struct {
	// Permissions lists base permission patterns to include in the agent's settings file.
	// Workflow-level permissions are merged with these at render time.
	Permissions []string `yaml:"permissions,omitempty"`
}

// MCPConfig holds declarative MCP renderer conventions for an agent.
// These values tell renderers where to write MCP config, which JSON keys to use,
// and how to format environment variable placeholders. All fields are optional;
// renderers fall back to Claude conventions when MCPConfig is nil.
type MCPConfig struct {
	FilePath    string `yaml:"filePath"`    // MCP config file path relative to workspace dir
	RootKey     string `yaml:"rootKey"`     // top-level JSON key wrapping MCP entries
	EnvKey      string `yaml:"envKey"`      // key name for env vars in each MCP entry
	EnvVarStyle string `yaml:"envVarStyle"` // env var placeholder pattern using VAR as token
}

// AgentDefinition is the minimal agent configuration loaded from agents/*.yaml.
// All rendering logic lives in the Go AgentRenderer implementations — not in YAML config.
// Adding a new agent means implementing AgentRenderer in Go, not writing complex YAML.
type AgentDefinition struct {
	Name         string          `yaml:"name"`               // e.g. "claude", "opencode"
	Type         string          `yaml:"type"`               // selects the built-in renderer: "claude", "opencode", "copilot", "factory"
	Workspace    string          `yaml:"workspace"`          // e.g. ".claude"
	SkillDir     string          `yaml:"skillDir"`           // reusable backing skill tree relative to workspace, e.g. "skills"
	AgentDir     string          `yaml:"agentDir,omitempty"` // surfaced native agent entry files relative to workspace, e.g. "agents" (optional; only for platforms that distinguish skill storage from agent surfaces, such as Copilot)
	CommandDir   string          `yaml:"commandDir"`         // where commands go, e.g. "commands" (optional)
	RulesDir     string          `yaml:"rulesDir"`           // e.g. "rules"
	CatalogFile  string          `yaml:"catalogFile"`        // e.g. "CLAUDE.md"
	DefaultRules string          `yaml:"defaultRulesMode"`   // "concat" | "individual" | "both"
	Settings     *SettingsConfig `yaml:"settings,omitempty"` // optional settings for generated settings files
	MCP          *MCPConfig      `yaml:"mcp,omitempty"`      // optional declarative MCP renderer conventions
}

// Validate checks that the AgentDefinition has all required fields.
func (a AgentDefinition) Validate() error {
	if a.Name == "" {
		return fmt.Errorf("agent: name is required")
	}
	if a.Type == "" {
		return fmt.Errorf("agent %q: type is required", a.Name)
	}
	if !reservedAgentTypes[a.Type] {
		return fmt.Errorf("agent %q: unknown type %q (supported: claude, opencode, copilot, factory)", a.Name, a.Type)
	}
	if a.Workspace == "" {
		return fmt.Errorf("agent %q: workspace is required", a.Name)
	}
	if a.SkillDir == "" {
		return fmt.Errorf("agent %q: skillDir is required", a.Name)
	}
	if a.CatalogFile == "" {
		return fmt.Errorf("agent %q: catalogFile is required", a.Name)
	}
	return nil
}
