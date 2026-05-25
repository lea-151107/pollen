package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/lea/pollen/internal/history"
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
