package collections

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/userconfig"
)

const currentVersion = 1

type Store struct {
	path    string
	entries []Entry
}

// Open loads collections from disk. Missing files yield an empty store.
func Open() (*Store, error) {
	path, err := userconfig.Path("collections.json")
	if err != nil {
		return nil, err
	}
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		fmt.Fprintf(os.Stderr, "pollen: collections.json is corrupt (%v); starting with empty collections\n", err)
		return s, nil
	}
	s.entries = f.Entries
	return s, nil
}

func (s *Store) Entries() []Entry { return s.entries }

// Add appends a new entry at the end.
func (s *Store) Add(e Entry) {
	s.entries = append(s.entries, e)
}

// DeleteAt removes the entry at index i. Returns false when the index is out
// of range.
func (s *Store) DeleteAt(i int) bool {
	if i < 0 || i >= len(s.entries) {
		return false
	}
	s.entries = append(s.entries[:i], s.entries[i+1:]...)
	return true
}

// Rename updates the Name of the entry with the given ID. Returns false if not found.
func (s *Store) Rename(id, name string) bool {
	idx := s.IndexOf(id)
	if idx < 0 {
		return false
	}
	s.entries[idx].Name = name
	return true
}

// UpdateRequest replaces the Request of the entry with the given ID.
// Returns false if not found.
func (s *Store) UpdateRequest(id string, req history.Request) bool {
	idx := s.IndexOf(id)
	if idx < 0 {
		return false
	}
	s.entries[idx].Request = req
	return true
}

// IndexOf returns the index of the entry with the given ID, or -1 if not found.
func (s *Store) IndexOf(id string) int {
	for i, e := range s.entries {
		if e.ID == id {
			return i
		}
	}
	return -1
}

// Save writes the current entries to disk atomically.
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f := File{Version: currentVersion, Entries: s.entries}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
