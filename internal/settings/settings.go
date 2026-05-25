// Package settings persists user-toggleable options across sessions.
//
// Stored as JSON at ~/.config/pollen/settings.json. Missing file is treated as
// default-valued Settings, not an error.
package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lea/pollen/internal/userconfig"
)

type Settings struct {
	SkipTLSVerify bool `json:"skip_tls_verify"`
}

// Load reads settings from disk. A missing file yields zero-valued Settings.
func Load() (*Settings, error) {
	path, err := defaultPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Settings{}, nil
		}
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		// Corrupt file shouldn't brick startup; warn via empty settings.
		return &Settings{}, nil
	}
	return &s, nil
}

// Save writes settings atomically.
func (s *Settings) Save() error {
	path, err := defaultPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func defaultPath() (string, error) {
	return userconfig.Path("settings.json")
}
