// SPDX-License-Identifier: MIT

package model

// ToolDef holds the declarative definition for a tool that can be installed
// by the DevRune wizard. Each tool definition lives in a YAML file inside the
// starter catalog's tools/ directory.
type ToolDef struct {
	// Name is the short identifier for the tool (e.g. "engram", "crit").
	Name string `yaml:"name"`

	// Description is a human-readable summary shown in the wizard UI.
	Description string `yaml:"description"`

	// Command is the shell command used to install the tool
	// (e.g. "brew install gentleman-programming/tap/engram").
	Command string `yaml:"command"`

	// Binary is the name of the executable that should be present after
	// installation, used to check whether the tool is already installed.
	Binary string `yaml:"binary"`

	// DependsOn lists optional DevRune-managed resources that must be present
	// for this tool to be fully functional. Omit when there are no dependencies.
	DependsOn *ToolDeps `yaml:"depends_on,omitempty"`
}

// ToolDeps describes the DevRune-managed resources that a tool depends on.
// Both fields are optional; omit whichever does not apply.
type ToolDeps struct {
	// MCP is the name of an MCP server entry (from the catalog) that this
	// tool requires.  Example: "engram".
	MCP string `yaml:"mcp,omitempty"`

	// Workflow is the name of a DevRune workflow (from the catalog) that this
	// tool is associated with.  Example: "sdd".
	Workflow string `yaml:"workflow,omitempty"`
}
