package app

import "testing"

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
