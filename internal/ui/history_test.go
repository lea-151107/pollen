package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/lea-151107/pollen/internal/history"
)

func TestHistory_FilterMatches(t *testing.T) {
	h := NewHistory()
	h.SetEntries([]history.Entry{
		{ID: "1", Request: history.Request{Method: "GET", URL: "https://example.com/users"}},
		{ID: "2", Request: history.Request{Method: "POST", URL: "https://example.com/login"}},
		{ID: "3", Request: history.Request{Method: "GET", URL: "https://api.other.com/items"}},
	})
	h.filter = "example"
	got := h.filtered()
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	if got[0].ID != "1" || got[1].ID != "2" {
		t.Errorf("got IDs %s/%s, want 1/2", got[0].ID, got[1].ID)
	}
}

func TestHistory_FilterCaseInsensitive(t *testing.T) {
	h := NewHistory()
	h.SetEntries([]history.Entry{
		{ID: "1", Request: history.Request{Method: "GET", URL: "https://EXAMPLE.com/"}},
	})
	h.filter = "example"
	if len(h.filtered()) != 1 {
		t.Errorf("filter should be case-insensitive")
	}
}

func TestHistory_FilterMatchesMethod(t *testing.T) {
	h := NewHistory()
	h.SetEntries([]history.Entry{
		{ID: "1", Request: history.Request{Method: "DELETE", URL: "https://a.com"}},
		{ID: "2", Request: history.Request{Method: "GET", URL: "https://a.com"}},
	})
	h.filter = "delete"
	got := h.filtered()
	if len(got) != 1 || got[0].ID != "1" {
		t.Errorf("method filter failed: %+v", got)
	}
}

func TestHistory_EmptyFilterReturnsAll(t *testing.T) {
	entries := []history.Entry{
		{ID: "1", Request: history.Request{Method: "GET", URL: "https://a.com"}},
		{ID: "2", Request: history.Request{Method: "GET", URL: "https://b.com"}},
	}
	h := NewHistory()
	h.SetEntries(entries)
	if len(h.filtered()) != 2 {
		t.Errorf("empty filter should return all")
	}
}

func TestHistory_FilterClampSelected(t *testing.T) {
	h := NewHistory()
	h.SetEntries([]history.Entry{
		{ID: "1", Request: history.Request{Method: "GET", URL: "https://a.com"}},
		{ID: "2", Request: history.Request{Method: "GET", URL: "https://b.com"}},
		{ID: "3", Request: history.Request{Method: "GET", URL: "https://c.com"}},
	})
	h.selected = 2
	h.filter = "a.com"
	h.SetEntries(h.entries) // re-clamp
	if h.selected != 0 {
		t.Errorf("selected should clamp to filtered range, got %d", h.selected)
	}
}

// TestHistory_SelectByID_ReanchorAcrossPrepend is the regression test for the
// cursor-desync bug: after a request completes, the app captures the selected
// entry's ID, Prepends the new entry, re-sets the entries, and re-anchors by
// ID. When a filter is active and the newly-prepended entry does NOT match it,
// the filtered list doesn't move, so the cursor must stay on the same entry —
// the old blind Shift(1) wrongly slid it down by one.
func TestHistory_SelectByID_ReanchorAcrossPrepend(t *testing.T) {
	h := NewHistory()
	h.SetEntries([]history.Entry{
		{ID: "old1", Request: history.Request{Method: "GET", URL: "https://example.com/a"}},
		{ID: "old2", Request: history.Request{Method: "GET", URL: "https://example.com/b"}},
	})
	h.filter = "example" // active filter
	h.selected = 0        // user selected the top filtered row (old1)

	// Capture selection, then simulate a Prepend of a NON-matching entry.
	selID := ""
	if e := h.Selected(); e != nil {
		selID = e.ID
	}
	if selID != "old1" {
		t.Fatalf("precondition: selected should be old1, got %q", selID)
	}
	h.SetEntries([]history.Entry{
		{ID: "new", Request: history.Request{Method: "GET", URL: "https://other.com/x"}}, // doesn't match "example"
		{ID: "old1", Request: history.Request{Method: "GET", URL: "https://example.com/a"}},
		{ID: "old2", Request: history.Request{Method: "GET", URL: "https://example.com/b"}},
	})
	h.SelectByID(selID)

	if got := h.Selected(); got == nil || got.ID != "old1" {
		t.Fatalf("cursor should stay on old1 after non-matching prepend, got %+v", got)
	}
}

// TestHistory_SelectByID_MatchingPrepend covers the ordinary case: the new
// entry matches the filter (or there's no filter), so it lands at filtered
// index 0 and the previously-selected entry moves down by one — re-anchoring by
// ID must follow it.
func TestHistory_SelectByID_MatchingPrepend(t *testing.T) {
	h := NewHistory()
	h.SetEntries([]history.Entry{
		{ID: "old1", Request: history.Request{Method: "GET", URL: "https://example.com/a"}},
	})
	h.selected = 0
	selID := h.Selected().ID
	h.SetEntries([]history.Entry{
		{ID: "new", Request: history.Request{Method: "GET", URL: "https://example.com/new"}},
		{ID: "old1", Request: history.Request{Method: "GET", URL: "https://example.com/a"}},
	})
	h.SelectByID(selID)
	if got := h.Selected(); got == nil || got.ID != "old1" {
		t.Fatalf("cursor should follow old1 to index 1, got %+v", got)
	}
	if h.selected != 1 {
		t.Errorf("selected index should be 1, got %d", h.selected)
	}
}

func TestFormatRelative(t *testing.T) {
	now := time.Now()
	cases := []struct {
		t    time.Time
		want string
	}{
		{time.Time{}, ""},
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5m ago"},
		{now.Add(-90 * time.Minute), "1h ago"},
		{now.Add(-25 * time.Hour), "1d ago"},
		{now.Add(72 * time.Hour), "soon"}, // future clock skew
	}
	for _, c := range cases {
		got := formatRelative(c.t)
		if got != c.want {
			t.Errorf("formatRelative(%v): got %q want %q", c.t, got, c.want)
		}
	}
}

func TestFormatRelative_LargeDay(t *testing.T) {
	got := formatRelative(time.Now().Add(-72 * time.Hour))
	if !strings.HasSuffix(got, "d ago") {
		t.Errorf("expected '...d ago', got %q", got)
	}
}

// stripANSI removes ANSI CSI escape sequences so plain-text content can be
// compared even when lipgloss wrapped the result in styling codes.
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			// Skip ESC '[' ... until a final byte in [@-~].
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

func TestHighlightMatch_ASCII(t *testing.T) {
	got := highlightMatch("Hello World", "world")
	plain := stripANSI(got)
	if plain != "Hello World" {
		t.Errorf("plain text should be preserved, got %q", plain)
	}
	if !strings.Contains(got, "World") {
		t.Errorf("match should appear in output: %q", got)
	}
}

func TestHighlightMatch_NoMatch(t *testing.T) {
	got := highlightMatch("Hello", "xyz")
	if got != "Hello" {
		t.Errorf("no match should return original: %q", got)
	}
}

func TestHighlightMatch_EmptyNeedle(t *testing.T) {
	got := highlightMatch("Hello", "")
	if got != "Hello" {
		t.Errorf("empty needle should return original: %q", got)
	}
}

// TestHighlightMatch_KelvinSign is the regression test for the bug where
// strings.ToLower changed byte widths (U+212A KELVIN SIGN → 'k') so the byte
// index from the lowercased string misaligned slicing on the original. After
// the fix the highlighter should still produce valid UTF-8 with the original
// text content fully preserved.
func TestHighlightMatch_KelvinSign(t *testing.T) {
	// U+212A KELVIN SIGN: 3 bytes (E2 84 AA), lowercases to 'k' (1 byte).
	text := "hiKworld" // "hiKworld" where K is U+212A
	got := highlightMatch(text, "k")
	plain := stripANSI(got)
	if plain != text {
		t.Errorf("plain content should equal original, got %q want %q", plain, text)
	}
	if !strings.ContainsRune(plain, 'K') {
		t.Errorf("Kelvin sign rune lost in output: %q", plain)
	}
}

func TestHighlightMatchColored_KelvinSign(t *testing.T) {
	text := "hiKworld"
	got := highlightMatchColored(text, len([]rune(text)), "k", lipgloss.Color("44"))
	plain := stripANSI(got)
	if !strings.ContainsRune(plain, 'K') {
		t.Errorf("Kelvin sign rune lost: %q", plain)
	}
}

func TestCaseInsensitiveIndex_KelvinSign(t *testing.T) {
	// U+212A is 3 bytes at positions 2-4; expecting [2, 5) byte range.
	start, end, ok := caseInsensitiveIndex("hiKworld", "k")
	if !ok {
		t.Fatal("should find a match")
	}
	if start != 2 || end != 5 {
		t.Errorf("got [%d,%d) want [2,5)", start, end)
	}
}

func TestCaseInsensitiveIndex_Multibyte(t *testing.T) {
	// Pure ASCII smoke test.
	start, end, ok := caseInsensitiveIndex("Hello World", "world")
	if !ok || start != 6 || end != 11 {
		t.Errorf("got start=%d end=%d ok=%v want 6,11,true", start, end, ok)
	}
}
