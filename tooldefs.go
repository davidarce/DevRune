// SPDX-License-Identifier: MIT

// Package devrune (root package) embeds the built-in tool YAML files.
// This file exists at the module root so that //go:embed can access tools/*.yaml
// without parent-directory traversal (which Go's embed directive does not permit).
package devrune

import "embed"

// BuiltinToolsFS exposes the embedded tool definition YAML files.
// Import as: import devrune "github.com/davidarce/devrune"
//
//go:embed tools/*.yaml
var BuiltinToolsFS embed.FS
