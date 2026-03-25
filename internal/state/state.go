// Package state manages the .devrune/state.yaml workspace state file.
// It records managed paths, the active lock hash, installed agents, and
// active workflows so that reinstalls can cleanly remove previous artifacts.
package state
