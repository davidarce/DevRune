// SPDX-License-Identifier: MIT

package steps

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// ---------------------------------------------------------------------------
// Tests for installModel — error state rendering (T019)
// ---------------------------------------------------------------------------

// TestInstallModel_ErrorStateRendersMessage verifies that when installDoneMsg
// carries a non-nil error, the model's View() contains the error text.
func TestInstallModel_ErrorStateRendersMessage(t *testing.T) {
	m := newInstallModel(nil, nil)
	sentErr := errors.New("resolve failed: network timeout")

	// Simulate receiving installDoneMsg with an error.
	updatedModel, _ := m.Update(installDoneMsg{err: sentErr})

	im, ok := updatedModel.(installModel)
	if !ok {
		t.Fatalf("Update returned unexpected model type %T", updatedModel)
	}

	if im.err == nil {
		t.Fatal("expected model.err to be set after installDoneMsg with error, got nil")
	}

	view := im.View()
	rendered := view.Content

	if !strings.Contains(rendered, "resolve failed: network timeout") {
		t.Errorf("View() does not contain error message %q\nrendered:\n%s",
			sentErr.Error(), rendered)
	}

	if !strings.Contains(rendered, "Press any key to exit") {
		t.Errorf("View() does not contain keypress hint\nrendered:\n%s", rendered)
	}
}


// TestInstallModel_ErrorStateNotQuitUntilKeypress verifies that the model
// does NOT quit immediately on receiving an error — it stays alive to show
// the error, and only quits on a subsequent keypress.
func TestInstallModel_ErrorStateNotQuitUntilKeypress(t *testing.T) {
	m := newInstallModel(nil, nil)
	sentErr := errors.New("install failed")

	// Deliver the error message.
	updatedModel, cmd := m.Update(installDoneMsg{err: sentErr})
	if cmd != nil {
		// The returned cmd must NOT be tea.Quit.
		msg := cmd()
		if _, isQuit := msg.(tea.QuitMsg); isQuit {
			t.Fatal("model quit immediately on error — expected to wait for keypress")
		}
	}

	// Now deliver a keypress — model should quit.
	im := updatedModel.(installModel)
	_, quitCmd := im.Update(tea.KeyPressMsg{})
	if quitCmd == nil {
		t.Fatal("expected a quit command after keypress on error state, got nil")
	}
	msg := quitCmd()
	if _, isQuit := msg.(tea.QuitMsg); !isQuit {
		t.Fatalf("expected QuitMsg after keypress on error state, got %T", msg)
	}
}

// TestInstallModel_SuccessStateDoneFlag verifies that installDoneMsg without
// an error sets the done flag and the View() contains the completion message.
func TestInstallModel_SuccessState(t *testing.T) {
	m := newInstallModel(nil, nil)

	updatedModel, _ := m.Update(installDoneMsg{})

	im, ok := updatedModel.(installModel)
	if !ok {
		t.Fatalf("Update returned unexpected model type %T", updatedModel)
	}

	if !im.done {
		t.Fatal("expected model.done to be true after installDoneMsg without error")
	}
	if im.err != nil {
		t.Errorf("expected model.err to be nil on success, got %v", im.err)
	}

	rendered := im.View().Content
	if !strings.Contains(rendered, "Installation complete") {
		t.Errorf("View() does not contain completion message\nrendered:\n%s", rendered)
	}
}

// TestInstallModel_PhaseTransition verifies that installPhaseMsg transitions
// the model from "resolving" to "installing".
func TestInstallModel_PhaseTransition(t *testing.T) {
	m := newInstallModel(nil, nil)
	if m.phase != "resolving" {
		t.Fatalf("initial phase = %q, want %q", m.phase, "resolving")
	}

	updatedModel, _ := m.Update(installPhaseMsg{})
	im := updatedModel.(installModel)

	if im.phase != "installing" {
		t.Errorf("phase after installPhaseMsg = %q, want %q", im.phase, "installing")
	}
}
