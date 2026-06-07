package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea-151107/pollen/internal/settings"
)

// enterSettingsEdit opens the Settings overlay and drives it (through the
// app's own Update routing, so the key path under test is exercised) into
// edit mode on the first non-bool field: down once from cursor 0 (the
// "TLS verification skip" bool) to cursor 1 ("HTTP request timeout", an
// int), then Enter to open the field editor.
func enterSettingsEdit(t *testing.T, m Model) Model {
	t.Helper()
	m.settingsPanel.Open(settings.Defaults())
	step := func(km tea.KeyMsg) {
		updated, _ := m.Update(km)
		m = updated.(Model)
	}
	step(tea.KeyMsg{Type: tea.KeyDown})
	step(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.settingsPanel.IsEditing() {
		t.Fatalf("setup: expected settings panel to be in edit mode")
	}
	return m
}

// TestSettings_CtrlP_WhileEditing_DoesNotClose pins the fix: the Settings
// toggle key (Ctrl+P, added in v1.7.1) must not punch through the panel's
// deliberate "no accidental close while editing" protection. Pressing it
// mid-edit previously called Close() before delegating to the panel,
// discarding the in-progress edit.
func TestSettings_CtrlP_WhileEditing_DoesNotClose(t *testing.T) {
	m := newApplyTestModel(t)
	m = enterSettingsEdit(t, m)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = updated.(Model)

	if !m.settingsPanel.IsOpen() {
		t.Errorf("Ctrl+P while editing must not close the Settings overlay")
	}
	if !m.settingsPanel.IsEditing() {
		t.Errorf("Ctrl+P while editing must keep the field editor active")
	}
}

// TestSettings_CtrlP_WhileNavigating_Closes guards the toggle's normal
// behaviour: from navigation mode (not editing), Ctrl+P still closes the
// overlay.
func TestSettings_CtrlP_WhileNavigating_Closes(t *testing.T) {
	m := newApplyTestModel(t)
	m.settingsPanel.Open(settings.Defaults())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = updated.(Model)

	if m.settingsPanel.IsOpen() {
		t.Errorf("Ctrl+P from navigation mode should close the Settings overlay")
	}
}
