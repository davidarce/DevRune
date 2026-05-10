// SPDX-License-Identifier: MIT

package steps

import (
	"errors"
	"sync"
	"testing"

	"github.com/davidarce/devrune/internal/model"
)

// ---------------------------------------------------------------------------
// T015 — Parallel upgrade engine tests
// ---------------------------------------------------------------------------

// mockExecutor returns a ToolCommandExecutor that records every command it
// receives (protected by mu) and returns the provided error for matching cmds.
func mockExecutor(mu *sync.Mutex, recorded *[]string, failCmd string, failErr error) ToolCommandExecutor {
	return func(command string) error {
		mu.Lock()
		*recorded = append(*recorded, command)
		mu.Unlock()
		if failCmd != "" && command == failCmd {
			return failErr
		}
		return nil
	}
}

// TestUpgradeToolsParallel_AllOK verifica que 3 tools upgradables con executor
// sin errores producen Results con status=ok en el orden del input.
func TestUpgradeToolsParallel_AllOK(t *testing.T) {
	items := []upgradeToolItem{
		{Name: "engram", Command: "brew install engram", Upgradable: true},
		{Name: "crit", Command: "brew install crit", Upgradable: true},
		{Name: "other", Command: "brew install other", Upgradable: true},
	}

	var mu sync.Mutex
	var recorded []string
	exec := mockExecutor(&mu, &recorded, "", nil)

	summary := upgradeToolsParallel(items, exec)

	if len(summary.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(summary.Results))
	}

	for i, res := range summary.Results {
		if res.Name != items[i].Name {
			t.Errorf("result[%d]: expected name %q, got %q", i, items[i].Name, res.Name)
		}
		if res.Status != ToolUpgradeOK {
			t.Errorf("result[%d] %q: expected status ok, got %q", i, res.Name, res.Status)
		}
		if res.Error != "" {
			t.Errorf("result[%d] %q: expected no error, got %q", i, res.Name, res.Error)
		}
	}
}

// TestUpgradeToolsParallel_Mixed verifica que cuando la tool del medio falla,
// solo esa tiene status=fail y las demás status=ok; el orden de input se mantiene.
func TestUpgradeToolsParallel_Mixed(t *testing.T) {
	failCmd := "brew install crit"
	failErr := errors.New("exit status 1")

	items := []upgradeToolItem{
		{Name: "engram", Command: "brew install engram", Upgradable: true},
		{Name: "crit", Command: failCmd, Upgradable: true},
		{Name: "other", Command: "brew install other", Upgradable: true},
	}

	var mu sync.Mutex
	var recorded []string
	exec := mockExecutor(&mu, &recorded, failCmd, failErr)

	summary := upgradeToolsParallel(items, exec)

	if len(summary.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(summary.Results))
	}

	// engram → ok
	if summary.Results[0].Status != ToolUpgradeOK {
		t.Errorf("engram: expected ok, got %q", summary.Results[0].Status)
	}

	// crit → fail
	if summary.Results[1].Status != ToolUpgradeFail {
		t.Errorf("crit: expected fail, got %q", summary.Results[1].Status)
	}
	if summary.Results[1].Error != failErr.Error() {
		t.Errorf("crit: expected error %q, got %q", failErr.Error(), summary.Results[1].Error)
	}

	// other → ok
	if summary.Results[2].Status != ToolUpgradeOK {
		t.Errorf("other: expected ok, got %q", summary.Results[2].Status)
	}

	// orden estable: names deben coincidir con input
	names := []string{"engram", "crit", "other"}
	for i, r := range summary.Results {
		if r.Name != names[i] {
			t.Errorf("result[%d]: expected name %q, got %q", i, names[i], r.Name)
		}
	}
}

// TestUpgradeToolsParallel_NonUpgradableSkipped verifica que una tool con
// Upgradable=false no se incluye en Results y el executor nunca la recibe.
func TestUpgradeToolsParallel_NonUpgradableSkipped(t *testing.T) {
	items := []upgradeToolItem{
		{Name: "engram", Command: "brew install engram", Upgradable: true},
		{Name: "custom-local", Command: "", Upgradable: false},
		{Name: "crit", Command: "brew install crit", Upgradable: true},
	}

	var mu sync.Mutex
	var recorded []string
	exec := mockExecutor(&mu, &recorded, "", nil)

	summary := upgradeToolsParallel(items, exec)

	// Solo 2 upgradable → 2 results
	if len(summary.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(summary.Results))
	}

	// El executor no debe haber recibido el command de custom-local (vacío)
	mu.Lock()
	defer mu.Unlock()
	for _, cmd := range recorded {
		if cmd == "" {
			t.Error("executor was called with empty command (non-upgradable tool should be skipped)")
		}
	}

	// Los nombres de los resultados deben ser solo los upgradables
	for _, r := range summary.Results {
		if r.Name == "custom-local" {
			t.Error("non-upgradable tool 'custom-local' should not appear in results")
		}
	}
}

// TestUpgradeToolsParallel_ConcurrencyCapture usa sync.Mutex para capturar
// commands y verifica que solo los Upgradable=true llegaron al executor.
func TestUpgradeToolsParallel_ConcurrencyCapture(t *testing.T) {
	items := []upgradeToolItem{
		{Name: "a", Command: "cmd-a", Upgradable: true},
		{Name: "b", Command: "cmd-b", Upgradable: false},
		{Name: "c", Command: "cmd-c", Upgradable: true},
		{Name: "d", Command: "cmd-d", Upgradable: false},
		{Name: "e", Command: "cmd-e", Upgradable: true},
	}

	var mu sync.Mutex
	var recorded []string

	exec := func(command string) error {
		mu.Lock()
		recorded = append(recorded, command)
		mu.Unlock()
		return nil
	}

	upgradeToolsParallel(items, exec)

	mu.Lock()
	got := make([]string, len(recorded))
	copy(got, recorded)
	mu.Unlock()

	// Exactamente 3 commands upgradables
	if len(got) != 3 {
		t.Fatalf("expected 3 commands executed, got %d: %v", len(got), got)
	}

	// Todos deben ser de tools upgradables
	allowed := map[string]bool{"cmd-a": true, "cmd-c": true, "cmd-e": true}
	for _, cmd := range got {
		if !allowed[cmd] {
			t.Errorf("unexpected command executed: %q (non-upgradable tool reached executor)", cmd)
		}
	}
}

// ---------------------------------------------------------------------------
// T016 — Effective command classification tests (buildUpgradeToolItems)
// ---------------------------------------------------------------------------

func catalogWith(name, command string) map[string]model.ToolDef {
	return map[string]model.ToolDef{
		name: {Name: name, Command: command},
	}
}

// TestBuildUpgradeToolItems_YAMLOverrideWins verifica que el command del
// ToolRef (YAML) gana sobre el catálogo cuando ambos están presentes.
func TestBuildUpgradeToolItems_YAMLOverrideWins(t *testing.T) {
	tools := []model.ToolRef{
		{Name: "engram", Command: "custom-override-command"},
	}
	catalog := catalogWith("engram", "catalog-command")

	items := buildUpgradeToolItems(tools, catalog)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Command != "custom-override-command" {
		t.Errorf("expected YAML override to win, got %q", items[0].Command)
	}
	if !items[0].Upgradable {
		t.Error("expected Upgradable=true for non-empty command")
	}
}

// TestBuildUpgradeToolItems_FallbackToCatalog verifica que cuando el ToolRef
// tiene Command vacío, se usa el command del catálogo.
func TestBuildUpgradeToolItems_FallbackToCatalog(t *testing.T) {
	tools := []model.ToolRef{
		{Name: "crit", Command: ""},
	}
	catalog := catalogWith("crit", "brew install crit")

	items := buildUpgradeToolItems(tools, catalog)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Command != "brew install crit" {
		t.Errorf("expected catalog fallback command, got %q", items[0].Command)
	}
	if !items[0].Upgradable {
		t.Error("expected Upgradable=true when catalog provides command")
	}
}

// TestBuildUpgradeToolItems_BothEmptyNotUpgradable verifica que cuando tanto
// ToolRef.Command como catalog están vacíos/whitespace, Upgradable=false.
func TestBuildUpgradeToolItems_BothEmptyNotUpgradable(t *testing.T) {
	tools := []model.ToolRef{
		{Name: "custom-local", Command: "   "}, // whitespace
	}
	catalog := catalogWith("custom-local", "  ") // whitespace en catálogo

	items := buildUpgradeToolItems(tools, catalog)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Upgradable {
		t.Error("expected Upgradable=false when both YAML and catalog are empty/whitespace")
	}
}

// TestBuildUpgradeToolItems_NoCatalogEntry verifica que cuando no hay entrada
// en el catálogo pero el ToolRef tiene command, se usa ese command.
func TestBuildUpgradeToolItems_NoCatalogEntryWithYAML(t *testing.T) {
	tools := []model.ToolRef{
		{Name: "my-tool", Command: "brew install my-tool"},
	}
	catalog := map[string]model.ToolDef{} // catálogo vacío

	items := buildUpgradeToolItems(tools, catalog)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Command != "brew install my-tool" {
		t.Errorf("expected ToolRef command when no catalog entry, got %q", items[0].Command)
	}
	if !items[0].Upgradable {
		t.Error("expected Upgradable=true when ToolRef.Command is present")
	}
}

// TestBuildUpgradeToolItems_NoCatalogEntryNoYAML verifica que cuando no hay
// entrada en el catálogo ni command en ToolRef, Upgradable=false.
func TestBuildUpgradeToolItems_NoCatalogEntryNoYAML(t *testing.T) {
	tools := []model.ToolRef{
		{Name: "unknown-tool", Command: ""},
	}
	catalog := map[string]model.ToolDef{} // catálogo vacío

	items := buildUpgradeToolItems(tools, catalog)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Upgradable {
		t.Error("expected Upgradable=false when no catalog entry and no YAML command")
	}
	if items[0].Command != "" {
		t.Errorf("expected empty command, got %q", items[0].Command)
	}
}

// TestBuildUpgradeToolItems_EmptyToolsList verifica que una lista vacía de
// tools produce un slice vacío.
func TestBuildUpgradeToolItems_EmptyToolsList(t *testing.T) {
	items := buildUpgradeToolItems(nil, map[string]model.ToolDef{})

	if len(items) != 0 {
		t.Errorf("expected empty slice for nil tools, got %d items", len(items))
	}
}
