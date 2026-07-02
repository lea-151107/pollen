package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea-151107/pollen/internal/settings"
)

func sampleSettings() *settings.Settings {
	return &settings.Settings{
		SkipTLSVerify:              false,
		ResponsePanelRatio:         0.5,
		RequestTimeoutSecs:         60,
		MaxResponseMiB:             32,
		HistoryLimit:               200,
		TextPreviewKiB:             100,
		SidebarMaxWidth:            40,
		HexDumpKiB:                 4,
		ProxyURL:                   "",
		DisableRedirects:           false,
		CACertFile:                 "",
		EnableCookies:              false,
		IntruderConcurrency:        5,
		IntruderDelayMs:            0,
		IntruderMaxRequests:        1000,
		IntruderResponseBodyCapKiB: 64,
		OAuthPersistTokens:         true,
	}
}

func settingsKeyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// typeIntoEditor sends a string of runes into a SettingsPanel in
// edit mode. Each rune goes through Update individually so the
// textinput.Model sees them as separate key events.
func typeIntoEditor(p SettingsPanel, s string) SettingsPanel {
	for _, r := range s {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return p
}

func TestSettings_Open_DraftMatchesInput(t *testing.T) {
	p := NewSettingsPanel()
	s := sampleSettings()
	p.Open(s)
	if !p.IsOpen() {
		t.Fatalf("IsOpen should be true after Open")
	}
	if p.cursor != 0 {
		t.Errorf("cursor should start at 0")
	}
	if p.draft.RequestTimeoutSecs != 60 {
		t.Errorf("draft should reflect input")
	}
	// Mutating the draft should not affect the input (deep copy).
	p.draft.RequestTimeoutSecs = 999
	if s.RequestTimeoutSecs != 60 {
		t.Errorf("Open did not deep-copy: original was mutated")
	}
}

func TestSettings_CursorNavigation_UpDown(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	p, _ = p.Update(settingsKeyMsg("down"))
	if p.cursor != 1 {
		t.Errorf("down → cursor = %d, want 1", p.cursor)
	}
	p, _ = p.Update(settingsKeyMsg("up"))
	if p.cursor != 0 {
		t.Errorf("up → cursor = %d, want 0", p.cursor)
	}
	// Clamp at 0.
	p, _ = p.Update(settingsKeyMsg("up"))
	if p.cursor != 0 {
		t.Errorf("up at 0 should clamp")
	}
}

func TestSettings_g_G_JumpsFirstLast(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	p, _ = p.Update(settingsKeyMsg("G"))
	if p.cursor != len(p.fields)-1 {
		t.Errorf("G → cursor = %d, want %d", p.cursor, len(p.fields)-1)
	}
	p, _ = p.Update(settingsKeyMsg("g"))
	if p.cursor != 0 {
		t.Errorf("g → cursor = %d, want 0", p.cursor)
	}
}

func TestSettings_BoolToggle_EmitsAppliedMsg(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	// Cursor 0 is "TLS verification skip" (bool). Enter toggles it.
	p, cmd := p.Update(settingsKeyMsg("enter"))
	if cmd == nil {
		t.Fatalf("toggle should emit a Cmd")
	}
	if !p.draft.SkipTLSVerify {
		t.Errorf("draft.SkipTLSVerify should be true after toggle")
	}
	msg := cmd()
	applied, ok := msg.(SettingsAppliedMsg)
	if !ok {
		t.Fatalf("expected SettingsAppliedMsg, got %T", msg)
	}
	if !applied.Setting.SkipTLSVerify {
		t.Errorf("applied msg should carry toggled value")
	}
}

func TestSettings_IntEdit_ValidatesRangeTooSmall(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	// Move to "HTTP request timeout" (cursor 1, int field).
	p, _ = p.Update(settingsKeyMsg("down"))
	// Enter edit mode.
	p, _ = p.Update(settingsKeyMsg("enter"))
	if !p.editing {
		t.Fatalf("should be in edit mode")
	}
	// Clear existing value and type "0" (below min 1).
	p.editor.SetValue("0")
	p, cmd := p.Update(settingsKeyMsg("enter"))
	if cmd != nil {
		t.Errorf("invalid commit should not emit Cmd")
	}
	if !p.editing {
		t.Errorf("should stay in edit mode on validation failure")
	}
	if p.editErr == "" {
		t.Errorf("should display validation error")
	}
}

func TestSettings_IntEdit_ValidatesRangeTooLarge(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	p, _ = p.Update(settingsKeyMsg("down"))
	p, _ = p.Update(settingsKeyMsg("enter"))
	p.editor.SetValue("99999")
	p, cmd := p.Update(settingsKeyMsg("enter"))
	if cmd != nil {
		t.Errorf("invalid commit should not emit Cmd")
	}
	if !p.editing || p.editErr == "" {
		t.Errorf("should stay in edit mode with error")
	}
}

func TestSettings_IntEdit_AppliesValidValue(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	p, _ = p.Update(settingsKeyMsg("down")) // HTTP request timeout
	p, _ = p.Update(settingsKeyMsg("enter"))
	p.editor.SetValue("120")
	p, cmd := p.Update(settingsKeyMsg("enter"))
	if cmd == nil {
		t.Fatalf("valid commit should emit Cmd")
	}
	if p.editing {
		t.Errorf("should exit edit mode after successful commit")
	}
	if p.draft.RequestTimeoutSecs != 120 {
		t.Errorf("draft should be updated, got %d", p.draft.RequestTimeoutSecs)
	}
	msg := cmd()
	applied, ok := msg.(SettingsAppliedMsg)
	if !ok || applied.Setting.RequestTimeoutSecs != 120 {
		t.Errorf("applied msg should carry new value")
	}
}

func TestSettings_FloatEdit_RejectsNaN(t *testing.T) {
	// v1.7.2 regression: NaN comparisons in IEEE 754 are always
	// false, so a raw "NaN" string would slip past the range check
	// and land in settings.json, breaking layout math that uses the
	// ratio.
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	p, _ = p.Update(settingsKeyMsg("G")) // Response panel ratio (last field)
	p, _ = p.Update(settingsKeyMsg("enter"))
	p.editor.SetValue("NaN")
	p, cmd := p.Update(settingsKeyMsg("enter"))
	if cmd != nil {
		t.Errorf("NaN must not commit")
	}
	if !p.editing || p.editErr == "" {
		t.Errorf("editor should stay open with a validation error")
	}
	if p.draft.ResponsePanelRatio != 0.5 {
		t.Errorf("draft must not be polluted with NaN, got %v", p.draft.ResponsePanelRatio)
	}
}

func TestSettings_FloatEdit_RejectsPosInf(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	p, _ = p.Update(settingsKeyMsg("G"))
	p, _ = p.Update(settingsKeyMsg("enter"))
	p.editor.SetValue("Inf")
	p, cmd := p.Update(settingsKeyMsg("enter"))
	if cmd != nil {
		t.Errorf("+Inf must not commit")
	}
}

func TestSettings_FloatEdit_RejectsNegInf(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	p, _ = p.Update(settingsKeyMsg("G"))
	p, _ = p.Update(settingsKeyMsg("enter"))
	p.editor.SetValue("-Inf")
	p, cmd := p.Update(settingsKeyMsg("enter"))
	if cmd != nil {
		t.Errorf("-Inf must not commit")
	}
}

func TestSettings_FloatEdit_ValidatesRatio(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	// Last field is "Response panel ratio" (float).
	p, _ = p.Update(settingsKeyMsg("G"))
	p, _ = p.Update(settingsKeyMsg("enter"))
	p.editor.SetValue("1.5") // out of (0, 1) range
	p, cmd := p.Update(settingsKeyMsg("enter"))
	if cmd != nil {
		t.Errorf("out-of-range float should not commit")
	}
	if !p.editing || p.editErr == "" {
		t.Errorf("should stay in edit with error")
	}
}

func TestSettings_StringEdit_ProxyURLValidation(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	// Find Proxy URL field by stepping through fields.
	for i, f := range p.fields {
		if f.label == "Proxy URL" {
			p.cursor = i
			break
		}
	}
	p, _ = p.Update(settingsKeyMsg("enter"))
	p.editor.SetValue("http://example.com:8080")
	p, cmd := p.Update(settingsKeyMsg("enter"))
	if cmd == nil {
		t.Fatalf("valid URL should commit")
	}
	if p.draft.ProxyURL != "http://example.com:8080" {
		t.Errorf("draft.ProxyURL = %q", p.draft.ProxyURL)
	}
}

func TestSettings_EscFromEditDiscards(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	p, _ = p.Update(settingsKeyMsg("down")) // HTTP request timeout
	p, _ = p.Update(settingsKeyMsg("enter"))
	p.editor.SetValue("999")
	p, _ = p.Update(settingsKeyMsg("esc"))
	if p.editing {
		t.Errorf("Esc should exit edit mode")
	}
	if p.draft.RequestTimeoutSecs != 60 {
		t.Errorf("Esc should discard, draft = %d, want 60", p.draft.RequestTimeoutSecs)
	}
}

func TestSettings_EscFromNavigateCloses(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	p, _ = p.Update(settingsKeyMsg("esc"))
	if p.IsOpen() {
		t.Errorf("Esc from nav should close")
	}
}

func TestSettings_QClosesFromNavigate(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	p, _ = p.Update(settingsKeyMsg("q"))
	if p.IsOpen() {
		t.Errorf("q from nav should close")
	}
}

func TestSettings_RestartFieldShowsBadge(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	p.SetSize(120, 40)
	got := p.View()
	// CA cert file and Enable cookies are restart-required.
	if !strings.Contains(got, "restart") {
		t.Errorf("View should show 'restart' badge for CA cert / cookies, got:\n%s", got)
	}
}

func TestSettings_NavigatesAllFields(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	// G jumps to last; verify we have 18 fields.
	p, _ = p.Update(settingsKeyMsg("G"))
	if p.cursor != 17 {
		t.Errorf("expected 18 fields (cursor 0-17 at G), got %d", p.cursor)
	}
}

func TestSettings_View_ShowsCurrentValues(t *testing.T) {
	p := NewSettingsPanel()
	p.Open(sampleSettings())
	p.SetSize(120, 40)
	got := p.View()
	if !strings.Contains(got, "60 sec") {
		t.Errorf("View should show timeout '60 sec', got:\n%s", got)
	}
	if !strings.Contains(got, "200 entries") {
		t.Errorf("View should show history limit '200 entries'")
	}
}

func TestSettings_Closed_ViewIsEmpty(t *testing.T) {
	p := NewSettingsPanel()
	if p.View() != "" {
		t.Errorf("closed panel should render empty")
	}
}
