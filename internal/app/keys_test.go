package app

import "testing"

// TestIsTextEditingFocus_CoversCtrlPCollisions pins the v1.7.2
// regression check: the Settings binding (Ctrl+P alongside Ctrl+,)
// is now guarded by isTextEditingFocus so it doesn't shadow
// bubbles textarea's LinePrevious (default ctrl+p) or textinput's
// PrevSuggestion (default ctrl+p). The guard must return true for
// every focus area whose underlying widget binds Ctrl+P by
// default, otherwise the bug returns.
func TestIsTextEditingFocus_CoversCtrlPCollisions(t *testing.T) {
	// Body editor mode → textarea Ctrl+P (LinePrevious) must reach
	// the textarea, not open Settings.
	if !isTextEditingFocus(focusBody, true, false, false, false) {
		t.Errorf("focusBody in editor mode should be text-editing")
	}
	// Headers / URL / Auth / Query use textinput which binds
	// Ctrl+P to PrevSuggestion. The guard must apply unconditionally
	// for these focus areas.
	for _, f := range []focusArea{focusHeaders, focusURL, focusAuth, focusQuery} {
		if !isTextEditingFocus(f, false, false, false, false) {
			t.Errorf("focusArea %v should be text-editing (textinput Ctrl+P collision)", f)
		}
	}
	// Filter modes on the sidebars also expose a textinput.
	if !isTextEditingFocus(focusHistory, false, true, false, false) {
		t.Errorf("focusHistory in filter mode should be text-editing")
	}
	if !isTextEditingFocus(focusCollections, false, false, true, false) {
		t.Errorf("focusCollections in filter mode should be text-editing")
	}
	// Response panel in search / filter input.
	if !isTextEditingFocus(focusResponse, false, false, false, true) {
		t.Errorf("focusResponse with search/filter active should be text-editing")
	}
}

// TestIsTextEditingFocus_FallthroughCases pins that focuses
// without an active textinput pass Ctrl+P through to Settings.
func TestIsTextEditingFocus_FallthroughCases(t *testing.T) {
	// Body when NOT in editor mode (just tab-focused, navigating).
	if isTextEditingFocus(focusBody, false, false, false, false) {
		t.Errorf("focusBody outside editor mode should not be text-editing")
	}
	// Response without any active input prompt.
	if isTextEditingFocus(focusResponse, false, false, false, false) {
		t.Errorf("focusResponse with no input should not be text-editing")
	}
	// History / Collections without filter mode.
	if isTextEditingFocus(focusHistory, false, false, false, false) {
		t.Errorf("focusHistory without filter should not be text-editing")
	}
	if isTextEditingFocus(focusCollections, false, false, false, false) {
		t.Errorf("focusCollections without filter should not be text-editing")
	}
}

// TestKeyMap_SettingsBindsCtrlP pins the v1.7.1 fix: standard
// terminals can't transmit Ctrl+, (no traditional ASCII control
// code for the comma character), so the Settings overlay needs a
// universally-available primary key. Ctrl+P maps to ASCII 0x10
// (DLE) on every terminal. Ctrl+, is kept as an alias for kitty /
// CSI-u-capable terminals where the muscle memory from VS Code
// etc. still works.
func TestKeyMap_SettingsBindsCtrlP(t *testing.T) {
	km := DefaultKeyMap()
	keys := km.Settings.Keys()
	foundP, foundComma := false, false
	for _, k := range keys {
		switch k {
		case "ctrl+p":
			foundP = true
		case "ctrl+,":
			foundComma = true
		}
	}
	if !foundP {
		t.Errorf("Settings should bind ctrl+p (universally available); keys = %v", keys)
	}
	if !foundComma {
		t.Errorf("Settings should retain ctrl+, alias for CSI-u terminals; keys = %v", keys)
	}
}
