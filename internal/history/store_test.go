package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return &Store{path: filepath.Join(dir, "history.json")}
}

func TestStore_SaveLoad(t *testing.T) {
	s := newTestStore(t)
	s.Prepend(Entry{
		ID:        "abc",
		Timestamp: time.Now().UTC().Round(time.Second),
		Request: Request{
			Method:   "POST",
			URL:      "https://example.com",
			Headers:  []Header{{Key: "Accept", Value: "*/*"}},
			Body:     `{"a":1}`,
			BodyType: BodyJSON,
		},
		Response: &Response{Status: 200, StatusText: "200 OK", Body: "ok", DurationMs: 10, SizeBytes: 2},
	})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(s.path); err != nil {
		t.Fatalf("file not written: %v", err)
	}

	s2 := &Store{path: s.path}
	data, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	_ = data
	// Reuse Open's logic by simulating: read & unmarshal.
	loaded, err := loadFromPath(s2.path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Entries()) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded.Entries()))
	}
	got := loaded.Entries()[0]
	if got.ID != "abc" || got.Request.Method != "POST" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestStore_DeleteAt(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 3; i++ {
		s.Prepend(Entry{ID: string(rune('a' + i))})
	}
	// entries are [c, b, a] after prepends
	if !s.DeleteAt(1) {
		t.Fatal("DeleteAt(1) should succeed")
	}
	if len(s.Entries()) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(s.Entries()))
	}
	if s.Entries()[0].ID != "c" || s.Entries()[1].ID != "a" {
		t.Errorf("wrong remaining entries: %+v", s.Entries())
	}
	if s.DeleteAt(-1) {
		t.Error("DeleteAt(-1) should return false")
	}
	if s.DeleteAt(10) {
		t.Error("DeleteAt(10) should return false")
	}
}

func TestStore_InsertAt(t *testing.T) {
	s := newTestStore(t)
	s.Prepend(Entry{ID: "c"})
	s.Prepend(Entry{ID: "b"})
	s.Prepend(Entry{ID: "a"})
	// [a, b, c]

	s.InsertAt(1, Entry{ID: "x"})
	// [a, x, b, c]
	got := s.Entries()
	want := []string{"a", "x", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len: got %d want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].ID != w {
			t.Errorf("at %d: got %q want %q", i, got[i].ID, w)
		}
	}
}

func TestStore_InsertAt_OutOfRange(t *testing.T) {
	s := newTestStore(t)
	s.Prepend(Entry{ID: "a"})

	// Negative index → prepend
	s.InsertAt(-5, Entry{ID: "x"})
	if s.Entries()[0].ID != "x" {
		t.Errorf("negative index should prepend, got %+v", s.Entries())
	}

	// Index beyond len → append
	s.InsertAt(999, Entry{ID: "z"})
	last := s.Entries()[len(s.Entries())-1]
	if last.ID != "z" {
		t.Errorf("out-of-range index should append, got %+v", s.Entries())
	}
}

func TestStore_PrependLimit(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < maxEntries+50; i++ {
		s.Prepend(Entry{ID: "x"})
	}
	if len(s.Entries()) != maxEntries {
		t.Errorf("expected cap at %d, got %d", maxEntries, len(s.Entries()))
	}
}

// loadFromPath is a test helper that mirrors Open's behavior with a specific path.
func loadFromPath(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	s.entries = f.Entries
	return s, nil
}
