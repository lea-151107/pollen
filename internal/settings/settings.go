// Package settings persists user-toggleable options across sessions.
//
// Stored as JSON at ~/.config/pollen/settings.json. Missing or corrupt files
// are treated as default-valued Settings, not as errors, so a bad disk state
// never blocks startup.
package settings

import (
	"fmt"
	"net/url"
	"os"

	"github.com/lea/pollen/internal/userconfig"
)

const fileName = "settings.json"

type Settings struct {
	SkipTLSVerify      bool    `json:"skip_tls_verify"`
	ResponsePanelRatio float64 `json:"response_panel_ratio,omitempty"`
	RequestTimeoutSecs int     `json:"request_timeout_secs,omitempty"`
	MaxResponseMiB     int     `json:"max_response_mib,omitempty"`
	HistoryLimit       int     `json:"history_limit,omitempty"`
	TextPreviewKiB     int     `json:"text_preview_kib,omitempty"`
	SidebarMaxWidth    int     `json:"sidebar_max_width,omitempty"`
	HexDumpKiB         int     `json:"hex_dump_kib,omitempty"`
	ProxyURL           string  `json:"proxy_url,omitempty"`
	DisableRedirects   bool    `json:"disable_redirects,omitempty"`
	CACertFile         string  `json:"ca_cert_file,omitempty"`
	EnableCookies      bool    `json:"enable_cookies,omitempty"`
}

// Load reads settings from disk. Missing or corrupt files fall back to
// defaults — a bad disk state never blocks startup.
func Load() (*Settings, error) {
	s := &Settings{}
	if _, err := userconfig.LoadJSON(fileName, s); err != nil {
		// Corrupt file: reset to zero so partial unmarshal doesn't leave
		// stray values, then fall through to the normalization below.
		s = &Settings{}
	}
	if s.ResponsePanelRatio <= 0 || s.ResponsePanelRatio >= 1 {
		s.ResponsePanelRatio = 0.5
	}
	if s.RequestTimeoutSecs <= 0 || s.RequestTimeoutSecs > 600 {
		s.RequestTimeoutSecs = 60
	}
	if s.MaxResponseMiB <= 0 || s.MaxResponseMiB > 1024 {
		s.MaxResponseMiB = 32
	}
	if s.HistoryLimit <= 0 || s.HistoryLimit > 10000 {
		s.HistoryLimit = 200
	}
	if s.TextPreviewKiB <= 0 || s.TextPreviewKiB > 10240 {
		s.TextPreviewKiB = 100
	}
	if s.SidebarMaxWidth < 20 || s.SidebarMaxWidth > 200 {
		s.SidebarMaxWidth = 40
	}
	if s.HexDumpKiB <= 0 || s.HexDumpKiB > 1024 {
		s.HexDumpKiB = 4
	}
	if s.ProxyURL != "" {
		if _, err := url.Parse(s.ProxyURL); err != nil {
			s.ProxyURL = ""
		}
	}
	return s, nil
}

// Save writes settings atomically.
func (s *Settings) Save() error {
	return userconfig.SaveJSON(fileName, s)
}

// WriteDefaults creates settings.json populated with all default values.
// Returns the created file path. Errors if the file already exists.
func WriteDefaults() (string, error) {
	path, err := userconfig.Path(fileName)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("settings.json already exists at %s (delete it to reset)", path)
	}
	s, _ := Load() // no file on disk → returns all defaults
	if err := s.Save(); err != nil {
		return "", fmt.Errorf("failed to write settings.json: %w", err)
	}
	return path, nil
}
