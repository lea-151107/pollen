package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/lea-151107/pollen/internal/collections"
	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/httpx"
	"github.com/lea-151107/pollen/internal/importer"
	intruderpkg "github.com/lea-151107/pollen/internal/intruder"
	"github.com/lea-151107/pollen/internal/oauth"
	"github.com/lea-151107/pollen/internal/settings"
	"github.com/lea-151107/pollen/internal/ui"
)

// clearStatusMsg fires when a transient toast should be cleared. The gen
// field is checked against Model.statusGen so a stale tick (scheduled before
// a newer setStatus) doesn't wipe the newer message.
type clearStatusMsg struct{ gen int }

// isTextEditingFocus reports whether the currently focused panel is actively
// accepting character input. Used to gate single-letter global shortcuts
// (currently `u` for undo) so they don't swallow real input.
func isTextEditingFocus(f focusArea, bodyInEditor, historyFilterMode, collFilterMode, responseInputActive bool) bool {
	switch f {
	case focusURL, focusQuery, focusAuth, focusHeaders:
		return true
	case focusBody:
		return bodyInEditor
	case focusHistory:
		return historyFilterMode
	case focusCollections:
		return collFilterMode
	case focusResponse:
		return responseInputActive
	}
	return false
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.intruder.SetSize(msg.Width, msg.Height)
		m.help.SetSize(msg.Width, msg.Height)
		m.settingsPanel.SetSize(msg.Width, msg.Height)
		return m, nil

	case sendResultMsg:
		// Discard out-of-order results from older Send presses; only the
		// latest in-flight request's response should land in the UI.
		if msg.gen != m.requestGen {
			return m, nil
		}
		m.response.SetLoading(false)
		if msg.entry.Error != "" {
			m.response.SetError(msg.entry.Error)
		} else if msg.entry.Response != nil {
			m.response.SetResponse(msg.entry.Response, msg.entry.Request.URL)
		}
		m.store.Prepend(msg.entry)
		_ = m.store.Save()
		// Prepend shifts every existing entry by 1 — slide the cursor too so
		// it keeps pointing at the same entry the user was looking at.
		m.history.Shift(1)
		m.history.SetEntries(m.store.Entries())
		// Any history mutation invalidates a pending undo (indices have shifted).
		m.pendingUndo = nil
		return m, nil

	case ui.HistorySelectMsg:
		m.applyEntry(msg.Entry)
		m.focus = focusURL
		m.applyFocus()
		return m, nil

	case ui.CollectionSelectMsg:
		m.applyEntry(history.Entry{Request: msg.Entry.Request})
		m.lastLoadedCollID = msg.Entry.ID
		m.focus = focusURL
		m.applyFocus()
		return m, nil

	case ui.CollectionRenameMsg:
		idx := m.collStore.IndexOf(msg.ID)
		if idx < 0 {
			return m, nil
		}
		m.renameTargetID = msg.ID
		m.renameInput.SetValue(m.collStore.Entries()[idx].Name)
		m.renamingColl = true
		return m, m.renameInput.Focus()

	case ui.CollectionDeleteMsg:
		idx := m.collStore.IndexOf(msg.ID)
		if idx < 0 {
			return m, nil
		}
		if !m.collStore.DeleteAt(idx) {
			return m, nil
		}
		_ = m.collStore.Save()
		m.collUI.SetEntries(m.collStore.Entries())
		m.setStatus(statusOK, "collection entry deleted")
		return m, m.statusTick(2 * time.Second)

	case ui.ResponseCopyMsg:
		m.deliverCopy(msg.Text, "response body")
		return m, m.statusTick(2 * time.Second)

	case ui.HistoryDeleteMsg:
		// Look up by ID so the operation works regardless of any active
		// history filter (the filter shifts UI indices but not store indices).
		idx := m.store.IndexOf(msg.ID)
		if idx < 0 {
			return m, nil
		}
		snapshot := m.store.Entries()[idx]
		if !m.store.DeleteAt(idx) {
			return m, nil
		}
		_ = m.store.Save()
		m.history.SetEntries(m.store.Entries())
		m.setStatus(statusOK, "deleted (u to undo)")
		m.pendingUndo = &pendingUndo{entry: snapshot, index: idx, gen: m.statusGen}
		return m, m.statusTick(5 * time.Second)

	case clearStatusMsg:
		// Ignore stale Ticks scheduled for an earlier message.
		if msg.gen == m.statusGen {
			m.statusMsg = ""
			m.statusKind = statusOK
			// Undo window expires together with the toast that announced it.
			m.pendingUndo = nil
		}
		return m, nil

	case ui.IntruderResultMsg:
		// Workers stream results in via this msg; only retain them while
		// the overlay is still open so an aborted run isn't accumulating
		// memory after the user dismisses it.
		if m.intruder.State() == ui.IntruderResults {
			m.intruder.AppendResult(msg.Result)
		}
		// Schedule the next read; the channel close becomes IntruderDoneMsg.
		if m.intruderCh != nil {
			return m, nextIntruderResultCmd(m.intruderCh)
		}
		return m, nil

	case ui.AuthOAuthTokenMsg:
		m.auth.SetOAuthToken(msg.Token)
		m.persistOAuthToken(oauth.GrantClientCredentials)
		m.setStatus(statusOK, "OAuth token acquired")
		return m, m.statusTick(2 * time.Second)

	case ui.AuthOAuthErrorMsg:
		m.auth.SetOAuthError(msg.Err)
		m.setStatus(statusError, "OAuth: "+msg.Err)
		return m, m.statusTick(4 * time.Second)

	case ui.AuthOAuthACTokenMsg:
		m.auth.SetOAuthACToken(msg.Token)
		m.persistOAuthToken(oauth.GrantAuthorizationCode)
		m.setStatus(statusOK, "OAuth AC token acquired")
		return m, m.statusTick(2 * time.Second)

	case ui.AuthOAuthACErrorMsg:
		m.auth.SetOAuthACError(msg.Err)
		m.setStatus(statusError, "OAuth AC: "+msg.Err)
		return m, m.statusTick(4 * time.Second)

	case ui.AuthOAuthDCAuthorizeMsg:
		// Device authorization step finished; display the user_code
		// and chain into the polling loop. Authorize msg carries
		// ctx + cfg so the poll Cmd reuses both.
		m.auth.SetOAuthDCAuth(msg.Auth)
		return m, ui.OAuthDCPollCmd(msg.Ctx, msg.Cfg, msg.Auth.DeviceCode, msg.Auth.Interval)

	case ui.AuthOAuthDCTokenMsg:
		m.auth.SetOAuthDCToken(msg.Token)
		m.persistOAuthToken(oauth.GrantDeviceCode)
		m.setStatus(statusOK, "OAuth DC token acquired")
		return m, m.statusTick(2 * time.Second)

	case ui.AuthOAuthDCErrorMsg:
		m.auth.SetOAuthDCError(msg.Err)
		m.setStatus(statusError, "OAuth DC: "+msg.Err)
		return m, m.statusTick(4 * time.Second)

	case ui.AuthForgetTokenMsg:
		if m.tokenStore != nil {
			if m.tokenStore.Forget(msg.TokenURL, msg.ClientID, msg.Grant) {
				_ = m.tokenStore.Save()
			}
		}
		m.setStatus(statusOK, "OAuth token forgotten")
		return m, m.statusTick(2 * time.Second)

	case authRefreshedSendMsg:
		m.auth.ApplyRefreshedToken(msg.Token)
		// Persist the freshly-refreshed token so the next pollen
		// start picks up the new ExpiresAt instead of refreshing
		// from a stale on-disk entry. Routes to the CC or AC slot
		// based on the active auth type.
		switch m.auth.Type() {
		case ui.AuthOAuth:
			m.persistOAuthToken(oauth.GrantClientCredentials)
		case ui.AuthOAuthAC:
			m.persistOAuthToken(oauth.GrantAuthorizationCode)
		case ui.AuthOAuthDC:
			m.persistOAuthToken(oauth.GrantDeviceCode)
		}
		// Replace the "refreshing OAuth token…" toast set in the Send
		// handler with a positive confirmation. Without this, the
		// "refreshing…" string would persist on screen past the
		// completed refresh + send because sendResultMsg doesn't touch
		// status. tea.Batch lets the actual send and the status tick
		// fire independently.
		m.setStatus(statusOK, "OAuth token refreshed")
		return m, tea.Batch(m.sendRequest(), m.statusTick(2*time.Second))

	case ui.SettingsAppliedMsg:
		m.applySettings(msg.Setting)
		return m, nil

	case ui.HelpOpenSettingsMsg:
		// "Open Settings" button in the Ctrl+/ help overlay.
		// Help.Update already cleared its open flag before emitting
		// this msg, so we only need to open Settings here.
		s, _ := settings.Load()
		m.settingsPanel.Open(s)
		return m, nil

	case ui.HelpResetSettingsMsg:
		// "Reset settings to defaults" button in the help overlay,
		// after y confirmation. applySettings handles both runtime
		// dispatch and disk persistence (s.Save). Re-seed the
		// Settings panel if it happens to be open so the user sees
		// the new values.
		def := settings.Defaults()
		m.applySettings(def)
		if m.settingsPanel.IsOpen() {
			m.settingsPanel.Open(def)
		}
		m.setStatus(statusOK, "settings reset to defaults")
		return m, m.statusTick(2 * time.Second)

	case authRefreshFailedMsg:
		m.setStatus(statusError, "refresh failed: "+msg.Err+"  · press g on Auth panel to re-authorize")
		return m, m.statusTick(5 * time.Second)

	case ui.IntruderDoneMsg:
		m.intruder.MarkDone("")
		m.intruderCh = nil
		// Persist the latest run so the --export-intruder CLI can read
		// the same data. Failures are silent; the TUI keeps its
		// in-memory copy regardless.
		_ = intruderpkg.SaveLastRun(m.intruder.Results())
		return m, nil

	case ui.IntruderExportMsg:
		// In-app CSV export: render results via the same exporter the
		// --export-intruder CLI uses, write to the path the user typed.
		results := m.intruder.Results()
		data, err := intruderpkg.CSV(results)
		if err != nil {
			m.setStatus(statusError, fmt.Sprintf("intruder export: %v", err))
			return m, m.statusTick(4 * time.Second)
		}
		path := expandHome(msg.Path)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			m.setStatus(statusError, fmt.Sprintf("intruder export: %v", err))
			return m, m.statusTick(4 * time.Second)
		}
		m.setStatus(statusOK, fmt.Sprintf("exported %d rows to %s", len(results), path))
		return m, m.statusTick(3 * time.Second)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Rename-collection dialog.
	if m.renamingColl {
		switch km.String() {
		case "esc":
			m.renamingColl = false
			m.renameInput.SetValue("")
			m.renameInput.Blur()
			m.renameTargetID = ""
		case "enter":
			name := m.renameInput.Value()
			if name == "" {
				name = "Untitled"
			}
			if m.collStore.Rename(m.renameTargetID, name) {
				_ = m.collStore.Save()
				m.collUI.SetEntries(m.collStore.Entries())
				m.setStatus(statusOK, fmt.Sprintf("renamed to: %s", name))
			}
			m.renamingColl = false
			m.renameInput.SetValue("")
			m.renameInput.Blur()
			m.renameTargetID = ""
			return m, m.statusTick(2 * time.Second)
		default:
			var cmd tea.Cmd
			m.renameInput, cmd = m.renameInput.Update(km)
			return m, cmd
		}
		return m, nil
	}

	// Update-collection prompt (shown when Ctrl+B is pressed after loading a
	// collection entry).
	if m.collUpdatePromptOpen {
		switch km.String() {
		case "enter":
			req := m.currentRequest()
			if m.collStore.UpdateRequest(m.collUpdateTargetID, req) {
				_ = m.collStore.Save()
				m.collUI.SetEntries(m.collStore.Entries())
				m.setStatus(statusOK, fmt.Sprintf("updated: %s", m.collUpdateTargetName))
				m.lastLoadedCollID = ""
			} else {
				m.setStatus(statusError, "collection entry not found")
			}
			m.collUpdatePromptOpen = false
			return m, m.statusTick(2 * time.Second)
		case "n":
			m.collUpdatePromptOpen = false
			m.savingToCollection = true
			m.saveCollInput.SetValue("")
			return m, m.saveCollInput.Focus()
		case "esc":
			m.collUpdatePromptOpen = false
		}
		return m, nil
	}

	// Import-file dialog intercepts all input before other overlays.
	if m.importingFile {
		switch km.String() {
		case "esc":
			m.importingFile = false
			m.importInput.SetValue("")
			m.importInput.Blur()
		case "enter":
			path := expandHome(m.importInput.Value())
			m.importingFile = false
			m.importInput.SetValue("")
			m.importInput.Blur()
			entries, err := importer.Import(path)
			if err != nil {
				m.setStatus(statusError, fmt.Sprintf("import failed: %v", err))
				return m, m.statusTick(4 * time.Second)
			}
			for _, e := range entries {
				m.collStore.Add(e)
			}
			_ = m.collStore.Save()
			m.collUI.SetEntries(m.collStore.Entries())
			m.setStatus(statusOK, fmt.Sprintf("imported %d entries", len(entries)))
			return m, m.statusTick(3 * time.Second)
		default:
			var cmd tea.Cmd
			m.importInput, cmd = m.importInput.Update(km)
			return m, cmd
		}
		return m, nil
	}

	// Intruder overlay takes precedence over any other panel input so the
	// modal feels modal. Enter in the config state starts the run; Esc
	// inside the results table cancels and closes.
	if m.intruder.State() != ui.IntruderHidden {
		// Enter in the config form triggers a run; let updateConfig fall
		// through for every other key (text input + Tab navigation).
		if m.intruder.State() == ui.IntruderConfig && km.String() == "enter" {
			return m, m.startIntruderRun()
		}
		var cmd tea.Cmd
		m.intruder, cmd = m.intruder.Update(km)
		return m, cmd
	}

	// Save-to-collection dialog intercepts all input first.
	if m.savingToCollection {
		switch km.String() {
		case "esc":
			m.savingToCollection = false
			m.saveCollInput.SetValue("")
			m.saveCollInput.Blur()
		case "enter":
			name := m.saveCollInput.Value()
			if name == "" {
				name = "Untitled"
			}
			req := m.currentRequest()
			m.collStore.Add(collections.Entry{
				ID:      uuid.NewString(),
				Name:    name,
				Request: req,
			})
			_ = m.collStore.Save()
			m.collUI.SetEntries(m.collStore.Entries())
			m.savingToCollection = false
			m.saveCollInput.SetValue("")
			m.saveCollInput.Blur()
			m.lastLoadedCollID = ""
			m.setStatus(statusOK, fmt.Sprintf("saved to collection: %s", name))
			return m, m.statusTick(2 * time.Second)
		default:
			var cmd tea.Cmd
			m.saveCollInput, cmd = m.saveCollInput.Update(km)
			return m, cmd
		}
		return m, nil
	}

	// Global keys take precedence except where noted.
	switch {
	case key.Matches(km, m.keys.Quit):
		return m, tea.Quit

	case km.String() == "ctrl+l":
		return m, tea.ClearScreen

	case m.copyMenuOpen:
		return m.handleCopyMenu(km)

	case m.help.IsOpen():
		// Ctrl+/ closes the overlay regardless of the panel state so
		// the toggle stays symmetric. Other keys (↑/↓ j/k, Enter, g/G,
		// PgUp/PgDn, Esc, q) are handled inside the Help component.
		if key.Matches(km, m.keys.Help) {
			m.help.Close()
			return m, nil
		}
		var cmd tea.Cmd
		m.help, cmd = m.help.Update(km)
		return m, cmd

	case m.settingsPanel.IsOpen():
		// Ctrl+P / Ctrl+, toggles the overlay closed — but only from
		// navigation mode. While a field editor is active the panel
		// deliberately blocks accidental close (q types into the field,
		// Esc only exits the editor), so the toggle must defer to the
		// editor too; otherwise an in-progress edit is silently lost.
		// Everything else delegates to the SettingsPanel's own Update.
		if key.Matches(km, m.keys.Settings) && !m.settingsPanel.IsEditing() {
			m.settingsPanel.Close()
			return m, nil
		}
		var cmd tea.Cmd
		m.settingsPanel, cmd = m.settingsPanel.Update(km)
		return m, cmd

	case m.envSwitcherOpen:
		return m.handleEnvSwitcher(km)

	case key.Matches(km, m.keys.Help):
		// Ctrl+/ is a non-printing key, so it doesn't conflict with text
		// input — no isTextEditingFocus guard needed.
		m.help.Open(m.keys.HelpSections())
		return m, nil

	case key.Matches(km, m.keys.Settings) && !isTextEditingFocus(m.focus, m.body.InEditorMode(), m.history.InFilterMode(), m.collUI.InFilterMode(), m.response.FilterActive() || m.response.SearchActive()):
		// Open the Settings overlay with the current on-disk state.
		// Live edits propagate via SettingsAppliedMsg → applySettings.
		// The isTextEditingFocus guard keeps Ctrl+P out of the global
		// path while the user is typing in a textinput / textarea so
		// bubbles' default Ctrl+P bindings (textarea LinePrevious,
		// textinput PrevSuggestion) still work. Settings can be
		// opened with Ctrl+P from any non-editing focus or via the
		// CSI-u-only Ctrl+, alias.
		s, _ := settings.Load()
		m.settingsPanel.Open(s)
		return m, nil

	case km.String() == "u" && m.pendingUndo != nil && !isTextEditingFocus(m.focus, m.body.InEditorMode(), m.history.InFilterMode(), m.collUI.InFilterMode(), m.response.FilterActive() || m.response.SearchActive()):
		u := m.pendingUndo
		m.store.InsertAt(u.index, u.entry)
		_ = m.store.Save()
		m.history.SetEntries(m.store.Entries())
		m.pendingUndo = nil
		m.setStatus(statusOK, "restored")
		return m, m.statusTick(2 * time.Second)

	case key.Matches(km, m.keys.Send):
		// When the active auth is OAuth (CC or AC), the current token
		// is near expiry, and a refresh_token is available, run the
		// refresh first and chain to send via authRefreshedSendMsg.
		// Skipped silently for all other auth types.
		if cfg, ok := needsRefresh(m.auth); ok {
			m.setStatus(statusOK, "refreshing OAuth token…")
			return m, refreshThenSendCmd(cfg)
		}
		// Split into two statements: `return m, m.sendRequest()` would rely on
		// undefined evaluation order between `m` and the pointer-receiver call
		// that mutates m. Current gc happens to evaluate the call first, but
		// the Go spec leaves this unspecified.
		cmd := m.sendRequest()
		return m, cmd

	case key.Matches(km, m.keys.Copy):
		m.copyMenuOpen = true
		return m, nil

	case key.Matches(km, m.keys.ToggleHist):
		m.showHistory = !m.showHistory
		if m.showHistory {
			m.showCollections = false
		}
		// Reset focus if the focused panel just became hidden.
		if !m.showHistory && m.focus == focusHistory {
			m.focus = focusURL
			m.applyFocus()
		} else if m.showHistory && m.focus == focusCollections {
			m.focus = focusURL
			m.applyFocus()
		}
		return m, nil

	case key.Matches(km, m.keys.ToggleColl):
		m.showCollections = !m.showCollections
		if m.showCollections {
			m.showHistory = false
		}
		// Reset focus if the focused panel just became hidden.
		if !m.showCollections && m.focus == focusCollections {
			m.focus = focusURL
			m.applyFocus()
		} else if m.showCollections && m.focus == focusHistory {
			m.focus = focusURL
			m.applyFocus()
		}
		return m, nil

	case key.Matches(km, m.keys.SaveToColl):
		if m.lastLoadedCollID != "" {
			idx := m.collStore.IndexOf(m.lastLoadedCollID)
			if idx >= 0 {
				m.collUpdateTargetID = m.lastLoadedCollID
				m.collUpdateTargetName = m.collStore.Entries()[idx].Name
				m.collUpdatePromptOpen = true
				return m, nil
			}
		}
		m.savingToCollection = true
		return m, m.saveCollInput.Focus()

	case key.Matches(km, m.keys.ImportFile):
		m.importingFile = true
		return m, m.importInput.Focus()

	case key.Matches(km, m.keys.Intruder):
		m.intruder.SetSize(m.width, m.height)
		m.intruder.OpenConfig()
		return m, nil

	case key.Matches(km, m.keys.SwitchEnv):
		names := m.env.Names()
		if len(names) == 0 {
			m.setStatus(statusWarn, "no environments defined in env.json")
			return m, m.statusTick(2 * time.Second)
		}
		m.envSwitcherOpen = true
		// Start cursor at current selection if it exists.
		m.envSwitcherCursor = 0
		for i, n := range names {
			if n == m.env.Current {
				m.envSwitcherCursor = i
				break
			}
		}
		return m, nil

	case key.Matches(km, m.keys.ToggleTLS):
		newVal := !httpx.SkipTLSVerify.Load()
		httpx.SkipTLSVerify.Store(newVal)
		m.tlsInsecure = newVal
		// Persist; surface persistence failures so the user knows the
		// preference won't survive a restart.
		s, _ := settings.Load()
		s.SkipTLSVerify = newVal
		saveErr := s.Save()
		switch {
		case saveErr != nil:
			m.setStatus(statusWarn, fmt.Sprintf("TLS verification: toggled (settings save failed: %v)", saveErr))
		case newVal:
			m.setStatus(statusWarn, "TLS verification: OFF (insecure)")
		default:
			m.setStatus(statusOK, "TLS verification: ON")
		}
		return m, m.statusTick(2 * time.Second)

	case m.focus == focusResponse && m.response.FilterActive() && km.String() == "esc":
		// Let response handle Esc to close the filter bar rather than quitting.
		var cmd tea.Cmd
		m.response, cmd = m.response.Update(km)
		return m, cmd

	case m.focus == focusResponse && km.String() == "s" && !m.response.FilterActive() && !m.response.SearchActive():
		// See note above on sendRequest — avoid relying on undefined eval order.
		cmd := m.saveResponse()
		return m, cmd

	case key.Matches(km, m.keys.NextFocus):
		// Headers consumes Tab when it has an active suggestion (autocomplete).
		if m.focus == focusHeaders && m.headers.HasSuggestion() {
			var cmd tea.Cmd
			m.headers, cmd = m.headers.Update(km)
			return m, cmd
		}
		// Body editor consumes Tab while typing to insert indentation.
		if m.focus == focusBody && m.body.InEditorMode() {
			var cmd tea.Cmd
			m.body, cmd = m.body.Update(km)
			return m, cmd
		}
		m.cycleFocus(true)
		return m, nil

	case key.Matches(km, m.keys.PrevFocus):
		// Body editor consumes Shift+Tab symmetric with Tab — Tab inserts an
		// indent, Shift+Tab is currently a no-op (reserved for a future
		// un-indent). Either way, the user stays in the editor.
		if m.focus == focusBody && m.body.InEditorMode() {
			return m, nil
		}
		m.cycleFocus(false)
		return m, nil
	}

	// Delegate to focused component.
	var cmd tea.Cmd
	switch m.focus {
	case focusHistory:
		m.history, cmd = m.history.Update(km)
	case focusCollections:
		m.collUI, cmd = m.collUI.Update(km)
	case focusMethod:
		m.method, cmd = m.method.Update(km)
	case focusURL:
		m.urlBar, cmd = m.urlBar.Update(km)
	case focusQuery:
		m.query, cmd = m.query.Update(km)
	case focusAuth:
		m.auth, cmd = m.auth.Update(km)
	case focusHeaders:
		m.headers, cmd = m.headers.Update(km)
	case focusBody:
		m.body, cmd = m.body.Update(km)
	case focusResponse:
		m.response, cmd = m.response.Update(km)
	}
	return m, cmd
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
