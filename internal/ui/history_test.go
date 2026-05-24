package ui

import (
	"strings"
	"testing"
	"time"
)

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
