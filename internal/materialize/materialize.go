// Package materialize implements Stage 3 of the DevRune pipeline: converting a
// Lockfile into workspace mutations. AgentRenderer implementations handle all
// agent-specific frontmatter conversion, MCP config generation, and catalog
// generation. No YAML transform config is needed — all agent knowledge is
// compiled into Go renderer structs.
package materialize
