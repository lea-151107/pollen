package history

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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
		return nil, err
	}
	s.entries = f.Entries
	return s, nil
}

func defaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pollen", "history.json"), nil
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
