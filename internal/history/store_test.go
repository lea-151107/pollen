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
