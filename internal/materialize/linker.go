// Package materialize implements Stage 3 of the DevRune pipeline: converting a
// Lockfile into workspace mutations. AgentRenderer implementations handle all
// agent-specific frontmatter conversion, MCP config generation, and catalog
// generation. No YAML transform config is needed — all agent knowledge is
// compiled into Go renderer structs.
package materialize

import "fmt"

// Linker creates links or copies from a cache path to a workspace destination path.
// Implementations: SymlinkLinker, CopyLinker, HardlinkLinker.
type Linker interface {
	// Link creates a link (or copy) from src to dst.
	// Parent directories of dst are created if they do not exist.
	Link(src, dst string) error

	// Mode returns the link mode string: "symlink", "copy", or "hardlink".
	Mode() string
}

// NewLinker returns the appropriate Linker for the given mode string.
// Supported modes: "symlink" (default), "copy", "hardlink".
// An empty mode string defaults to "symlink".
func NewLinker(mode string) (Linker, error) {
	switch mode {
	case "", "symlink":
		return &SymlinkLinker{}, nil
	case "copy":
		return &CopyLinker{}, nil
	case "hardlink":
		return &HardlinkLinker{}, nil
	default:
		return nil, fmt.Errorf("linker: unknown mode %q (supported: symlink, copy, hardlink)", mode)
	}
}
