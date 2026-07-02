package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea-151107/pollen/internal/collections"
	"github.com/lea-151107/pollen/internal/env"
	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/userconfig"
)

func mouseTestModel(t *testing.T) Model {
	t.Helper()
	userconfig.SetOverride(t.TempDir())
	t.Cleanup(func() { userconfig.SetOverride("") })
	store, _ := history.Open()
	coll, _ := collections.Open()
	m := New(store, coll, env.New(), Options{MouseEnabled: true})
	m.width, m.height = 120, 30
	return m
}

func click(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
}

func TestHandleMouse_DisabledIsNoop(t *testing.T) {
	m := mouseTestModel(t)
	m.mouseEnabled = false
	m.focus = focusResponse
	m.applyFocus()

	sw, _, _, _ := m.layoutDims()
	nm, _ := m.handleMouse(click(sw+1, 1)) // would otherwise focus the request panel
	if nm.(Model).focus != focusResponse {
		t.Errorf("mouse disabled should be a no-op; focus = %v", nm.(Model).focus)
	}
}

func TestHandleMouse_ClickFocusesPanel(t *testing.T) {
	m := mouseTestModel(t)
	sw, rw, _, _ := m.layoutDims()

	// Click the request column → focus URL.
	m.focus = focusResponse
	m.applyFocus()
	nm, _ := m.handleMouse(click(sw+1, 1))
	if got := nm.(Model).focus; got != focusURL {
		t.Errorf("request-column click: focus = %v, want focusURL", got)
	}

	// Click the response column → focus response.
	m2 := mouseTestModel(t)
	nm2, _ := m2.handleMouse(click(sw+rw+1, 1))
	if got := nm2.(Model).focus; got != focusResponse {
		t.Errorf("response-column click: focus = %v, want focusResponse", got)
	}
}

func TestHandleMouse_HistoryRowClickLoads(t *testing.T) {
	m := mouseTestModel(t)
	m.showHistory = true
	m.showCollections = false
	m.history.SetEntries([]history.Entry{
		{ID: "a", Request: history.Request{Method: "GET", URL: "https://first.example"}},
		{ID: "b", Request: history.Request{Method: "POST", URL: "https://second.example"}},
	})

	// Rows begin at panel-y 2 (border row 0, title row 1). Click the 2nd row.
	nm, _ := m.handleMouse(click(2, 3))
	got := nm.(Model)
	if got.focus != focusURL {
		t.Fatalf("row click should focus URL, got %v", got.focus)
	}
	if got.urlBar.Value() != "https://second.example" {
		t.Errorf("row click loaded URL = %q, want second entry", got.urlBar.Value())
	}
}

func TestHandleMouse_WheelScrollsWithoutPanic(t *testing.T) {
	m := mouseTestModel(t)
	sw, rw, _, _ := m.layoutDims()
	// Wheel over the response column — should not panic and should be handled.
	wheel := tea.MouseMsg{X: sw + rw + 1, Y: 1, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}
	if _, cmd := m.handleMouse(wheel); cmd != nil {
		t.Errorf("response wheel should not emit a command, got %v", cmd)
	}
}
