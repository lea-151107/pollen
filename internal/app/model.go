package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea/pollen/internal/history"
	"github.com/lea/pollen/internal/ui"
)

type focusArea int

const (
	focusHistory focusArea = iota
	focusMethod
	focusURL
	focusHeaders
	focusBody
	focusResponse
)

var focusOrder = []focusArea{
	focusHistory, focusMethod, focusURL, focusHeaders, focusBody, focusResponse,
}

type Model struct {
	keys  KeyMap
	store *history.Store

	method   ui.Method
	urlBar   ui.URLBar
	headers  ui.Headers
	body     ui.Body
	response ui.Response
	history  ui.History

	focus       focusArea
	width       int
	height      int
	showHistory bool

	copyMenuOpen bool
	helpOpen     bool

	statusMsg  string // transient toast: copy result / save result / error
	statusKind statusKind
	statusGen  int // monotonic; a clearStatusMsg only clears if its gen matches

	// pendingUndo holds the last deleted history entry for a short window so
	// that the user can press `u` to restore it. Cleared with the status toast.
	pendingUndo *pendingUndo
}

type pendingUndo struct {
	entry history.Entry
	index int
	gen   int // matches statusGen at the time of delete
}

type statusKind int

const (
	statusOK statusKind = iota
	statusWarn
	statusError
)

func New(store *history.Store) Model {
	m := Model{
		keys:        DefaultKeyMap(),
		store:       store,
		method:      ui.NewMethod(),
		urlBar:      ui.NewURLBar(),
		headers:     ui.NewHeaders(),
		body:        ui.NewBody(),
		response:    ui.NewResponse(),
		history:     ui.NewHistory(),
		focus:       focusURL,
		showHistory: true,
	}
	m.history.SetEntries(store.Entries())
	m.applyFocus()
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) applyFocus() {
	m.history.Blur()
	m.method.Blur()
	m.urlBar.Blur()
	m.headers.Blur()
	m.body.Blur()
	m.response.Blur()

	switch m.focus {
	case focusHistory:
		m.history.Focus()
	case focusMethod:
		m.method.Focus()
	case focusURL:
		m.urlBar.Focus()
	case focusHeaders:
		m.headers.Focus()
	case focusBody:
		m.body.Focus()
	case focusResponse:
		m.response.Focus()
	}
}

// setStatus sets a transient message of the given kind and returns a Tick cmd
// that schedules its automatic clearing after `ttl`. Each call bumps statusGen
// so an older Tick can't wipe a newer message.
func (m *Model) setStatus(kind statusKind, msg string) {
	m.statusKind = kind
	m.statusMsg = msg
	m.statusGen++
}

// statusTick returns a command that clears the current status after ttl,
// scoped by the current statusGen.
func (m Model) statusTick(ttl time.Duration) tea.Cmd {
	gen := m.statusGen
	return tea.Tick(ttl, func(time.Time) tea.Msg {
		return clearStatusMsg{gen: gen}
	})
}

func (m *Model) cycleFocus(forward bool) {
	cur := -1
	for i, f := range focusOrder {
		if f == m.focus {
			cur = i
			break
		}
	}
	if cur < 0 {
		cur = 0
	}
	step := 1
	if !forward {
		step = -1
	}
	for n := 0; n < len(focusOrder); n++ {
		cur = (cur + step + len(focusOrder)) % len(focusOrder)
		next := focusOrder[cur]
		if next == focusHistory && !m.showHistory {
			continue
		}
		m.focus = next
		m.applyFocus()
		return
	}
}
