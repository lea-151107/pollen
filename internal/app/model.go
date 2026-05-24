package app

import (
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
	statusMsg string // transient toast: copy result / save result / error

	errorMsg string
}

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
