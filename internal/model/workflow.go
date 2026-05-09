// SPDX-License-Identifier: MIT

package model

import "fmt"

const WorkflowAPIVersion = "devrune/workflow/v1"

// DecisionRule maps a natural-language scenario to a skill resolution.
// Used to build the Decision Rules table in the agent catalog.
type DecisionRule struct {
	Scenario   string `yaml:"scenario"`   // e.g. `"commit", "commit my changes", "create commit"`
	Resolution string `yaml:"resolution"` // e.g. "Use `git:commit`"
}

// InvocationControl describes how a skill or set of skills should be invoked.
// Used to build the Invocation Controls section in the agent catalog.
type InvocationControl struct {
	Skills      string `yaml:"skills"`      // comma-separated skill names, e.g. "git:commit, git:pull-request"
	Description string `yaml:"description"` // when to auto-invoke
}

// WorkflowManifest represents a workflow.yaml file.
// A workflow is a PURE MANIFEST — it lists its components and nothing else.
// The library handles all per-agent registration, catalog generation, gitignore,
// and workspace integration. Workflow authors never think about agent-specific formats.
//
// v3.4: The workflow directory is the atomic unit of installation. workflow.yaml only
// marks which subdirectories are skills (needing frontmatter transform) and which file
// is the entrypoint. Everything else in the workflow directory materializes as-is.
type WorkflowManifest struct {
	APIVersion string             `yaml:"apiVersion"` // "devrune/workflow/v1"
	Metadata   WorkflowMetadata   `yaml:"metadata"`
	Components WorkflowComponents `yaml:"components"`

	// RegistryContent holds the pre-processed content of the Registry file.
	// This is a transient runtime field — it is never serialized to or from YAML.
	// It is populated during InstallWorkflow and consumed by RenderCatalog.
	RegistryContent string `yaml:"-"`
}

// WorkflowMetadata holds identifying information for a workflow.
type WorkflowMetadata struct {
	Name        string `yaml:"name"`                  // slug identifier, e.g. "sdd"
	DisplayName string `yaml:"displayName,omitempty"` // human-readable label for catalogs, e.g. "SDD (Spec-Driven Development)"
	Version     string `yaml:"version"`               // semver, e.g. "1.0.0"
	WorkingDir  string `yaml:"workingDir,omitempty"`   // directory name for workflow files (orchestrator, _shared/); defaults to Name
}

// EffectiveDisplayName returns DisplayName if set, otherwise falls back to Name.
func (m WorkflowMetadata) EffectiveDisplayName() string {
	if m.DisplayName != "" {
		return m.DisplayName
	}
	return m.Name
}

// EffectiveWorkingDir returns WorkingDir if set, otherwise falls back to Name.
func (m WorkflowMetadata) EffectiveWorkingDir() string {
	if m.WorkingDir != "" {
		return m.WorkingDir
	}
	return m.Name
}

// WorkflowRole describes the projection metadata for a single agent role within a workflow.
// Renderers use this to synthesize platform-native agent entries (e.g. opencode.json agents,
// Copilot agent markdown) rather than hardcoding SDD-specific knowledge.
type WorkflowRole struct {
	// Name is the agent role identifier, e.g. "sdd-explorer", "sdd-orchestrator".
	Name string `yaml:"name"`

	// Kind is the role category: "subagent" or "orchestrator".
	Kind string `yaml:"kind"`

	// Skill is the skill directory name associated with this role (subagents only).
	// For orchestrators this field is omitted; the entrypoint is used instead.
	Skill string `yaml:"skill,omitempty"`

	// Models is a per-agent map of suggested model values for this role,
	// keyed by agent name (claude, opencode, copilot). Required for subagent
	// roles. Values are the alias or full model id understood by that agent's
	// renderer:
	//   - claude:   short tier names (haiku, sonnet, opus)
	//   - opencode: short tier names (resolved to github-copilot/<full-id>)
	//               or fully-qualified provider/model strings
	//   - copilot:  VS Code display names verbatim (e.g. "Claude Sonnet 4.6")
	// DevRune uses these as defaults in the TUI model selection step. Per-role
	// TUI overrides win at render time; absent that, the value here is used.
	// Orchestrator roles do not declare models — orchestrator selection is
	// agent-specific and handled separately.
	Models map[string]string `yaml:"models,omitempty"`

	// Placeholder is an optional explicit placeholder key suffix override.
	// When set, the placeholder {WORKFLOW_MODEL_<Placeholder>} is used instead of
	// auto-deriving from the role name. Example: placeholder: "CHECKER" →
	// {WORKFLOW_MODEL_CHECKER}. When omitted, the key is derived by stripping the
	// workflow name prefix from the role name.
	Placeholder string `yaml:"placeholder,omitempty"`
}

// ModelFor returns the model declared for the given agent in this role, or "" if absent.
func (r WorkflowRole) ModelFor(agent string) string {
	if r.Models == nil {
		return ""
	}
	return r.Models[agent]
}

// WorkflowComponents declares the components that make up the workflow.
// The workflow directory is the atomic unit — everything in it gets materialized.
// This struct only declares which parts need special handling.
type WorkflowComponents struct {
	// Skills lists skill names this workflow needs. Each renderer resolves a
	// name in two steps:
	//   1. Workflow-internal: subdirectory of the workflow directory
	//      (e.g. "<workflow-dir>/<name>/SKILL.md"). Used for skills shipped
	//      alongside the workflow (e.g. SDD's sdd-explore, sdd-plan).
	//   2. Catalog-level fallback: top-level skill directory of the catalog
	//      (e.g. "<catalog-root>/skills/<name>/SKILL.md"). Used for shared,
	//      reusable skills the workflow depends on but doesn't own
	//      (e.g. SDD's PRD gate invokes write-a-prd, which lives at
	//      catalog/skills/write-a-prd/, not under the SDD workflow).
	// Internal-pass skills get rendered first; anything still listed but not
	// yet rendered triggers the catalog-level resolver. Missing skills produce
	// a clear error naming both attempted paths.
	// All non-skill files and directories under the workflow dir are
	// materialized as-is.
	Skills []string `yaml:"skills"`

	// Entrypoint is the file path (relative to workflow directory) of the
	// entrypoint document (e.g. "ORCHESTRATOR.md"). Plain string — not a struct.
	// When set, the renderer installs it in agent-native format.
	// No EntrypointRef struct — v3.4 simplification.
	Entrypoint string `yaml:"entrypoint,omitempty"`

	// Roles lists the agent role projection metadata for this workflow.
	// Renderers use these to synthesize platform-native agent entries (e.g. opencode.json agents).
	// Backward compatible: workflows without roles continue to parse and install normally.
	// When roles are present: kind must be "subagent" or "orchestrator"; subagents should have skill.
	Roles []WorkflowRole `yaml:"roles,omitempty"`

	// Rules lists rule directory names within the workflow directory.
	Rules []string `yaml:"rules,omitempty"`

	// Commands lists workflow-level slash commands for the agent's catalog.
	Commands []WorkflowCommand `yaml:"commands,omitempty"`

	// MCPs lists MCP definition file names within the workflow directory.
	MCPs []string `yaml:"mcps,omitempty"`

	// DecisionRules lists scenario-to-skill mappings for the agent catalog's Decision Rules table.
	DecisionRules []DecisionRule `yaml:"decisionRules,omitempty"`

	// InvocationControls lists skill invocation mode descriptors for the Invocation Controls section.
	InvocationControls []InvocationControl `yaml:"invocationControls,omitempty"`

	// Registry is the path (relative to the workflow directory) of a markdown file whose
	// contents are injected verbatim into the agent catalog's Workflows section.
	Registry string `yaml:"registry,omitempty"`

	// Permissions lists permission patterns to include in the agent's settings file
	// (e.g. ".claude/settings.json"). Additive with base agent permissions.
	Permissions []string `yaml:"permissions,omitempty"`

	// Gitignore lists patterns to add to .gitignore for this workflow's working
	// directories and artifacts (e.g. ".sdd/" for the SDD workflow). Optional —
	// workflows that don't produce on-disk artifacts can omit this.
	Gitignore []string `yaml:"gitignore,omitempty"`

	// Hooks declares opaque hook definitions per agent. DevRune validates JSON
	// syntax of each definition file but does NOT interpret the content. Renderers
	// deep-merge the JSON into the agent's settings file (e.g. .claude/settings.json).
	// Optional — workflows without hooks can omit this.
	Hooks *WorkflowHooksConfig `yaml:"hooks,omitempty"`
}

// WorkflowCommand represents a slash command exposed by the workflow in the agent's catalog.
type WorkflowCommand struct {
	Name     string `yaml:"name"`               // e.g. "sdd-explore"
	Action   string `yaml:"action"`             // e.g. "Explore and investigate"
	Argument string `yaml:"argument,omitempty"` // e.g. "<topic>"
}

// WorkflowHookDef is an opaque hook definition for a specific agent.
// DevRune validates JSON syntax but does NOT interpret the content.
// The JSON file contains the exact native format for that agent.
type WorkflowHookDef struct {
	Definition string `yaml:"definition"` // path to JSON file relative to workflow dir
}

// WorkflowHooksConfig maps agent names to their opaque hook definitions.
type WorkflowHooksConfig struct {
	Agents map[string][]WorkflowHookDef `yaml:"agents"` // key = agent name (e.g. "claude", "opencode")
}

// Validate checks that the WorkflowManifest is well-formed.
//
// Rules:
//   - apiVersion must be "devrune/workflow/v1"
//   - metadata.name is required
//   - At least one skill or command must be declared
//   - When roles are present: kind must be "subagent" or "orchestrator"
//   - Subagent roles should have a skill value; orchestrator roles must not
func (w WorkflowManifest) Validate() error {
	if w.APIVersion != WorkflowAPIVersion {
		return fmt.Errorf("workflow: apiVersion must be %q (got %q)", WorkflowAPIVersion, w.APIVersion)
	}
	if w.Metadata.Name == "" {
		return fmt.Errorf("workflow: metadata.name is required")
	}
	if len(w.Components.Skills) == 0 && len(w.Components.Commands) == 0 {
		return fmt.Errorf("workflow %q: at least one skill or command must be declared", w.Metadata.Name)
	}
	for i, role := range w.Components.Roles {
		if role.Name == "" {
			return fmt.Errorf("workflow %q: role[%d] name is required", w.Metadata.Name, i)
		}
		if role.Kind != "subagent" && role.Kind != "orchestrator" {
			return fmt.Errorf("workflow %q: role %q kind must be \"subagent\" or \"orchestrator\" (got %q)", w.Metadata.Name, role.Name, role.Kind)
		}
		if role.Kind == "subagent" {
			if role.Skill == "" {
				return fmt.Errorf("workflow %q: subagent role %q must declare a skill", w.Metadata.Name, role.Name)
			}
			if len(role.Models) == 0 {
				return fmt.Errorf("workflow %q: subagent role %q must declare a non-empty models map (one entry per agent: claude, opencode, copilot)", w.Metadata.Name, role.Name)
			}
			for agent, value := range role.Models {
				if !ModelRoutingAgents[agent] {
					return fmt.Errorf("workflow %q: role %q models key %q is not a recognised agent (expected one of: claude, opencode, copilot)", w.Metadata.Name, role.Name, agent)
				}
				if value == "" {
					return fmt.Errorf("workflow %q: role %q models[%q] must not be empty", w.Metadata.Name, role.Name, agent)
				}
			}
		}
		if role.Kind == "orchestrator" && len(role.Models) > 0 {
			return fmt.Errorf("workflow %q: orchestrator role %q must not declare models — orchestrator model selection is agent-specific and handled separately", w.Metadata.Name, role.Name)
		}
	}
	if w.Components.Hooks != nil {
		for agent, defs := range w.Components.Hooks.Agents {
			for j, def := range defs {
				if def.Definition == "" {
					return fmt.Errorf("workflow %q: hooks.agents[%q][%d].definition must not be empty", w.Metadata.Name, agent, j)
				}
			}
		}
	}
	return nil
}
