package userconfig

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// LoadJSON reads <pollen-dir>/<name> and unmarshals it into dst.
// Returns (false, nil) when the file does not exist — callers should treat
// that as "use defaults". A successful read returns (true, nil); corrupt JSON
// or other read errors are surfaced verbatim so callers can decide whether
// to fall back to defaults or fail.
func LoadJSON(name string, dst any) (loaded bool, err error) {
	path, err := Path(name)
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return false, err
	}
	return true, nil
}

// SaveJSON writes src as indented JSON to <pollen-dir>/<name>, atomically
// (via a sibling .tmp file + rename). The parent directory is created if
// missing.
func SaveJSON(name string, src any) error {
	path, err := Path(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
