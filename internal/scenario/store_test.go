package scenario

import (
	"testing"

	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/userconfig"
)

func TestStore_RoundTrip(t *testing.T) {
	userconfig.SetOverride(t.TempDir())
	t.Cleanup(func() { userconfig.SetOverride("") })

	s, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(s.Entries()) != 0 {
		t.Fatalf("fresh store should be empty, got %d", len(s.Entries()))
	}

	sc := Scenario{
		ID:   "id1",
		Name: "Login Flow",
		Steps: []Step{
			{Name: "login", Request: history.Request{Method: "POST", URL: "https://x/login"}},
		},
	}
	s.Add(sc)
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reopened, err := Open()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if len(reopened.Entries()) != 1 {
		t.Fatalf("want 1 entry after reload, got %d", len(reopened.Entries()))
	}
	got, ok := reopened.ByName("login flow") // case-insensitive
	if !ok {
		t.Fatal("ByName should find scenario case-insensitively")
	}
	if got.ID != "id1" || len(got.Steps) != 1 {
		t.Errorf("unexpected reloaded scenario: %+v", got)
	}
}

func TestStore_ReplaceAndDelete(t *testing.T) {
	s := &Store{}
	s.Add(Scenario{ID: "a", Name: "A"})
	s.Add(Scenario{ID: "b", Name: "B"})

	if !s.Replace("a", Scenario{ID: "a", Name: "A2"}) {
		t.Fatal("Replace should succeed for existing id")
	}
	if s.Entries()[0].Name != "A2" {
		t.Errorf("Replace did not update: %+v", s.Entries()[0])
	}
	if s.Replace("missing", Scenario{}) {
		t.Error("Replace should fail for unknown id")
	}
	if !s.DeleteAt(0) || len(s.Entries()) != 1 {
		t.Errorf("DeleteAt failed, entries=%d", len(s.Entries()))
	}
	if s.IndexOf("b") != 0 {
		t.Errorf("IndexOf(b) = %d, want 0", s.IndexOf("b"))
	}
}
