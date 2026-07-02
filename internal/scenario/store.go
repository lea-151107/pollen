package scenario

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/lea-151107/pollen/internal/userconfig"
)

const currentVersion = 1

// Store is the on-disk collection of scenarios. It mirrors
// collections.Store: load-once, mutate in memory, Save atomically.
type Store struct {
	path    string
	entries []Scenario
}

// Open loads scenarios from disk. A missing file yields an empty store;
// corrupt JSON is reported to stderr and also yields an empty store so a bad
// file never blocks startup.
func Open() (*Store, error) {
	path, err := userconfig.Path("scenarios.json")
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
		fmt.Fprintf(os.Stderr, "pollen: scenarios.json is corrupt (%v); starting with no scenarios\n", err)
		return s, nil
	}
	s.entries = f.Entries
	return s, nil
}

func (s *Store) Entries() []Scenario { return s.entries }

// Add appends a new scenario at the end.
func (s *Store) Add(sc Scenario) {
	s.entries = append(s.entries, sc)
}

// DeleteAt removes the scenario at index i. Returns false when out of range.
func (s *Store) DeleteAt(i int) bool {
	if i < 0 || i >= len(s.entries) {
		return false
	}
	s.entries = append(s.entries[:i], s.entries[i+1:]...)
	return true
}

// Rename updates the Name of the scenario with the given ID.
func (s *Store) Rename(id, name string) bool {
	idx := s.IndexOf(id)
	if idx < 0 {
		return false
	}
	s.entries[idx].Name = name
	return true
}

// Replace swaps the scenario with the given ID for sc (keeping sc's ID).
// Returns false if not found.
func (s *Store) Replace(id string, sc Scenario) bool {
	idx := s.IndexOf(id)
	if idx < 0 {
		return false
	}
	s.entries[idx] = sc
	return true
}

// IndexOf returns the index of the scenario with the given ID, or -1.
func (s *Store) IndexOf(id string) int {
	for i, e := range s.entries {
		if e.ID == id {
			return i
		}
	}
	return -1
}

// ByName returns the scenario with the given name (case-insensitive) and true,
// or a zero Scenario and false. Used by the --run CLI runner to resolve a
// scenario name to its definition.
func (s *Store) ByName(name string) (Scenario, bool) {
	for _, e := range s.entries {
		if strings.EqualFold(e.Name, name) {
			return e, true
		}
	}
	return Scenario{}, false
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
