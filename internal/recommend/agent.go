// SPDX-License-Identifier: MIT

package recommend

import (
	"errors"
	"os/exec"
)

// ErrNoAgent is returned when no supported AI agent binary is found on PATH.
var ErrNoAgent = errors.New("no AI agent found (claude or opencode)")

// DetectAgent finds the first available AI agent binary.
// Priority: "claude" > "opencode". Returns binary path, agent name, and error.
func DetectAgent() (binaryPath string, agentName string, err error) {
	if path, err := exec.LookPath("claude"); err == nil {
		return path, "claude", nil
	}
	if path, err := exec.LookPath("opencode"); err == nil {
		return path, "opencode", nil
	}
	return "", "", ErrNoAgent
}
