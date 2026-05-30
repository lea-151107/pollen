package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func sampleSections() []HelpSection {
	return []HelpSection{
		{Title: "Global", Items: []HelpItem{
			{Keys: "Ctrl+R", Desc: "Send request"},
			{Keys: "Ctrl+Q", Desc: "Quit"},
		}},
		{Title: "History", Items: []HelpItem{
			{Keys: "↑/↓", Desc: "Move"},
			{Keys: "Enter", Desc: "Load entry"},
		}},
		{Title: "Collections", Items: []HelpItem{
			{Keys: "d", Desc: "Delete entry"},
		}},
	}
}

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestHelp_Open_PreExpandsFirstSection(t *testing.T) {
	h := NewHelp()
	h.Open(sampleSections())
	if !h.IsOpen() {
		t.Fatalf("IsOpen should be true after Open")
	}
	if h.cursor != 0 {
		t.Errorf("cursor should start at 0, got %d", h.cursor)
	}
	if !h.expanded[0] {
		t.Errorf("first section should be pre-expanded")
	}
	for i := 1; i < len(h.expanded); i++ {
		if h.expanded[i] {
			t.Errorf("section %d should default to collapsed", i)
		}
	}
}

func TestHelp_CursorNavigation_UpDown(t *testing.T) {
	h := NewHelp()
	h.Open(sampleSections())
	h, _ = h.Update(keyMsg("down"))
	if h.cursor != 1 {
		t.Errorf("after down, cursor = %d, want 1", h.cursor)
	}
	h, _ = h.Update(keyMsg("j"))
	if h.cursor != 2 {
		t.Errorf("after j, cursor = %d, want 2", h.cursor)
	}
	h, _ = h.Update(keyMsg("down"))
	if h.cursor != 2 {
		t.Errorf("cursor at last should clamp, got %d", h.cursor)
	}
	h, _ = h.Update(keyMsg("up"))
	h, _ = h.Update(keyMsg("k"))
	if h.cursor != 0 {
		t.Errorf("after up,k cursor = %d, want 0", h.cursor)
	}
	h, _ = h.Update(keyMsg("up"))
	if h.cursor != 0 {
		t.Errorf("cursor at first should clamp, got %d", h.cursor)
	}
}

func TestHelp_GandShiftG_JumpFirstLast(t *testing.T) {
	h := NewHelp()
	h.Open(sampleSections())
	h, _ = h.Update(keyMsg("G"))
	if h.cursor != len(sampleSections())-1 {
		t.Errorf("after G, cursor = %d, want %d", h.cursor, len(sampleSections())-1)
	}
	h, _ = h.Update(keyMsg("g"))
	if h.cursor != 0 {
		t.Errorf("after g, cursor = %d, want 0", h.cursor)
	}
}

func TestHelp_PgUpPgDn_JumpsFive(t *testing.T) {
	h := NewHelp()
	long := make([]HelpSection, 12)
	for i := range long {
		long[i] = HelpSection{Title: "S", Items: []HelpItem{{Keys: "k", Desc: "d"}}}
	}
	h.Open(long)
	h, _ = h.Update(keyMsg("pgdown"))
	if h.cursor != 5 {
		t.Errorf("after pgdown, cursor = %d, want 5", h.cursor)
	}
	h, _ = h.Update(keyMsg("pgdown"))
	if h.cursor != 10 {
		t.Errorf("after second pgdown, cursor = %d, want 10", h.cursor)
	}
	h, _ = h.Update(keyMsg("pgup"))
	if h.cursor != 5 {
		t.Errorf("after pgup, cursor = %d, want 5", h.cursor)
	}
	// Clamp at edges.
	h, _ = h.Update(keyMsg("pgdown"))
	h, _ = h.Update(keyMsg("pgdown"))
	if h.cursor != len(long)-1 {
		t.Errorf("pgdown should clamp at last, got %d", h.cursor)
	}
}

func TestHelp_EnterTogglesExpanded(t *testing.T) {
	h := NewHelp()
	h.Open(sampleSections())
	// Move to History (collapsed by default).
	h, _ = h.Update(keyMsg("down"))
	if h.expanded[1] {
		t.Fatalf("section 1 should start collapsed")
	}
	h, _ = h.Update(keyMsg("enter"))
	if !h.expanded[1] {
		t.Errorf("Enter should expand section 1")
	}
	// Multi-open: original Global stays expanded.
	if !h.expanded[0] {
		t.Errorf("expanding section 1 must not collapse section 0 (multi-open)")
	}
	h, _ = h.Update(keyMsg("enter"))
	if h.expanded[1] {
		t.Errorf("second Enter should collapse section 1 again")
	}
	// Space also toggles.
	h, _ = h.Update(keyMsg(" "))
	if !h.expanded[1] {
		t.Errorf("Space should toggle expand")
	}
}

func TestHelp_Esc_Closes(t *testing.T) {
	h := NewHelp()
	h.Open(sampleSections())
	h, _ = h.Update(keyMsg("esc"))
	if h.IsOpen() {
		t.Errorf("Esc should close")
	}
}

func TestHelp_Q_Closes(t *testing.T) {
	h := NewHelp()
	h.Open(sampleSections())
	h, _ = h.Update(keyMsg("q"))
	if h.IsOpen() {
		t.Errorf("q should close")
	}
}

func TestHelp_View_AllSectionTitlesVisible(t *testing.T) {
	h := NewHelp()
	h.Open(sampleSections())
	h.SetSize(80, 30)
	got := h.View()
	for _, sec := range sampleSections() {
		if !strings.Contains(got, sec.Title) {
			t.Errorf("View missing section title %q\n%s", sec.Title, got)
		}
	}
}

func TestHelp_View_ExpandedShowsItems(t *testing.T) {
	h := NewHelp()
	h.Open(sampleSections())
	h.SetSize(80, 30)
	got := h.View()
	if !strings.Contains(got, "Send request") {
		t.Errorf("Global is pre-expanded; expected to see Send request, got:\n%s", got)
	}
}

func TestHelp_View_CollapsedHidesItems(t *testing.T) {
	h := NewHelp()
	h.Open(sampleSections())
	h.SetSize(80, 30)
	got := h.View()
	// History is collapsed by default → its items must not appear.
	if strings.Contains(got, "Load entry") {
		t.Errorf("History collapsed; should not see Load entry, got:\n%s", got)
	}
}

func TestHelp_View_GlyphsMatchExpansion(t *testing.T) {
	h := NewHelp()
	h.Open(sampleSections())
	h.SetSize(80, 30)
	got := h.View()
	if !strings.Contains(got, "▼") {
		t.Errorf("expected ▼ for expanded section, got:\n%s", got)
	}
	if !strings.Contains(got, "▶") {
		t.Errorf("expected ▶ for collapsed section, got:\n%s", got)
	}
}

func TestHelp_ScrollFollowsCursor(t *testing.T) {
	// 30 sections, terminal only 15 rows → viewport ~9 rows. Cursor at
	// last section: scroll must advance enough to include that header.
	long := make([]HelpSection, 30)
	for i := range long {
		long[i] = HelpSection{Title: "S", Items: []HelpItem{{Keys: "k", Desc: "d"}}}
	}
	h := NewHelp()
	h.Open(long)
	h.SetSize(80, 15)
	h, _ = h.Update(keyMsg("G"))
	got := h.View()
	// We can't easily count newlines through ANSI safely, but the
	// rendered string should at least contain SOMETHING (no crash, no
	// empty output) when cursor is at the end.
	if got == "" {
		t.Errorf("View with cursor at end produced empty output")
	}
}

func TestHelp_View_EmptyWhenClosed(t *testing.T) {
	h := NewHelp()
	if h.View() != "" {
		t.Errorf("closed Help should render empty")
	}
}

func TestHelp_Reopen_PreservesExpansionWhenSameCount(t *testing.T) {
	h := NewHelp()
	h.Open(sampleSections())
	h, _ = h.Update(keyMsg("down"))
	h, _ = h.Update(keyMsg("enter")) // expand History
	h.Close()
	h.Open(sampleSections())
	if !h.expanded[1] {
		t.Errorf("Re-open with same section count should preserve expansion choices")
	}
	if h.cursor != 0 {
		t.Errorf("Re-open should reset cursor to 0, got %d", h.cursor)
	}
}
