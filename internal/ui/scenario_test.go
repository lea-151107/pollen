package ui

import (
	"testing"

	"github.com/lea-151107/pollen/internal/collections"
	"github.com/lea-151107/pollen/internal/history"
)

func TestSanitizeToken(t *testing.T) {
	cases := map[string]string{
		"Login Request":   "login_request",
		"GET /users/{id}": "get__users_id",
		"  spaced  ":      "spaced",
		"UPPER-case":      "upper_case",
		"":                "",
	}
	for in, want := range cases {
		if got := sanitizeToken(in); got != want {
			t.Errorf("sanitizeToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildScenario_Validation(t *testing.T) {
	m := NewScenario()
	m.OpenBuild([]collections.Entry{
		{ID: "1", Name: "Login", Request: history.Request{Method: "POST", URL: "u"}},
	})

	// No name yet.
	if _, err := m.BuildScenario(); err == nil {
		t.Error("expected error when name is empty")
	}

	m.nameInput.SetValue("flow")
	// No steps yet.
	if _, err := m.BuildScenario(); err == nil {
		t.Error("expected error when there are no steps")
	}

	m.collCursor = 0
	m.addStep()
	sc, err := m.BuildScenario()
	if err != nil {
		t.Fatalf("BuildScenario: %v", err)
	}
	if sc.Name != "flow" || len(sc.Steps) != 1 {
		t.Fatalf("unexpected scenario: %+v", sc)
	}
	if sc.Steps[0].Name != "login" {
		t.Errorf("step name = %q, want derived token %q", sc.Steps[0].Name, "login")
	}
	if sc.Steps[0].FromCollectionID != "1" {
		t.Errorf("step should record source collection id, got %q", sc.Steps[0].FromCollectionID)
	}
}

func TestAddStep_UniqueNames(t *testing.T) {
	m := NewScenario()
	m.OpenBuild([]collections.Entry{
		{ID: "1", Name: "Login", Request: history.Request{Method: "POST"}},
	})
	m.collCursor = 0
	m.addStep()
	m.addStep() // same entry again → must not collide
	if len(m.steps) != 2 {
		t.Fatalf("want 2 steps, got %d", len(m.steps))
	}
	if m.steps[0].Name == m.steps[1].Name {
		t.Errorf("step names must be unique, both %q", m.steps[0].Name)
	}
	if m.steps[1].Name != "login_2" {
		t.Errorf("second step name = %q, want %q", m.steps[1].Name, "login_2")
	}
}
