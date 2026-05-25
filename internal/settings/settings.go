// Package settings persists user-toggleable options across sessions.
//
// Stored as JSON at ~/.config/pollen/settings.json. Missing or corrupt files
// are treated as default-valued Settings, not as errors, so a bad disk state
// never blocks startup.
package settings

import (
	"github.com/lea/pollen/internal/userconfig"
)

const fileName = "settings.json"

type Settings struct {
	SkipTLSVerify bool `json:"skip_tls_verify"`
}

// Load reads settings from disk. A missing or corrupt file yields a
// zero-valued Settings.
func Load() (*Settings, error) {
	s := &Settings{}
	if _, err := userconfig.LoadJSON(fileName, s); err != nil {
		// Corrupt file shouldn't brick startup; fall back to defaults.
		return &Settings{}, nil
	}
	return s, nil
}

// Save writes settings atomically.
func (s *Settings) Save() error {
	return userconfig.SaveJSON(fileName, s)
}
