package ui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lea-151107/pollen/internal/intruder"
)

// IntruderState is the three-way mode of the Intruder overlay.
type IntruderState int

const (
	IntruderHidden IntruderState = iota
	IntruderConfig
	IntruderResults
)

// Intruder owns the v1.2.0 Intruder UI: a configuration modal that the
// caller pops up on Ctrl+R, followed by a fullscreen result table that
// streams rows as workers complete. Cancellation (Esc in the results
// view) flows back to the runner via the stored cancel func.
type Intruder struct {
	state IntruderState

	// Config-form inputs.
	kind         intruder.PayloadKind
	payloadInput textinput.Model
	concInput    textinput.Model
	delayInput   textinput.Model
	maxInput     textinput.Model
	focus        int // 0=kind, 1=payload, 2=concurrency, 3=delay, 4=max
	formErr      string

	// Run state.
	results []intruder.Result
	cancel  context.CancelFunc
	done    bool
	runErr  string

	scrollOffset int

	width, height int
}

// IntruderResultMsg carries one runner Result back to the Bubble Tea
// model as a tea.Msg.
type IntruderResultMsg struct {
	Result intruder.Result
}

// IntruderDoneMsg signals the runner channel has closed.
type IntruderDoneMsg struct{}

// NewIntruder constructs an Intruder UI with the given default values
// (taken from settings.json).
func NewIntruder(defaultConcurrency, defaultDelayMs, defaultMaxReq int) Intruder {
	mk := func(placeholder string) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.CharLimit = 256
		ti.Width = 40
		return ti
	}
	payload := mk("e.g. 1-100  or  1-100/5")
	payload.Focus()

	conc := mk("")
	conc.SetValue(strconv.Itoa(defaultConcurrency))
	delay := mk("")
	delay.SetValue(strconv.Itoa(defaultDelayMs))
	max := mk("")
	max.SetValue(strconv.Itoa(defaultMaxReq))

	return Intruder{
		state:        IntruderHidden,
		kind:         intruder.PayloadRange,
		payloadInput: payload,
		concInput:    conc,
		delayInput:   delay,
		maxInput:     max,
		focus:        1,
	}
}

func (m Intruder) State() IntruderState         { return m.state }
func (m Intruder) Results() []intruder.Result    { return m.results }
func (m *Intruder) SetSize(w, h int)             { m.width = w; m.height = h }
func (m *Intruder) OpenConfig()                  { m.state = IntruderConfig; m.formErr = ""; m.scrollOffset = 0 }
func (m *Intruder) Close()                       { m.state = IntruderHidden }
func (m *Intruder) SetFormErr(s string)          { m.formErr = s }
func (m Intruder) IsRunning() bool               { return m.cancel != nil && !m.done }
func (m Intruder) Done() bool                    { return m.done }

// BuildConfig parses the form into an intruder.RunConfig and returns it
// together with an error message string (empty on success).
func (m Intruder) BuildConfig() (intruder.PayloadConfig, int, int, int, string) {
	conc, err := parsePositiveInt(m.concInput.Value(), "concurrency")
	if err != "" {
		return intruder.PayloadConfig{}, 0, 0, 0, err
	}
	delay, err := parseNonNegativeInt(m.delayInput.Value(), "delay")
	if err != "" {
		return intruder.PayloadConfig{}, 0, 0, 0, err
	}
	max, err := parsePositiveInt(m.maxInput.Value(), "max requests")
	if err != "" {
		return intruder.PayloadConfig{}, 0, 0, 0, err
	}
	p, err := parsePayloadInput(m.kind, m.payloadInput.Value())
	if err != "" {
		return intruder.PayloadConfig{}, 0, 0, 0, err
	}
	return p, conc, delay, max, ""
}

// AppendResult records a Result delivered by the runner. The caller (the
// app's Update loop) decides when to call this in response to an
// IntruderResultMsg.
func (m *Intruder) AppendResult(r intruder.Result) {
	m.results = append(m.results, r)
}

// MarkDone records that the runner channel closed.
func (m *Intruder) MarkDone(err string) {
	m.done = true
	m.runErr = err
	m.cancel = nil
}

// StartRun records the cancel func and transitions to the results view.
// Results are reset so a second run within the same session starts
// clean.
func (m *Intruder) StartRun(cancel context.CancelFunc) {
	m.results = nil
	m.scrollOffset = 0
	m.done = false
	m.runErr = ""
	m.cancel = cancel
	m.state = IntruderResults
}

// CancelRun fires the stored cancel func (if any) without changing the
// state — the caller decides whether to close the overlay.
func (m *Intruder) CancelRun() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

// --- Update ---

func (m Intruder) Update(msg tea.Msg) (Intruder, tea.Cmd) {
	switch m.state {
	case IntruderConfig:
		return m.updateConfig(msg)
	case IntruderResults:
		return m.updateResults(msg)
	}
	return m, nil
}

func (m Intruder) updateConfig(msg tea.Msg) (Intruder, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "esc":
		m.Close()
		return m, nil
	case "tab", "shift+tab":
		dir := 1
		if keyMsg.String() == "shift+tab" {
			dir = -1
		}
		m.focus = (m.focus + dir + 5) % 5
		m.applyFocus()
		return m, nil
	case "left":
		if m.focus == 0 {
			m.kind = (m.kind + 3) % 4 // cycle 4 kinds backwards
			m.payloadInput.Placeholder = placeholderFor(m.kind)
			return m, nil
		}
	case "right":
		if m.focus == 0 {
			m.kind = (m.kind + 1) % 4
			m.payloadInput.Placeholder = placeholderFor(m.kind)
			return m, nil
		}
	}
	// Delegate to the focused textinput.
	var cmd tea.Cmd
	switch m.focus {
	case 1:
		m.payloadInput, cmd = m.payloadInput.Update(msg)
	case 2:
		m.concInput, cmd = m.concInput.Update(msg)
	case 3:
		m.delayInput, cmd = m.delayInput.Update(msg)
	case 4:
		m.maxInput, cmd = m.maxInput.Update(msg)
	}
	return m, cmd
}

func (m Intruder) updateResults(msg tea.Msg) (Intruder, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	max := m.maxScrollOffset()
	switch keyMsg.String() {
	case "esc":
		m.CancelRun()
		m.Close()
		return m, nil
	case "up", "k":
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
	case "down", "j":
		if m.scrollOffset < max {
			m.scrollOffset++
		}
	case "pgup":
		m.scrollOffset -= 10
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
	case "pgdown":
		m.scrollOffset += 10
		if m.scrollOffset > max {
			m.scrollOffset = max
		}
	}
	return m, nil
}

// visibleRows is the number of result rows that fit in the table body.
// Kept centralized so updateResults' scroll clamp and viewResults' window
// don't drift apart.
func (m Intruder) visibleRows() int {
	v := m.height - 8
	if v < 5 {
		v = 5
	}
	return v
}

// maxScrollOffset is the largest scrollOffset that still keeps at least
// one row visible; scrolling further would render an empty table body.
func (m Intruder) maxScrollOffset() int {
	n := len(m.results) - m.visibleRows()
	if n < 0 {
		return 0
	}
	return n
}

func (m *Intruder) applyFocus() {
	m.payloadInput.Blur()
	m.concInput.Blur()
	m.delayInput.Blur()
	m.maxInput.Blur()
	switch m.focus {
	case 1:
		m.payloadInput.Focus()
	case 2:
		m.concInput.Focus()
	case 3:
		m.delayInput.Focus()
	case 4:
		m.maxInput.Focus()
	}
}

// --- View ---

func (m Intruder) View() string {
	switch m.state {
	case IntruderConfig:
		return m.viewConfig()
	case IntruderResults:
		return m.viewResults()
	}
	return ""
}

func (m Intruder) viewConfig() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	bold := lipgloss.NewStyle().Bold(true)
	hi := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)

	kindLabel := func(k intruder.PayloadKind, name string) string {
		label := "[ " + name + " ]"
		if m.kind == k {
			return hi.Render(label)
		}
		return dim.Render(label)
	}
	kindRow := strings.Join([]string{
		kindLabel(intruder.PayloadRange, "Range"),
		kindLabel(intruder.PayloadList, "List"),
		kindLabel(intruder.PayloadBrute, "Brute"),
		kindLabel(intruder.PayloadCaseToggle, "CaseToggle"),
	}, "  ")

	marker := func(field int, label string) string {
		ind := "  "
		if m.focus == field {
			ind = hi.Render("> ")
		}
		return ind + label
	}

	lines := []string{
		bold.Render("Intruder — Sniper mode"),
		"",
		marker(0, "Payload kind:  ") + kindRow,
		"",
		marker(1, "Payload:       ") + m.payloadInput.View(),
		dim.Render("  format: " + placeholderFor(m.kind)),
		"",
		marker(2, "Concurrency:   ") + m.concInput.View(),
		marker(3, "Delay (ms):    ") + m.delayInput.View(),
		marker(4, "Max requests:  ") + m.maxInput.View(),
		"",
		dim.Render("Tab: next field  ·  Shift+Tab: prev  ·  ←/→: switch kind  ·  Enter: run  ·  Esc: cancel"),
	}
	if m.formErr != "" {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("error: "+m.formErr))
	}

	body := strings.Join(lines, "\n")
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m Intruder) viewResults() string {
	header := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("Intruder results — %d sent", len(m.results)))
	if m.done {
		state := "done"
		if m.runErr != "" {
			state = "error: " + m.runErr
		}
		header += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("("+state+")")
	} else {
		header += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("(running…)")
	}

	cols := []string{"#", "payload", "status", "size", "ms", "content-type"}
	colW := []int{4, 30, 7, 8, 6, 24}
	headerRow := renderRow(cols, colW, true, false)

	rows := []string{headerRow}
	if len(m.results) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("  (waiting for first response…)"))
	}
	// Window the results so the table never overflows the terminal height.
	visible := m.visibleRows()
	start := m.scrollOffset
	end := start + visible
	if end > len(m.results) {
		end = len(m.results)
	}
	for i := start; i < end; i++ {
		r := m.results[i]
		statusCell := strconv.Itoa(r.Status)
		if r.Error != "" {
			statusCell = "ERR"
		}
		cells := []string{
			strconv.Itoa(r.Index),
			truncate(r.Payload, colW[1]),
			statusCell,
			formatSize(r.Size),
			strconv.FormatInt(r.DurationMs, 10),
			truncate(r.ContentType, colW[5]),
		}
		rows = append(rows, renderRow(cells, colW, false, statusColor(r)))
	}

	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(
		"↑/↓ scroll  ·  PgUp/PgDn page  ·  Esc abort & close")

	body := strings.Join([]string{
		header,
		"",
		strings.Join(rows, "\n"),
		"",
		hint,
	}, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func renderRow(cells []string, widths []int, header bool, highlight bool) string {
	parts := make([]string, len(cells))
	for i, c := range cells {
		s := c
		if len(s) > widths[i] {
			s = s[:widths[i]]
		}
		parts[i] = s + strings.Repeat(" ", widths[i]-len(s))
	}
	row := "  " + strings.Join(parts, "  ")
	style := lipgloss.NewStyle()
	switch {
	case header:
		style = style.Foreground(lipgloss.Color("244")).Bold(true)
	case highlight:
		style = style.Foreground(lipgloss.Color("9"))
	}
	return style.Render(row)
}

func statusColor(r intruder.Result) bool {
	// Non-2xx / network errors get the highlight colour.
	if r.Error != "" {
		return true
	}
	if r.Status >= 400 {
		return true
	}
	return false
}

func placeholderFor(k intruder.PayloadKind) string {
	switch k {
	case intruder.PayloadRange:
		return "<from>-<to>  or  <from>-<to>/<step>"
	case intruder.PayloadList:
		return "a,b,c  or  @/path/to/wordlist"
	case intruder.PayloadBrute:
		return "<alphabet> <min>-<max>   e.g.  abc 1-3"
	case intruder.PayloadCaseToggle:
		return "<base string>   e.g.  admin"
	}
	return ""
}

// parsePayloadInput converts the textinput contents into a
// intruder.PayloadConfig according to the selected kind. Returns an
// empty error string on success.
func parsePayloadInput(kind intruder.PayloadKind, raw string) (intruder.PayloadConfig, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return intruder.PayloadConfig{}, "payload is empty"
	}
	switch kind {
	case intruder.PayloadRange:
		// "1-100" or "1-100/5"
		body, stepStr, hasStep := strings.Cut(raw, "/")
		from, to, ok := strings.Cut(body, "-")
		if !ok {
			return intruder.PayloadConfig{}, "range needs <from>-<to>"
		}
		f, err1 := strconv.Atoi(strings.TrimSpace(from))
		tn, err2 := strconv.Atoi(strings.TrimSpace(to))
		if err1 != nil || err2 != nil {
			return intruder.PayloadConfig{}, "range bounds must be integers"
		}
		step := 1
		if hasStep {
			n, err := strconv.Atoi(strings.TrimSpace(stepStr))
			if err != nil || n < 1 {
				return intruder.PayloadConfig{}, "step must be an integer >= 1"
			}
			step = n
		}
		return intruder.PayloadConfig{Kind: intruder.PayloadRange, From: f, To: tn, Step: step}, ""
	case intruder.PayloadList:
		var words []string
		if strings.HasPrefix(raw, "@") {
			data, err := os.ReadFile(strings.TrimSpace(raw[1:]))
			if err != nil {
				return intruder.PayloadConfig{}, "cannot read wordlist: " + err.Error()
			}
			for _, line := range strings.Split(string(data), "\n") {
				words = append(words, line)
			}
		} else {
			for _, w := range strings.Split(raw, ",") {
				words = append(words, strings.TrimSpace(w))
			}
		}
		return intruder.PayloadConfig{Kind: intruder.PayloadList, Words: words}, ""
	case intruder.PayloadBrute:
		// "<alphabet> <min>-<max>"
		alpha, lens, ok := strings.Cut(raw, " ")
		if !ok {
			return intruder.PayloadConfig{}, "brute needs <alphabet> <min>-<max>"
		}
		minS, maxS, ok2 := strings.Cut(strings.TrimSpace(lens), "-")
		if !ok2 {
			return intruder.PayloadConfig{}, "brute lengths must be <min>-<max>"
		}
		minN, err1 := strconv.Atoi(strings.TrimSpace(minS))
		maxN, err2 := strconv.Atoi(strings.TrimSpace(maxS))
		if err1 != nil || err2 != nil {
			return intruder.PayloadConfig{}, "brute lengths must be integers"
		}
		return intruder.PayloadConfig{
			Kind: intruder.PayloadBrute, Alphabet: alpha,
			MinLen: minN, MaxLen: maxN,
		}, ""
	case intruder.PayloadCaseToggle:
		return intruder.PayloadConfig{Kind: intruder.PayloadCaseToggle, Base: raw}, ""
	}
	return intruder.PayloadConfig{}, "unknown payload kind"
}

func parsePositiveInt(s, name string) (int, string) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 {
		return 0, name + " must be a positive integer"
	}
	return n, ""
}

func parseNonNegativeInt(s, name string) (int, string) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0, name + " must be 0 or a positive integer"
	}
	return n, ""
}
