package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lea-151107/pollen/internal/userconfig"
)

const currentVersion = 1

type Store struct {
	path       string
	entries    []Entry
	maxEntries int
}

// Open loads history from disk. Missing files yield an empty store.
func Open() (*Store, error) {
	path, err := defaultPath()
	if err != nil {
		return nil, err
	}
	s := &Store{path: path, maxEntries: 200}
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

// SetMaxEntries updates the history cap and immediately trims any excess.
func (s *Store) SetMaxEntries(n int) {
	s.maxEntries = n
	cap := s.cap()
	if len(s.entries) > cap {
		s.entries = s.entries[:cap]
	}
}

// cap returns the effective cap, defaulting to 200 for zero-valued stores.
func (s *Store) cap() int {
	if s.maxEntries <= 0 {
		return 200
	}
	return s.maxEntries
}

// keepBodyBytes is the number of most-recent entries whose Response.BodyBytes
// are retained in memory for `s` (save raw bytes). Older entries have their
// BodyBytes dropped to bound session memory use — text bodies can still be
// re-saved from the Body string via Response.CurrentBytes' fallback; binary
// bodies past this window can no longer be saved.
const keepBodyBytes = 10

// Prepend adds a new entry at the front and trims to the effective cap. It
// also drops Response.BodyBytes from entries past keepBodyBytes so that long
// sessions with large responses don't accumulate up to
// max_response_mib × history_limit (≈ 6.4 GiB at defaults) in memory.
func (s *Store) Prepend(e Entry) {
	s.entries = append([]Entry{e}, s.entries...)
	if n := s.cap(); len(s.entries) > n {
		s.entries = s.entries[:n]
	}
	for i := keepBodyBytes; i < len(s.entries); i++ {
		if s.entries[i].Response != nil {
			s.entries[i].Response.BodyBytes = nil
		}
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
	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
