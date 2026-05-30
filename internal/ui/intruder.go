package ui

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

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

// IntruderState selects which view the Intruder overlay is rendering.
//
//	IntruderHidden  the overlay is not shown
//	IntruderConfig  the config modal (mode / positions / payloads)
//	IntruderResults the streaming result table
//	IntruderDetail  one selected result's full response (Enter from results)
type IntruderState int

const (
	IntruderHidden IntruderState = iota
	IntruderConfig
	IntruderResults
	IntruderDetail
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

	// In-app CSV export prompt: `e` opens a path input modal at the
	// bottom of the results overlay; Enter emits IntruderExportMsg so
	// the app can write the file (file I/O stays at the app layer).
	exportMode  bool
	exportInput textinput.Model

	// cursor is the focused row inside view(); scrollOffset is the
	// topmost rendered row. up/down moves cursor, scrolling the window
	// only when the cursor would otherwise leave the visible band.
	cursor       int
	scrollOffset int

	// detailIdx is the m.results index of the row whose response is being
	// shown in IntruderDetail state. detailScroll is the vertical scroll
	// position within that detail view.
	detailIdx    int
	detailScroll int

	width, height int
}

// IntruderResultMsg carries one runner Result back to the Bubble Tea
// model as a tea.Msg.
type IntruderResultMsg struct {
	Result intruder.Result
}

// IntruderDoneMsg signals the runner channel has closed.
type IntruderDoneMsg struct{}

// IntruderExportMsg is emitted when the user confirms an in-app CSV
// export path. The app handles file I/O; ui.Intruder only collects the
// path string.
type IntruderExportMsg struct {
	Path string
}

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

	exportInput := mk("")
	exportInput.CharLimit = 512
	exportInput.Width = 60

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
		exportInput:   exportInput,
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
	case IntruderDetail:
		return m.updateDetail(msg)
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
	// Export-path input mode: similar three-layer Esc/Enter pattern as
	// filter input. On Enter we emit IntruderExportMsg and let the app
	// layer write the file.
	if m.exportMode {
		switch keyMsg.String() {
		case "esc":
			m.exportMode = false
			m.exportInput.Blur()
			return m, nil
		case "enter":
			path := strings.TrimSpace(m.exportInput.Value())
			m.exportMode = false
			m.exportInput.Blur()
			if path == "" {
				return m, nil
			}
			return m, func() tea.Msg { return IntruderExportMsg{Path: path} }
		}
		var cmd tea.Cmd
		m.exportInput, cmd = m.exportInput.Update(msg)
		return m, cmd
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
	// Re-clamp scroll offset and cursor on every input. The view can
	// shrink between keystrokes (filter narrowed via typing; new result
	// arrived that changes filter membership; sort/preset reset). If we
	// don't re-clamp, cursor can sit past the last visible row — the
	// ▶ marker disappears AND down/j becomes a no-op (the < rows-1
	// check fails), forcing the user to press Up many times before
	// the cursor reappears.
	max := m.maxScrollOffset()
	if m.scrollOffset > max {
		m.scrollOffset = max
	}
	idx := m.view()
	rows := len(idx)
	if m.cursor >= rows {
		m.cursor = rows - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	visible := m.visibleRows()
	switch keyMsg.String() {
	case "esc":
		// Three-layer Esc: an active filter (substring or preset) is
		// cleared first; only the no-filter Esc actually aborts the run.
		if m.filter != "" || m.preset != presetAll {
			m.filter = ""
			m.preset = presetAll
			m.scrollOffset = 0
			m.cursor = 0
			return m, nil
		}
		m.CancelRun()
		m.Close()
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < rows-1 {
			m.cursor++
		}
	case "pgup":
		m.cursor -= 10
		if m.cursor < 0 {
			m.cursor = 0
		}
	case "pgdown":
		m.cursor += 10
		if m.cursor > rows-1 {
			m.cursor = rows - 1
		}
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		if rows > 0 {
			m.cursor = rows - 1
		}
	case "enter":
		// Push the focused row's response into the detail view.
		if rows > 0 && m.cursor >= 0 && m.cursor < rows {
			m.detailIdx = idx[m.cursor]
			m.detailScroll = 0
			m.state = IntruderDetail
		}
		return m, nil
	case "s":
		// Cycle sort column; default direction is ascending for index
		// (send-order), descending for the numeric metrics so the most
		// interesting rows (slowest, largest, highest status code)
		// surface first.
		m.sortCol = (m.sortCol + 1) % numSortColumns
		m.sortAsc = m.sortCol == sortIndex
		m.scrollOffset = 0
		m.cursor = 0
	case "S":
		m.sortAsc = !m.sortAsc
		m.scrollOffset = 0
		m.cursor = 0
	case "/":
		m.filterMode = true
	case "f":
		m.preset = (m.preset + 1) % numFilterPresets
		m.scrollOffset = 0
		m.cursor = 0
	case "e":
		// Open in-app CSV export prompt. No-op when there are no
		// results yet — the prompt would be misleading.
		if len(m.results) == 0 {
			return m, nil
		}
		m.exportMode = true
		m.exportInput.SetValue(defaultExportPath())
		m.exportInput.CursorEnd()
		return m, m.exportInput.Focus()
	}
	// Keep the cursor visible inside the rendered window.
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if visible > 0 && m.cursor >= m.scrollOffset+visible {
		m.scrollOffset = m.cursor - visible + 1
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	if m.scrollOffset > max {
		m.scrollOffset = max
	}
	return m, nil
}

// updateDetail handles input while the per-result detail view is shown.
// Esc returns to the results table; the body scrolls with arrows /
// PgUp / PgDn.
func (m Intruder) updateDetail(msg tea.Msg) (Intruder, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "esc":
		m.state = IntruderResults
		return m, nil
	case "up", "k":
		if m.detailScroll > 0 {
			m.detailScroll--
		}
	case "down", "j":
		m.detailScroll++
	case "pgup":
		m.detailScroll -= 10
		if m.detailScroll < 0 {
			m.detailScroll = 0
		}
	case "pgdown":
		m.detailScroll += 10
	case "home", "g":
		m.detailScroll = 0
	}
	return m, nil
}

// defaultExportPath returns a timestamped suggestion for the in-app
// CSV export prompt. The user can edit before committing.
func defaultExportPath() string {
	return fmt.Sprintf("intruder-%s.csv", time.Now().Format("20060102-150405"))
}

// rowPredicate decides whether a result row passes one filter token.
type rowPredicate func(intruder.Result) bool

// parseFilter splits a filter string into AND-composed predicates.
// Recognised tokens (whitespace-separated):
//
//	size:N        size == N
//	size:>N       size > N
//	size:<N       size < N
//	size:>=N      size >= N
//	size:<=N      size <= N
//	size:N-M      N <= size <= M
//	dur:...       same operators on DurationMs (milliseconds)
//	s:N           status == N
//	s:NXX         status class (e.g. 4xx → 400..499)
//	s:N-M, s:>N…  same operators on status
//	<bare token>  case-insensitive payload substring
//
// Unparseable tokens fall back to a payload-substring predicate so
// the user sees their typo land somewhere (rather than dropping the
// query silently).
func parseFilter(s string) []rowPredicate {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var preds []rowPredicate
	for _, tok := range strings.Fields(s) {
		preds = append(preds, parseFilterToken(tok))
	}
	return preds
}

func matchPredicates(r intruder.Result, preds []rowPredicate) bool {
	for _, p := range preds {
		if !p(r) {
			return false
		}
	}
	return true
}

func parseFilterToken(tok string) rowPredicate {
	field, expr, ok := strings.Cut(tok, ":")
	if !ok {
		needle := strings.ToLower(tok)
		return func(r intruder.Result) bool {
			return strings.Contains(strings.ToLower(r.Payload), needle)
		}
	}
	switch field {
	case "size":
		return numericPredicate(expr, func(r intruder.Result) int { return r.Size })
	case "dur":
		return numericPredicate(expr, func(r intruder.Result) int { return int(r.DurationMs) })
	case "s", "status":
		return statusPredicate(expr)
	default:
		// Unknown prefix: fall back to substring of the whole raw token.
		needle := strings.ToLower(tok)
		return func(r intruder.Result) bool {
			return strings.Contains(strings.ToLower(r.Payload), needle)
		}
	}
}

func numericPredicate(expr string, get func(intruder.Result) int) rowPredicate {
	lo, hi, op, ok := parseNumExpr(expr)
	if !ok {
		// Unparseable expression: never matches, so the user can tell
		// their DSL didn't work (rows disappear).
		return func(intruder.Result) bool { return false }
	}
	return func(r intruder.Result) bool {
		v := get(r)
		return applyNumOp(v, lo, hi, op)
	}
}

func statusPredicate(expr string) rowPredicate {
	// Special-case the NXX class form before falling through to the
	// generic numeric parser.
	if len(expr) == 3 && (expr[1] == 'x' || expr[1] == 'X') && (expr[2] == 'x' || expr[2] == 'X') {
		if d := int(expr[0] - '0'); d >= 1 && d <= 9 {
			lo := d * 100
			hi := lo + 99
			return func(r intruder.Result) bool {
				return r.Status >= lo && r.Status <= hi
			}
		}
	}
	lo, hi, op, ok := parseNumExpr(expr)
	if !ok {
		return func(intruder.Result) bool { return false }
	}
	return func(r intruder.Result) bool {
		return applyNumOp(r.Status, lo, hi, op)
	}
}

// parseNumExpr parses one of:
//
//	123       (op = "eq",   lo=123, hi=0)
//	>123      (op = "gt",   lo=123)
//	>=123     (op = "ge",   lo=123)
//	<123      (op = "lt",   lo=123)
//	<=123     (op = "le",   lo=123)
//	10-20     (op = "range",lo=10, hi=20)
func parseNumExpr(s string) (lo, hi int, op string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, "", false
	}
	switch {
	case strings.HasPrefix(s, ">="):
		n, err := strconv.Atoi(strings.TrimSpace(s[2:]))
		if err != nil {
			return 0, 0, "", false
		}
		return n, 0, "ge", true
	case strings.HasPrefix(s, "<="):
		n, err := strconv.Atoi(strings.TrimSpace(s[2:]))
		if err != nil {
			return 0, 0, "", false
		}
		return n, 0, "le", true
	case strings.HasPrefix(s, ">"):
		n, err := strconv.Atoi(strings.TrimSpace(s[1:]))
		if err != nil {
			return 0, 0, "", false
		}
		return n, 0, "gt", true
	case strings.HasPrefix(s, "<"):
		n, err := strconv.Atoi(strings.TrimSpace(s[1:]))
		if err != nil {
			return 0, 0, "", false
		}
		return n, 0, "lt", true
	}
	// Range "M-N"? Beware of negative-looking strings; we don't
	// support negative bounds here so a leading "-" is treated as a
	// parse failure.
	if a, b, found := strings.Cut(s, "-"); found && a != "" {
		al, err1 := strconv.Atoi(strings.TrimSpace(a))
		bl, err2 := strconv.Atoi(strings.TrimSpace(b))
		if err1 == nil && err2 == nil {
			return al, bl, "range", true
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, 0, "", false
	}
	return n, 0, "eq", true
}

func applyNumOp(v, lo, hi int, op string) bool {
	switch op {
	case "eq":
		return v == lo
	case "gt":
		return v > lo
	case "ge":
		return v >= lo
	case "lt":
		return v < lo
	case "le":
		return v <= lo
	case "range":
		return v >= lo && v <= hi
	}
	return false
}

// sizeMedian returns the median Size of the results referenced by idx.
// Returns 0 when idx is empty. The output is used to mark outlier rows
// (Size deviating by more than 50% from the median).
func sizeMedian(results []intruder.Result, idx []int) int {
	if len(idx) == 0 {
		return 0
	}
	sizes := make([]int, len(idx))
	for i, k := range idx {
		sizes[i] = results[k].Size
	}
	sort.Ints(sizes)
	mid := len(sizes) / 2
	if len(sizes)%2 == 1 {
		return sizes[mid]
	}
	// Even count: average the two middle values, rounded down.
	return (sizes[mid-1] + sizes[mid]) / 2
}

// isSizeOutlier reports whether r.Size deviates from median by more
// than 50%. median == 0 means "not enough data", in which case nothing
// is flagged.
func isSizeOutlier(r intruder.Result, median int) bool {
	if median <= 0 {
		return false
	}
	delta := r.Size - median
	if delta < 0 {
		delta = -delta
	}
	return delta*2 > median // |Size - median| > median * 0.5
}

// filterDescription returns a short label for the currently active
// filter, suitable for the header badge.
func (m Intruder) filterDescription() string {
	parts := []string{}
	if m.filter != "" {
		// Show the user's raw DSL — they typed it, they recognise it.
		parts = append(parts, m.filter)
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
	preds := parseFilter(m.filter)
	for i, r := range m.results {
		if !matchPredicates(r, preds) {
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
	case IntruderDetail:
		return m.viewDetail()
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
	headerRow := renderRow(cols, colW, true, "", false)

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
	// Compute the size median once per render (over the filtered view,
	// not the whole result set, so outliers reflect what the user is
	// looking at).
	median := sizeMedian(m.results, idx)
	for i := start; i < end; i++ {
		r := m.results[idx[i]]
		statusCell := strconv.Itoa(r.Status)
		if r.Error != "" {
			statusCell = "ERR"
		}
		sizeCell := formatSize(r.Size)
		if isSizeOutlier(r, median) {
			// "!" marker keeps cells ASCII so column alignment via
			// lipgloss.Width stays stable regardless of font width.
			sizeCell += "!"
		}
		cells := []string{
			strconv.Itoa(r.Index),
			truncate(r.Payload, colW[1]),
			statusCell,
			sizeCell,
			strconv.FormatInt(r.DurationMs, 10),
			truncate(r.ContentType, colW[5]),
		}
		rows = append(rows, renderRow(cells, colW, false, statusTint(r), i == m.cursor))
	}

	hintText := "↑/↓ move  ·  Enter detail  ·  s/S sort  ·  / filter  ·  f preset  ·  e export  ·  Esc abort"
	switch {
	case m.filterMode:
		hintText = "type to filter payload  ·  Enter accept  ·  Esc cancel"
	case m.exportMode:
		hintText = "type a path to write CSV  ·  Enter save  ·  Esc cancel"
	}
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(hintText)

	bottomLines := []string{hint}
	switch {
	case m.filterMode:
		prompt := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("/" + m.filter + "█")
		bottomLines = append([]string{prompt}, bottomLines...)
	case m.exportMode:
		prompt := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("export → " + m.exportInput.View())
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

// viewDetail renders the full HTTP response for the currently focused
// result row (set by Enter from updateResults). Esc returns to the
// table; up/down/PgUp/PgDn scroll the body.
func (m Intruder) viewDetail() string {
	if m.detailIdx < 0 || m.detailIdx >= len(m.results) {
		return m.viewResults() // safety fallback
	}
	r := m.results[m.detailIdx]
	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	statusText := r.StatusText
	if statusText == "" && r.Status > 0 {
		statusText = strconv.Itoa(r.Status)
	}
	header := bold.Render(fmt.Sprintf("Intruder result #%d — payload: %s", r.Index, truncate(r.Payload, 60)))
	statusLine := fmt.Sprintf("status: %s   size: %s   duration: %dms   content-type: %s",
		statusText, formatSize(r.Size), r.DurationMs, r.ContentType)
	if r.Error != "" {
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("network error: " + r.Error)
	}

	bodyLines := []string{}
	if r.Response != nil {
		if len(r.Response.Headers) > 0 {
			bodyLines = append(bodyLines, dim.Render("--- headers ---"))
			for _, h := range r.Response.Headers {
				bodyLines = append(bodyLines, h.Key+": "+h.Value)
			}
			bodyLines = append(bodyLines, "")
		}
		bodyLines = append(bodyLines, dim.Render("--- body ---"))
		if r.Response.IsBinary {
			bodyLines = append(bodyLines, dim.Render(fmt.Sprintf("(binary, %d bytes — see hex preview in the main Response panel)", len(r.Response.BodyBytes))))
		} else {
			for _, line := range strings.Split(r.Response.Body, "\n") {
				bodyLines = append(bodyLines, line)
			}
		}
		if r.Response.Truncated {
			bodyLines = append(bodyLines, dim.Render(fmt.Sprintf("(body truncated at %d bytes — adjust intruder_response_body_cap_kib to see more)", len(r.Response.BodyBytes))))
		}
	} else {
		bodyLines = append(bodyLines, dim.Render("(no response body retained)"))
	}

	// Vertical windowing: respect detailScroll and the available height.
	visible := m.height - 8
	if visible < 5 {
		visible = 5
	}
	start := m.detailScroll
	if start < 0 {
		start = 0
	}
	if start > len(bodyLines) {
		start = len(bodyLines)
	}
	end := start + visible
	if end > len(bodyLines) {
		end = len(bodyLines)
	}
	window := bodyLines[start:end]

	hint := dim.Render("↑/↓ scroll  ·  PgUp/PgDn page  ·  Esc back to results")
	body := strings.Join([]string{
		header,
		statusLine,
		"",
		strings.Join(window, "\n"),
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

func renderRow(cells []string, widths []int, header bool, tint lipgloss.Color, cursor bool) string {
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
	prefix := "  "
	if cursor && !header {
		prefix = "▶ "
	}
	row := prefix + strings.Join(parts, "  ")
	style := lipgloss.NewStyle()
	switch {
	case header:
		style = style.Foreground(lipgloss.Color("244")).Bold(true)
	case tint != "":
		style = style.Foreground(tint)
	}
	if cursor && !header {
		style = style.Bold(true)
	}
	return style.Render(row)
}

// statusTint picks a foreground colour for a result row. Empty string
// means "no tint" (terminal default), preserving the v1.2.x behaviour
// for 2xx rows. 4xx is yellow and 5xx + network errors are red so the
// user can distinguish "the server said no" from "the server crashed
// (or didn't answer)".
func statusTint(r intruder.Result) lipgloss.Color {
	if r.Error != "" || r.Status >= 500 {
		return lipgloss.Color("9") // red
	}
	if r.Status >= 400 {
		return lipgloss.Color("11") // yellow
	}
	return ""
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
