// Package ui — SettingsPanel renders a modal overlay that lets the
// user edit every settings.json key without leaving pollen. Each
// commit (bool toggle or Enter on a textinput) emits
// SettingsAppliedMsg so the app layer can dispatch the change to
// the appropriate runtime global and re-save the file to disk.
package ui

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lea-151107/pollen/internal/settings"
)

// SettingsAppliedMsg signals that one field of the draft Settings
// was committed (validated + applied to the draft). The app layer
// runs applySettings(msg.Setting) to update runtime globals and
// calls Save() to persist.
type SettingsAppliedMsg struct {
	Setting *settings.Settings
}

type settingKind int

const (
	settingBool settingKind = iota
	settingInt
	settingFloat
	settingString
)

// settingField is the per-row metadata that drives both rendering
// and editing. The Set callback returns a non-nil error to keep
// the editor open with the displayed validation message.
type settingField struct {
	label       string
	kind        settingKind
	restartNote bool
	display     func(*settings.Settings) string
	rawValue    func(*settings.Settings) string
	set         func(s *settings.Settings, raw string) error
	rangeHint   string
}

// SettingsPanel is the modal overlay that drives the editing flow.
type SettingsPanel struct {
	fields  []settingField
	cursor  int
	editing bool
	editor  textinput.Model
	editErr string
	draft   *settings.Settings
	width   int
	height  int
	open    bool
}

// NewSettingsPanel returns a closed SettingsPanel with the canonical
// field set. Call Open(settings.Settings) before showing it so the
// draft is seeded with the user's current configuration.
func NewSettingsPanel() SettingsPanel {
	ed := textinput.New()
	ed.CharLimit = 256
	ed.Width = 40
	return SettingsPanel{fields: builtinSettingsFields(), editor: ed}
}

// Open seeds the draft from s (deep copy via value semantics) and
// marks the panel open at cursor 0.
func (p *SettingsPanel) Open(s *settings.Settings) {
	if s == nil {
		p.draft = &settings.Settings{}
	} else {
		cp := *s
		p.draft = &cp
	}
	p.cursor = 0
	p.editing = false
	p.editErr = ""
	p.open = true
}

// Close hides the panel and discards any in-progress editor state.
func (p *SettingsPanel) Close() {
	p.open = false
	p.editing = false
	p.editErr = ""
	p.editor.Blur()
}

// IsOpen reports whether the panel is currently visible.
func (p SettingsPanel) IsOpen() bool { return p.open }

// IsEditing reports whether a field editor is currently active, so the
// app can keep the Settings toggle key (Ctrl+P / Ctrl+,) from closing the
// panel mid-edit. The panel deliberately blocks accidental close while
// editing — Esc only exits the editor, q types into the field — and this
// accessor lets the app routing honor that same protection.
func (p SettingsPanel) IsEditing() bool { return p.editing }

// SetSize records the parent terminal dimensions so View can clip
// the centered modal correctly.
func (p *SettingsPanel) SetSize(w, h int) {
	p.width = w
	p.height = h
}

// Update handles navigation, edit-mode entry/exit, validation, and
// commit. Returns SettingsAppliedMsg via the returned Cmd when a
// field is successfully committed.
func (p SettingsPanel) Update(msg tea.Msg) (SettingsPanel, tea.Cmd) {
	if !p.open {
		return p, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		// Forward non-key messages to the editor when active so the
		// textinput can react to blink/paste/etc.
		if p.editing {
			var cmd tea.Cmd
			p.editor, cmd = p.editor.Update(msg)
			return p, cmd
		}
		return p, nil
	}
	if len(p.fields) == 0 {
		return p, nil
	}
	if p.editing {
		return p.updateEditing(km)
	}
	return p.updateNavigate(km)
}

func (p SettingsPanel) updateNavigate(km tea.KeyMsg) (SettingsPanel, tea.Cmd) {
	switch km.String() {
	case "esc", "q":
		p.Close()
		return p, nil
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
		return p, nil
	case "down", "j":
		if p.cursor < len(p.fields)-1 {
			p.cursor++
		}
		return p, nil
	case "g", "home":
		p.cursor = 0
		return p, nil
	case "G", "end":
		p.cursor = len(p.fields) - 1
		return p, nil
	case "pgup":
		p.cursor -= 5
		if p.cursor < 0 {
			p.cursor = 0
		}
		return p, nil
	case "pgdown":
		p.cursor += 5
		if p.cursor > len(p.fields)-1 {
			p.cursor = len(p.fields) - 1
		}
		return p, nil
	case "enter", " ":
		f := p.fields[p.cursor]
		if f.kind == settingBool {
			// Bool toggles inline — no edit mode.
			cur := f.rawValue(p.draft)
			next := "true"
			if cur == "true" {
				next = "false"
			}
			if err := f.set(p.draft, next); err != nil {
				p.editErr = err.Error()
				return p, nil
			}
			return p, p.emitApplied()
		}
		// Enter edit mode for int / float / string.
		p.editing = true
		p.editErr = ""
		p.editor.SetValue(f.rawValue(p.draft))
		p.editor.CursorEnd()
		p.editor.Focus()
		return p, nil
	}
	return p, nil
}

func (p SettingsPanel) updateEditing(km tea.KeyMsg) (SettingsPanel, tea.Cmd) {
	switch km.String() {
	case "esc":
		p.editing = false
		p.editErr = ""
		p.editor.Blur()
		return p, nil
	case "enter":
		f := p.fields[p.cursor]
		if err := f.set(p.draft, p.editor.Value()); err != nil {
			p.editErr = err.Error()
			return p, nil
		}
		p.editing = false
		p.editErr = ""
		p.editor.Blur()
		return p, p.emitApplied()
	}
	var cmd tea.Cmd
	p.editor, cmd = p.editor.Update(km)
	return p, cmd
}

// emitApplied returns a Cmd that publishes the current draft via a
// SettingsAppliedMsg. The draft is copied first so subsequent
// edits in the panel don't observably mutate the message payload.
func (p SettingsPanel) emitApplied() tea.Cmd {
	cp := *p.draft
	return func() tea.Msg { return SettingsAppliedMsg{Setting: &cp} }
}

// View renders the centered overlay box.
func (p SettingsPanel) View() string {
	if !p.open {
		return ""
	}
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	focus := lipgloss.NewStyle().Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0"))
	restartBadge := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	var lines []string
	for i, f := range p.fields {
		marker := "  "
		if i == p.cursor {
			marker = "▶ "
		}
		label := padRightRune(f.label, 28)
		var valueCol string
		if p.editing && i == p.cursor && f.kind != settingBool {
			valueCol = p.editor.View()
			if f.rangeHint != "" {
				valueCol += "  " + dim.Render(f.rangeHint)
			}
		} else {
			valueCol = f.display(p.draft)
		}
		row := marker + label + valueCol
		if f.restartNote {
			row += "    " + restartBadge.Render("restart")
		}
		if i == p.cursor && !p.editing {
			row = focus.Render(row)
		}
		lines = append(lines, row)
		if p.editing && i == p.cursor && p.editErr != "" {
			lines = append(lines, "    "+errStyle.Render("error: "+p.editErr))
		}
	}

	var sb strings.Builder
	sb.WriteString("Settings")
	sb.WriteString("\n\n")
	sb.WriteString(strings.Join(lines, "\n"))
	sb.WriteString("\n\n")
	if p.editing {
		sb.WriteString(dim.Render("Enter: commit  ·  Esc: cancel edit"))
	} else {
		sb.WriteString(dim.Render("↑/↓ j/k: navigate  ·  Enter/Space: edit/toggle  ·  g/G: first/last  ·  Esc / q: close"))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render(sb.String())
	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, box)
}

// builtinSettingsFields is the canonical 17-row spec. Ranges and
// labels mirror what settings.Load() clamps to — see the
// TestSettings_RangesMatchLoad in settings_apply_test.go for the
// drift check.
func builtinSettingsFields() []settingField {
	intField := func(label string, min, max int, get func(*settings.Settings) int, set func(*settings.Settings, int), unit string, restart bool) settingField {
		return settingField{
			label: label,
			kind:  settingInt,
			restartNote: restart,
			display: func(s *settings.Settings) string {
				return fmt.Sprintf("%d %s", get(s), unit)
			},
			rawValue: func(s *settings.Settings) string {
				return strconv.Itoa(get(s))
			},
			set: func(s *settings.Settings, raw string) error {
				raw = strings.TrimSpace(raw)
				n, err := strconv.Atoi(raw)
				if err != nil {
					return fmt.Errorf("not a whole number")
				}
				if n < min || n > max {
					return fmt.Errorf("out of range %d–%d", min, max)
				}
				set(s, n)
				return nil
			},
			rangeHint: fmt.Sprintf("(%d–%d)", min, max),
		}
	}
	boolField := func(label string, suffix string, get func(*settings.Settings) bool, set func(*settings.Settings, bool), restart bool) settingField {
		return settingField{
			label: label,
			kind:  settingBool,
			restartNote: restart,
			display: func(s *settings.Settings) string {
				if get(s) {
					if suffix != "" {
						return "[✓] " + suffix
					}
					return "[✓]"
				}
				return "[ ]"
			},
			rawValue: func(s *settings.Settings) string {
				if get(s) {
					return "true"
				}
				return "false"
			},
			set: func(s *settings.Settings, raw string) error {
				set(s, raw == "true")
				return nil
			},
		}
	}
	stringField := func(label string, placeholder string, get func(*settings.Settings) string, set func(*settings.Settings, string) error, restart bool) settingField {
		return settingField{
			label: label,
			kind:  settingString,
			restartNote: restart,
			display: func(s *settings.Settings) string {
				v := get(s)
				if v == "" {
					return "(none)"
				}
				return v
			},
			rawValue: func(s *settings.Settings) string {
				return get(s)
			},
			set: func(s *settings.Settings, raw string) error {
				return set(s, strings.TrimSpace(raw))
			},
			rangeHint: placeholder,
		}
	}
	floatField := func(label string, min, max float64, get func(*settings.Settings) float64, set func(*settings.Settings, float64), restart bool) settingField {
		return settingField{
			label: label,
			kind:  settingFloat,
			restartNote: restart,
			display: func(s *settings.Settings) string {
				return fmt.Sprintf("%.2f", get(s))
			},
			rawValue: func(s *settings.Settings) string {
				return strconv.FormatFloat(get(s), 'f', 2, 64)
			},
			set: func(s *settings.Settings, raw string) error {
				raw = strings.TrimSpace(raw)
				f, err := strconv.ParseFloat(raw, 64)
				if err != nil {
					return fmt.Errorf("not a number")
				}
				// IEEE 754 NaN compares false against everything, so
				// it sneaks past the range check below. Reject NaN
				// and Inf explicitly before the comparison so a typed
				// "NaN" can't land in settings.json and break layout
				// math that uses the value.
				if math.IsNaN(f) || math.IsInf(f, 0) {
					return fmt.Errorf("not a finite number")
				}
				if f <= min || f >= max {
					return fmt.Errorf("must be > %.2f and < %.2f", min, max)
				}
				set(s, f)
				return nil
			},
			rangeHint: fmt.Sprintf("(%g < x < %g)", min, max),
		}
	}

	return []settingField{
		boolField("TLS verification skip", "insecure",
			func(s *settings.Settings) bool { return s.SkipTLSVerify },
			func(s *settings.Settings, v bool) { s.SkipTLSVerify = v }, false),
		intField("HTTP request timeout", 1, 600,
			func(s *settings.Settings) int { return s.RequestTimeoutSecs },
			func(s *settings.Settings, n int) { s.RequestTimeoutSecs = n }, "sec", false),
		intField("Max response size", 1, 1024,
			func(s *settings.Settings) int { return s.MaxResponseMiB },
			func(s *settings.Settings, n int) { s.MaxResponseMiB = n }, "MiB", false),
		intField("History limit", 1, 10000,
			func(s *settings.Settings) int { return s.HistoryLimit },
			func(s *settings.Settings, n int) { s.HistoryLimit = n }, "entries", false),
		intField("Text preview cap", 1, 10240,
			func(s *settings.Settings) int { return s.TextPreviewKiB },
			func(s *settings.Settings, n int) { s.TextPreviewKiB = n }, "KiB", false),
		intField("Sidebar max width", 20, 200,
			func(s *settings.Settings) int { return s.SidebarMaxWidth },
			func(s *settings.Settings, n int) { s.SidebarMaxWidth = n }, "cols", false),
		intField("Hex dump cap", 1, 1024,
			func(s *settings.Settings) int { return s.HexDumpKiB },
			func(s *settings.Settings, n int) { s.HexDumpKiB = n }, "KiB", false),
		stringField("Proxy URL", "http://host:port",
			func(s *settings.Settings) string { return s.ProxyURL },
			func(s *settings.Settings, v string) error {
				if v != "" {
					if _, err := url.Parse(v); err != nil {
						return fmt.Errorf("invalid URL: %v", err)
					}
				}
				s.ProxyURL = v
				return nil
			}, false),
		boolField("Disable redirects", "",
			func(s *settings.Settings) bool { return s.DisableRedirects },
			func(s *settings.Settings, v bool) { s.DisableRedirects = v }, false),
		stringField("CA cert file", "/path/to/cert.pem",
			func(s *settings.Settings) string { return s.CACertFile },
			func(s *settings.Settings, v string) error { s.CACertFile = v; return nil },
			true),
		boolField("Enable cookies", "",
			func(s *settings.Settings) bool { return s.EnableCookies },
			func(s *settings.Settings, v bool) { s.EnableCookies = v }, true),
		boolField("Enable mouse", "",
			func(s *settings.Settings) bool { return s.EnableMouse },
			func(s *settings.Settings, v bool) { s.EnableMouse = v }, false),
		intField("Intruder concurrency", 1, 256,
			func(s *settings.Settings) int { return s.IntruderConcurrency },
			func(s *settings.Settings, n int) { s.IntruderConcurrency = n }, "", false),
		intField("Intruder delay (ms)", 0, 60000,
			func(s *settings.Settings) int { return s.IntruderDelayMs },
			func(s *settings.Settings, n int) { s.IntruderDelayMs = n }, "ms", false),
		intField("Intruder max requests", 1, 1000000,
			func(s *settings.Settings) int { return s.IntruderMaxRequests },
			func(s *settings.Settings, n int) { s.IntruderMaxRequests = n }, "", false),
		intField("Intruder body cap", 1, 10240,
			func(s *settings.Settings) int { return s.IntruderResponseBodyCapKiB },
			func(s *settings.Settings, n int) { s.IntruderResponseBodyCapKiB = n }, "KiB", false),
		boolField("OAuth persist tokens", "",
			func(s *settings.Settings) bool { return s.OAuthPersistTokens },
			func(s *settings.Settings, v bool) { s.OAuthPersistTokens = v }, false),
		floatField("Response panel ratio", 0, 1,
			func(s *settings.Settings) float64 { return s.ResponsePanelRatio },
			func(s *settings.Settings, f float64) { s.ResponsePanelRatio = f }, false),
	}
}
