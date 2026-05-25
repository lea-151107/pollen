package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lea/pollen/internal/userconfig"
)

const (
	currentVersion = 1
	maxEntries     = 200
)

type Store struct {
	path    string
	entries []Entry
}

// Open loads history from disk. Missing files yield an empty store.
func Open() (*Store, error) {
	path, err := defaultPath()
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
		// Corrupt JSON shouldn't brick startup. Surface a warning to stderr but
		// continue with an empty store; the on-disk file is left untouched so
		// the user can inspect/recover it. The next Save() will rewrite it.
		fmt.Fprintf(os.Stderr, "pollen: history.json is corrupt (%v); starting with empty history\n", err)
		return s, nil
	}
	s.entries = f.Entries
	return s, nil
}

func defaultPath() (string, error) {
	return userconfig.Path("history.json")
}

func (s *Store) Entries() []Entry {
	return s.entries
}

// Prepend adds a new entry at the front and trims to maxEntries.
func (s *Store) Prepend(e Entry) {
	s.entries = append([]Entry{e}, s.entries...)
	if len(s.entries) > maxEntries {
		s.entries = s.entries[:maxEntries]
	}
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

// IndexOf returns the index of the entry with the given ID, or -1 if not found.
func (s *Store) IndexOf(id string) int {
	for i, e := range s.entries {
		if e.ID == id {
			return i
		}
	}
	return -1
}

// InsertAt inserts e at position i. Indices outside [0,len] are clamped so an
// out-of-range insert appends instead of erroring.
func (s *Store) InsertAt(i int, e Entry) {
	if i < 0 {
		i = 0
	}
	if i > len(s.entries) {
		i = len(s.entries)
	}
	s.entries = append(s.entries, Entry{})
	copy(s.entries[i+1:], s.entries[i:])
	s.entries[i] = e
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
	return os.Rename(tmp, s.path)
}
