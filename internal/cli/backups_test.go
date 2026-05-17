// SPDX-License-Identifier: MIT

// Package cli_test contains integration tests for the cli package.
//
// T015 restore → install reentrancy tests are in internal/tui/steps/backups_test.go
// because executeRestore lives in that package (unexported). The tests here cover
// the thin runBackupsFromMenu dispatcher layer.
package cli_test
