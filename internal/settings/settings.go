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

	"github.com/lea-151107/pollen/internal/userconfig"
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

	// EnableMouse turns on SGR mouse reporting (click to focus a panel, click a
	// sidebar row to load it, wheel to scroll). Off by default: enabling mouse
	// mode overrides the terminal's own text selection / copy (users then hold
	// Shift to select), which keyboard-driven users often prefer to keep.
	EnableMouse bool `json:"enable_mouse,omitempty"`
	IntruderConcurrency        int `json:"intruder_concurrency,omitempty"`
	IntruderDelayMs            int `json:"intruder_delay_ms,omitempty"`
	IntruderMaxRequests        int `json:"intruder_max_requests,omitempty"`
	IntruderResponseBodyCapKiB int `json:"intruder_response_body_cap_kib,omitempty"`

	// OAuthPersistTokens controls whether successfully-fetched OAuth
	// access/refresh tokens are written to ~/.config/pollen/oauth_tokens.json
	// (mode 0600) and re-loaded on next start. Defaults to true; users who
	// want session-only OAuth set this to false in settings.json.
	// Intentionally NOT tagged omitempty so both true and false are written
	// explicitly — the file ends up self-documenting.
	OAuthPersistTokens bool `json:"oauth_persist_tokens"`
}

// Defaults returns a *Settings populated with the canonical default values,
// matching what Load() yields when no settings.json is present. Used by both
// startup (via Load) and the in-TUI "reset to defaults" action so the two
// stay in lockstep.
func Defaults() *Settings {
	return &Settings{
		OAuthPersistTokens:         true,
		ResponsePanelRatio:         0.5,
		RequestTimeoutSecs:         60,
		MaxResponseMiB:             32,
		HistoryLimit:               200,
		TextPreviewKiB:             100,
		SidebarMaxWidth:            40,
		HexDumpKiB:                 4,
		IntruderConcurrency:        5,
		IntruderDelayMs:            0,
		IntruderMaxRequests:        1000,
		IntruderResponseBodyCapKiB: 64,
	}
}

// Load reads settings from disk. Missing or corrupt files fall back to
// defaults — a bad disk state never blocks startup.
func Load() (*Settings, error) {
	// Fields that default to a non-zero value are pre-initialized here
	// so json.Unmarshal — which only overwrites present fields —
	// preserves the default when the JSON omits the field.
	s := &Settings{OAuthPersistTokens: true}
	if _, err := userconfig.LoadJSON(fileName, s); err != nil {
		// Corrupt file: reset to defaults so a partial unmarshal doesn't
		// leave stray values, then fall through to the normalization below.
		s = &Settings{OAuthPersistTokens: true}
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
			// Surface the typo so the user notices instead of silently
			// having their proxy setting dropped. (The httpx layer also
			// defensively forces direct on parse failure, but that path
			// is unreachable in practice because we reset to "" here.)
			fmt.Fprintf(os.Stderr, "pollen: ignoring invalid proxy_url %q: %v\n", s.ProxyURL, err)
			s.ProxyURL = ""
		}
	}
	if s.IntruderConcurrency < 1 || s.IntruderConcurrency > 256 {
		s.IntruderConcurrency = 5
	}
	if s.IntruderDelayMs < 0 || s.IntruderDelayMs > 60000 {
		s.IntruderDelayMs = 0
	}
	if s.IntruderMaxRequests < 1 || s.IntruderMaxRequests > 1000000 {
		s.IntruderMaxRequests = 1000
	}
	// Per-result body cap for Intruder. Smaller than MaxResponseMiB so a
	// 1000-payload run with 256 KiB responses doesn't pin GiBs of RAM
	// just to support the Enter→Response detail view.
	if s.IntruderResponseBodyCapKiB < 1 || s.IntruderResponseBodyCapKiB > 10240 {
		s.IntruderResponseBodyCapKiB = 64
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
