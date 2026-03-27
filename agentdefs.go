// SPDX-License-Identifier: MIT

// Package devrune (root package) embeds the built-in agent YAML files.
// This file exists at the module root so that //go:embed can access agents/*.yaml
// without parent-directory traversal (which Go's embed directive does not permit).
package devrune

import "embed"

// BuiltinAgentsFS exposes the embedded agent definition YAML files.
// Import as: import devrune "github.com/davidarce/devrune"
//
//go:embed agents/*.yaml
var BuiltinAgentsFS embed.FS
