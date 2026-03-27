// SPDX-License-Identifier: MIT

package main

import (
	"os"

	"github.com/davidarce/devrune/internal/cli"
)

// version and commit are injected at build time via ldflags:
//
//	-X main.version=v1.2.3
//	-X main.commit=abc1234
var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	rootCmd := cli.NewRootCmd(version, commit)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
