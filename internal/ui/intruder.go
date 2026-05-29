package ui

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lea-151107/pollen/internal/intruder"
)

// sortColumn identifies which result-table column the user is sorting by.
// Cycled by `s` in the results view.
type sortColumn int

const (
	sortIndex sortColumn = iota
	sortStatus
	sortSize
	sortDuration
	numSortColumns
)

// filterPreset is a coarse status-class filter the user toggles with `f`,
// orthogonal to the substring filter and meant for the common "show me
// only failing rows" / "show me only successes" lookup.
type filterPreset int

const (
	presetAll filterPreset = iota
	presetErrors
	presetSuccess
	numFilterPresets
)

// IntruderState is the three-way mode of the Intruder overlay.
type IntruderState int

const (
	IntruderHidden IntruderState = iota
	IntruderConfig
	IntruderResults
)

// maxPositions caps the number of payload positions a Pitchfork or
// ClusterBomb run can declare. 8 is well past any realistic credential
// stuffing or fuzzing target and keeps the config UI usable.
const maxPositions = 8

// Intruder owns the Intruder UI: a configuration modal that the
// caller pops up on Ctrl+R, followed by a fullscreen result table that
// streams rows as workers complete. Cancellation (Esc in the results
// view) flows back to the runner via the stored cancel func.
type Intruder struct {
	state IntruderState

	// Config-form inputs.
	mode          intruder.AttackMode
	kinds         []intruder.PayloadKind // length = number of payload positions
	payloadInputs []textinput.Model      // length = number of payload positions
	concInput     textinput.Model
	delayInput    textinput.Model
	maxInput      textinput.Model
	focus         int // see focusLayout
	formErr       string

	// Run state.
	results []intruder.Result
	cancel  context.CancelFunc
	done    bool
	runErr  string

	// Sort state for the result table. sortIndex/ascending matches the
	// default v1.2.0 "send order" so existing users see no behaviour
	// change until they press `s`.
	sortCol sortColumn
	sortAsc bool

	// Filter state. filter is a payload substring match (lowercased on
	// comparison); preset is a status-class quick filter. Both apply
	// before sorting in view(). filterMode is true while the user is
	// typing into the bottom-of-screen prompt.
	filter     string
	filterMode bool
	preset     filterPreset

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
		state:         IntruderHidden,
		mode:          intruder.Sniper,
		kinds:         []intruder.PayloadKind{intruder.PayloadRange},
		payloadInputs: []textinput.Model{payload},
		concInput:     conc,
		delayInput:    delay,
		maxInput:      max,
		focus:         2, // start at the first payload input (after mode + position-count)
		sortCol:       sortIndex,
		sortAsc:       true,
	}
}

// focusLayout maps the modal's focus index to a conceptual field. The
// layout grows with the number of payload positions:
//
//	0           mode selector
//	1           position count selector
//	2 + 2*i     kind selector for payload i (i = 0..N-1)
//	3 + 2*i     text input for payload i
//	2 + 2*N     concurrency input
//	3 + 2*N     delay input
//	4 + 2*N     max-requests input
func (m Intruder) totalFocusFields() int {
	return 5 + 2*len(m.kinds)
}

func (m Intruder) focusConcurrency() int { return 2 + 2*len(m.kinds) }
func (m Intruder) focusDelay() int       { return 3 + 2*len(m.kinds) }
func (m Intruder) focusMax() int         { return 4 + 2*len(m.kinds) }

// positionForFocus returns the payload index (0-based) the focus is
// currently on, or -1 if focus isn't on a payload row. kindRow is true
// when focus is on the kind selector, false for the text input.
func (m Intruder) positionForFocus() (idx int, kindRow bool) {
	if m.focus < 2 || m.focus >= m.focusConcurrency() {
		return -1, false
	}
	off := m.focus - 2
	return off / 2, off%2 == 0
}

func (m Intruder) State() IntruderState         { return m.state }
func (m Intruder) Results() []intruder.Result    { return m.results }
func (m *Intruder) SetSize(w, h int)             { m.width = w; m.height = h }
func (m *Intruder) OpenConfig()                  { m.state = IntruderConfig; m.formErr = ""; m.scrollOffset = 0 }
func (m *Intruder) Close()                       { m.state = IntruderHidden }
func (m *Intruder) SetFormErr(s string)          { m.formErr = s }
func (m Intruder) IsRunning() bool               { return m.cancel != nil && !m.done }
func (m Intruder) Done() bool                    { return m.done }

// BuildConfig parses the form into an attack mode, the list of payload
// configs (one per position), and the concurrency knobs. The error
// string is empty on success.
func (m Intruder) BuildConfig() (intruder.AttackMode, []intruder.PayloadConfig, int, int, int, string) {
	conc, err := parsePositiveInt(m.concInput.Value(), "concurrency")
	if err != "" {
		return 0, nil, 0, 0, 0, err
	}
	delay, err := parseNonNegativeInt(m.delayInput.Value(), "delay")
	if err != "" {
		return 0, nil, 0, 0, 0, err
	}
	max, err := parsePositiveInt(m.maxInput.Value(), "max requests")
	if err != "" {
		return 0, nil, 0, 0, 0, err
	}
	configs := make([]intruder.PayloadConfig, len(m.kinds))
	for i, k := range m.kinds {
		p, perr := parsePayloadInput(k, m.payloadInputs[i].Value())
		if perr != "" {
			return 0, nil, 0, 0, 0, fmt.Sprintf("payload %d: %s", i+1, perr)
		}
		configs[i] = p
	}
	return m.mode, configs, conc, delay, max, ""
}

// SetPositionCount resizes the payload slice. Sniper is locked to 1;
// Pitchfork / ClusterBomb clamp to [2, maxPositions]. Growing adds
// fresh Range inputs; shrinking drops the trailing ones.
func (m *Intruder) setPositionCount(n int) {
	if m.mode == intruder.Sniper {
		n = 1
	} else {
		if n < 2 {
			n = 2
		}
		if n > maxPositions {
			n = maxPositions
		}
	}
	for len(m.kinds) < n {
		ti := textinput.New()
		ti.Placeholder = "e.g. 1-100"
		ti.CharLimit = 256
		ti.Width = 40
		m.kinds = append(m.kinds, intruder.PayloadRange)
		m.payloadInputs = append(m.payloadInputs, ti)
	}
	if len(m.kinds) > n {
		m.kinds = m.kinds[:n]
		m.payloadInputs = m.payloadInputs[:n]
	}
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
	total := m.totalFocusFields()
	switch keyMsg.String() {
	case "esc":
		m.Close()
		return m, nil
	case "tab", "shift+tab":
		dir := 1
		if keyMsg.String() == "shift+tab" {
			dir = -1
		}
		m.focus = (m.focus + dir + total) % total
		m.applyFocus()
		return m, nil
	case "left", "right":
		dir := 1
		if keyMsg.String() == "left" {
			dir = -1
		}
		switch m.focus {
		case 0:
			// Cycle attack mode and snap position count to a valid value.
			modes := []intruder.AttackMode{intruder.Sniper, intruder.Pitchfork, intruder.ClusterBomb}
			idx := 0
			for i, mm := range modes {
				if mm == m.mode {
					idx = i
				}
			}
			idx = (idx + dir + len(modes)) % len(modes)
			m.mode = modes[idx]
			// Ensure positions match the mode (Sniper → 1, others → >=2).
			n := len(m.kinds)
			if m.mode == intruder.Sniper {
				n = 1
			} else if n < 2 {
				n = 2
			}
			m.setPositionCount(n)
			if m.focus >= total {
				m.focus = 0
			}
			m.applyFocus()
			return m, nil
		case 1:
			// Cycle position count within the mode's allowed range.
			if m.mode == intruder.Sniper {
				return m, nil
			}
			n := len(m.kinds) + dir
			if n < 2 {
				n = 2
			}
			if n > maxPositions {
				n = maxPositions
			}
			m.setPositionCount(n)
			if m.focus >= m.totalFocusFields() {
				m.focus = m.totalFocusFields() - 1
			}
			m.applyFocus()
			return m, nil
		default:
			if idx, kindRow := m.positionForFocus(); kindRow && idx >= 0 {
				m.kinds[idx] = intruder.PayloadKind((int(m.kinds[idx]) + dir + 4) % 4)
				m.payloadInputs[idx].Placeholder = placeholderFor(m.kinds[idx])
				return m, nil
			}
		}
	}
	// Delegate to the focused textinput.
	var cmd tea.Cmd
	switch {
	case m.focus == m.focusConcurrency():
		m.concInput, cmd = m.concInput.Update(msg)
	case m.focus == m.focusDelay():
		m.delayInput, cmd = m.delayInput.Update(msg)
	case m.focus == m.focusMax():
		m.maxInput, cmd = m.maxInput.Update(msg)
	default:
		if idx, kindRow := m.positionForFocus(); !kindRow && idx >= 0 {
			m.payloadInputs[idx], cmd = m.payloadInputs[idx].Update(msg)
		}
	}
	return m, cmd
}

func (m Intruder) updateResults(msg tea.Msg) (Intruder, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	// Filter-input mode: swallow every key as filter editing until the
	// user commits (Enter) or aborts (Esc). Mirrors the three-layer
	// pattern in internal/ui/history.go so users get the same muscle
	// memory across the two filterable panels.
	if m.filterMode {
		switch keyMsg.String() {
		case "esc":
			m.filter = ""
			m.filterMode = false
			m.scrollOffset = 0
		case "enter":
			m.filterMode = false
		case "backspace":
			if rs := []rune(m.filter); len(rs) > 0 {
				m.filter = string(rs[:len(rs)-1])
				m.scrollOffset = 0
			}
		default:
			// Accept any single printable rune. Matches history.go's
			// pattern so multi-byte glyphs (CJK kanji, accented Latin)
			// are appended; named keys like "left" or "ctrl+a" are
			// multi-rune strings and excluded.
			s := keyMsg.String()
			rs := []rune(s)
			if len(rs) == 1 && rs[0] >= ' ' {
				m.filter += s
				m.scrollOffset = 0
			}
		}
		return m, nil
	}
	// Re-clamp the scroll offset on every input. If the view shrank
	// since the last keystroke (a Result arrived that changes filter
	// membership, or the user just narrowed via /f), scrollOffset can
	// be stale; this catches it before the next nav key would silently
	// no-op.
	max := m.maxScrollOffset()
	if m.scrollOffset > max {
		m.scrollOffset = max
	}
	switch keyMsg.String() {
	case "esc":
		// Three-layer Esc: an active filter (substring or preset) is
		// cleared first; only the no-filter Esc actually aborts the run.
		if m.filter != "" || m.preset != presetAll {
			m.filter = ""
			m.preset = presetAll
			m.scrollOffset = 0
			return m, nil
		}
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
	case "s":
		// Cycle sort column; default direction is ascending for index
		// (send-order), descending for the numeric metrics so the most
		// interesting rows (slowest, largest, highest status code)
		// surface first.
		m.sortCol = (m.sortCol + 1) % numSortColumns
		m.sortAsc = m.sortCol == sortIndex
		m.scrollOffset = 0
	case "S":
		m.sortAsc = !m.sortAsc
		m.scrollOffset = 0
	case "/":
		m.filterMode = true
	case "f":
		m.preset = (m.preset + 1) % numFilterPresets
		m.scrollOffset = 0
	}
	return m, nil
}

// filterDescription returns a short label for the currently active
// filter, suitable for the header badge.
func (m Intruder) filterDescription() string {
	parts := []string{}
	if m.filter != "" {
		parts = append(parts, "payload~"+m.filter)
	}
	switch m.preset {
	case presetErrors:
		parts = append(parts, "errors only")
	case presetSuccess:
		parts = append(parts, "2xx only")
	}
	return strings.Join(parts, ", ")
}

// view returns the indices into m.results in the order the table should
// render them, after applying the substring filter and the status-class
// preset. The underlying slice is never reordered so AppendResult can
// safely keep streaming in send-order while the user sorts and filters.
func (m Intruder) view() []int {
	out := make([]int, 0, len(m.results))
	needle := strings.ToLower(m.filter)
	for i, r := range m.results {
		if needle != "" && !strings.Contains(strings.ToLower(r.Payload), needle) {
			continue
		}
		switch m.preset {
		case presetErrors:
			if r.Error == "" && r.Status < 400 {
				continue
			}
		case presetSuccess:
			if !(r.Status >= 200 && r.Status < 300) {
				continue
			}
		}
		out = append(out, i)
	}
	if m.sortCol == sortIndex && m.sortAsc {
		return out // already in send order, filter preserves order
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := m.results[out[i]], m.results[out[j]]
		cmp := 0
		switch m.sortCol {
		case sortStatus:
			cmp = a.Status - b.Status
		case sortSize:
			cmp = a.Size - b.Size
		case sortDuration:
			switch {
			case a.DurationMs < b.DurationMs:
				cmp = -1
			case a.DurationMs > b.DurationMs:
				cmp = 1
			}
		default:
			cmp = a.Index - b.Index
		}
		if !m.sortAsc {
			cmp = -cmp
		}
		return cmp < 0
	})
	return out
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
// Driven off len(view()) so applying a filter immediately shrinks the
// scroll range.
func (m Intruder) maxScrollOffset() int {
	n := len(m.view()) - m.visibleRows()
	if n < 0 {
		return 0
	}
	return n
}

func (m *Intruder) applyFocus() {
	for i := range m.payloadInputs {
		m.payloadInputs[i].Blur()
	}
	m.concInput.Blur()
	m.delayInput.Blur()
	m.maxInput.Blur()
	switch {
	case m.focus == m.focusConcurrency():
		m.concInput.Focus()
	case m.focus == m.focusDelay():
		m.delayInput.Focus()
	case m.focus == m.focusMax():
		m.maxInput.Focus()
	default:
		if idx, kindRow := m.positionForFocus(); !kindRow && idx >= 0 {
			m.payloadInputs[idx].Focus()
		}
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

	highlight := func(name string, active bool) string {
		label := "[ " + name + " ]"
		if active {
			return hi.Render(label)
		}
		return dim.Render(label)
	}
	modeRow := strings.Join([]string{
		highlight("Sniper", m.mode == intruder.Sniper),
		highlight("Pitchfork", m.mode == intruder.Pitchfork),
		highlight("ClusterBomb", m.mode == intruder.ClusterBomb),
	}, "  ")
	kindRow := func(k intruder.PayloadKind) string {
		return strings.Join([]string{
			highlight("Range", k == intruder.PayloadRange),
			highlight("List", k == intruder.PayloadList),
			highlight("Brute", k == intruder.PayloadBrute),
			highlight("CaseToggle", k == intruder.PayloadCaseToggle),
		}, "  ")
	}

	marker := func(field int, label string) string {
		ind := "  "
		if m.focus == field {
			ind = hi.Render("> ")
		}
		return ind + label
	}

	title := "Intruder — Sniper mode"
	switch m.mode {
	case intruder.Pitchfork:
		title = "Intruder — Pitchfork mode"
	case intruder.ClusterBomb:
		title = "Intruder — ClusterBomb mode"
	}

	lines := []string{
		bold.Render(title),
		"",
		marker(0, "Mode:          ") + modeRow,
	}
	if m.mode != intruder.Sniper {
		lines = append(lines, marker(1, "Positions:     ")+hi.Render(strconv.Itoa(len(m.kinds)))+dim.Render("   (←/→ to adjust, 2.."+strconv.Itoa(maxPositions)+")"))
	} else {
		lines = append(lines, marker(1, "Positions:     ")+dim.Render("1  (Sniper)"))
	}
	for i := range m.kinds {
		label := "Payload " + strconv.Itoa(i+1) + " kind: "
		if len(m.kinds) == 1 {
			label = "Payload kind:  "
		}
		lines = append(lines, "")
		lines = append(lines, marker(2+2*i, label)+kindRow(m.kinds[i]))
		input := m.payloadInputs[i]
		input.Placeholder = placeholderFor(m.kinds[i])
		ilabel := "Payload " + strconv.Itoa(i+1) + ":      "
		if len(m.kinds) == 1 {
			ilabel = "Payload:       "
		}
		lines = append(lines, marker(3+2*i, ilabel)+input.View())
		lines = append(lines, dim.Render("  format: "+placeholderFor(m.kinds[i])))
	}
	lines = append(lines,
		"",
		marker(m.focusConcurrency(), "Concurrency:   ")+m.concInput.View(),
		marker(m.focusDelay(), "Delay (ms):    ")+m.delayInput.View(),
		marker(m.focusMax(), "Max requests:  ")+m.maxInput.View(),
		"",
		dim.Render("Tab: next field  ·  Shift+Tab: prev  ·  ←/→: switch mode/kind/positions  ·  Enter: run  ·  Esc: cancel"),
	)
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
	if filterActive := m.filter != "" || m.preset != presetAll; filterActive {
		shown := len(m.view())
		header += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(
			fmt.Sprintf("(%d/%d shown · %s)", shown, len(m.results), m.filterDescription()))
	}

	colLabels := []string{"#", "payload", "status", "size", "ms", "content-type"}
	// Mark the active sort column with ▲/▼ so the user can see at a
	// glance which column drives the row order.
	sortMarker := func(col sortColumn, label string) string {
		if m.sortCol != col {
			return label
		}
		if m.sortAsc {
			return label + " ▲"
		}
		return label + " ▼"
	}
	cols := []string{
		sortMarker(sortIndex, colLabels[0]),
		colLabels[1],
		sortMarker(sortStatus, colLabels[2]),
		sortMarker(sortSize, colLabels[3]),
		sortMarker(sortDuration, colLabels[4]),
		colLabels[5],
	}
	colW := []int{4, 30, 9, 10, 8, 24}
	headerRow := renderRow(cols, colW, true, false)

	rows := []string{headerRow}
	idx := m.view()
	if len(idx) == 0 {
		// Distinguish "still waiting" from "filter excluded everything"
		// so the user doesn't think the run hung when in fact their
		// filter is too strict.
		msg := "  (waiting for first response…)"
		if len(m.results) > 0 {
			msg = "  (no results match filter)"
		}
		rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(msg))
	}
	// Window the results so the table never overflows the terminal height.
	visible := m.visibleRows()
	start := m.scrollOffset
	end := start + visible
	if end > len(idx) {
		end = len(idx)
	}
	for i := start; i < end; i++ {
		r := m.results[idx[i]]
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

	hintText := "↑/↓ scroll  ·  s/S sort  ·  / filter  ·  f preset  ·  Esc abort"
	if m.filterMode {
		hintText = "type to filter payload  ·  Enter accept  ·  Esc cancel"
	}
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(hintText)

	bottomLines := []string{hint}
	if m.filterMode {
		prompt := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("/" + m.filter + "█")
		bottomLines = append([]string{prompt}, bottomLines...)
	}

	body := strings.Join([]string{
		header,
		"",
		strings.Join(rows, "\n"),
		"",
		strings.Join(bottomLines, "\n"),
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
		// Use visual width so multi-byte glyphs like ▲/▼ in the header
		// and CJK characters in user payloads truncate and pad
		// correctly. Byte-slicing would cut UTF-8 sequences mid-way.
		w := lipgloss.Width(s)
		if w > widths[i] {
			rs := []rune(s)
			for w > widths[i] && len(rs) > 0 {
				rs = rs[:len(rs)-1]
				s = string(rs)
				w = lipgloss.Width(s)
			}
		}
		parts[i] = s + strings.Repeat(" ", widths[i]-w)
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
