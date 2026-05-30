package ui

import (
	"strings"
	"testing"

	"github.com/lea-151107/pollen/internal/history"
)

func TestBody_View_CollapsesTabsAtNarrowWidth(t *testing.T) {
	b := NewBody()
	b.Set(history.BodyJSON, "")
	// width 20 → inner 18, full tab strip (~41 cols) cannot fit; expect
	// the collapsed ‹ JSON › form.
	got := b.View(20, 10)
	if !strings.Contains(got, "‹") || !strings.Contains(got, "›") {
		t.Errorf("expected collapsed bar with ‹ ›, got:\n%s", got)
	}
	if !strings.Contains(got, "JSON") {
		t.Errorf("collapsed bar should still show selected label JSON, got:\n%s", got)
	}
	if strings.Contains(got, "MULTIPART") || strings.Contains(got, "GRAPHQL") {
		t.Errorf("collapsed bar should NOT include other tabs, got:\n%s", got)
	}
}

func TestBody_View_FullTabsAtWideWidth(t *testing.T) {
	b := NewBody()
	b.Set(history.BodyJSON, "")
	got := b.View(120, 10)
	for _, label := range []string{"JSON", "FORM", "RAW", "GRAPHQL", "MULTIPART"} {
		if !strings.Contains(got, label) {
			t.Errorf("wide-width bar missing %q, got:\n%s", label, got)
		}
	}
	if strings.Contains(got, "‹") || strings.Contains(got, "›") {
		t.Errorf("wide-width bar should NOT collapse with ‹ ›, got:\n%s", got)
	}
}

func TestBody_View_CollapsedTrackerFollowsSelection(t *testing.T) {
	b := NewBody()
	b.Set(history.BodyMultipart, "")
	got := b.View(20, 10)
	if !strings.Contains(got, "MULTIPART") {
		t.Errorf("collapsed bar should reflect MULTIPART selection, got:\n%s", got)
	}
}
