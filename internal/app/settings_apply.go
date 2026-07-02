package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea-151107/pollen/internal/httpx"
	"github.com/lea-151107/pollen/internal/settings"
	"github.com/lea-151107/pollen/internal/ui"
)

// applySettings dispatches every Settings field to the right
// runtime global / panel state, then persists via s.Save(). Two
// fields are intentionally NOT applied here:
//   - CACertFile: parsed at startup into httpx.CACertPool; live
//     reload requires reading the PEM, building a fresh pool, and
//     swapping it atomically — out of scope for v1.7
//   - EnableCookies: cookie jar is created once at startup; toggling
//     mid-session would silently lose accumulated cookies
//
// Both are still written to settings.json so the next pollen start
// picks them up. The Settings overlay shows a "restart" badge for
// these to set the user's expectation.
//
// Returns a tea.Cmd (possibly nil): toggling EnableMouse switches SGR mouse
// reporting live via tea.EnableMouseCellMotion / tea.DisableMouse so it takes
// effect without a restart.
func (m *Model) applySettings(s *settings.Settings) tea.Cmd {
	// Start from the current snapshot so startup-only fields (CACertPool,
	// CookieJar) survive a live settings reapply, then atomically swap.
	c := httpx.Snapshot()
	c.SkipTLSVerify = s.SkipTLSVerify
	c.RequestTimeout = time.Duration(s.RequestTimeoutSecs) * time.Second
	c.MaxResponseBytes = s.MaxResponseMiB * 1024 * 1024
	c.ProxyURL = s.ProxyURL
	c.DisableRedirects = s.DisableRedirects
	httpx.SetConfig(c)

	m.store.SetMaxEntries(s.HistoryLimit)
	ui.TextPreviewLimit = s.TextPreviewKiB * 1024
	ui.DefaultHexDumpLimit = s.HexDumpKiB * 1024
	m.sidebarMaxWidth = s.SidebarMaxWidth
	m.responsePanelRatio = s.ResponsePanelRatio

	m.intruder.SetDefaultConcurrency(s.IntruderConcurrency)
	m.intruder.SetDefaultDelayMs(s.IntruderDelayMs)
	m.intruder.SetDefaultMaxRequests(s.IntruderMaxRequests)
	m.intruderBodyCapBytes = s.IntruderResponseBodyCapKiB * 1024

	m.persistTokens = s.OAuthPersistTokens

	// View-visible TLS skip mirror.
	m.tlsInsecure = s.SkipTLSVerify

	// Live mouse toggle: only emit a command when the state actually changes.
	var cmd tea.Cmd
	if s.EnableMouse != m.mouseEnabled {
		m.mouseEnabled = s.EnableMouse
		if s.EnableMouse {
			cmd = tea.EnableMouseCellMotion
		} else {
			cmd = tea.DisableMouse
		}
	}

	_ = s.Save()
	return cmd
}
