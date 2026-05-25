package app

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea/pollen/internal/collections"
	"github.com/lea/pollen/internal/env"
	"github.com/lea/pollen/internal/history"
	"github.com/lea/pollen/internal/httpx"
	"github.com/lea/pollen/internal/ui"
)

type focusArea int

const (
	focusHistory focusArea = iota
	focusCollections
	focusMethod
	focusURL
	focusQuery
	focusAuth
	focusHeaders
	focusBody
	focusResponse
)

var focusOrder = []focusArea{
	focusHistory, focusCollections, focusMethod, focusURL, focusQuery, focusAuth, focusHeaders, focusBody, focusResponse,
}

type Model struct {
	keys        KeyMap
	store       *history.Store
	collStore   *collections.Store
	env         *env.Env

	method      ui.Method
	urlBar      ui.URLBar
	query       ui.Query
	auth        ui.Auth
	headers     ui.Headers
	body        ui.Body
	response    ui.Response
	history     ui.History
	collUI      ui.Collections

	focus           focusArea
	width           int
	height          int
	showHistory     bool
	showCollections bool

	copyMenuOpen      bool
	helpOpen          bool
	envSwitcherOpen   bool
	envSwitcherCursor int // selected index in env switcher menu

	savingToCollection bool
	saveCollInput      textinput.Model

	importingFile bool
	importInput   textinput.Model

	// tlsInsecure mirrors httpx.SkipTLSVerify so the view layer doesn't have
	// to reach into the http package's globals just to draw a badge.
	tlsInsecure bool

	statusMsg  string // transient toast: copy result / save result / error
	statusKind statusKind
	statusGen  int // monotonic; a clearStatusMsg only clears if its gen matches

	// pendingUndo holds the last deleted history entry for a short window so
	// that the user can press `u` to restore it. Cleared with the status toast.
	pendingUndo *pendingUndo

	// requestGen is bumped on each Send so that responses from older in-flight
	// requests can be discarded when a newer Send is issued. Without this,
	// pressing Ctrl+S twice in quick succession could let the slower (older)
	// request's response overwrite the newer one.
	requestGen int
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

func New(store *history.Store, collStore *collections.Store, e *env.Env) Model {
	if e == nil {
		e = env.New()
	}
	saveInput := textinput.New()
	saveInput.Placeholder = "collection name"
	saveInput.CharLimit = 80
	importInput := textinput.New()
	importInput.Placeholder = "~/projects/api/openapi.yaml"
	importInput.CharLimit = 512
	m := Model{
		keys:        DefaultKeyMap(),
		store:       store,
		collStore:   collStore,
		env:         e,
		method:      ui.NewMethod(),
		urlBar:      ui.NewURLBar(),
		query:       ui.NewQuery(),
		auth:        ui.NewAuth(),
		headers:     ui.NewHeaders(),
		body:        ui.NewBody(),
		response:    ui.NewResponse(),
		history:     ui.NewHistory(),
		collUI:      ui.NewCollections(),
		focus:       focusURL,
		showHistory: true,
		saveCollInput: saveInput,
		importInput:   importInput,
	}
	m.history.SetEntries(store.Entries())
	m.collUI.SetEntries(collStore.Entries())
	// Seed view-visible TLS state from the httpx global (loaded by main.go).
	m.tlsInsecure = httpx.SkipTLSVerify.Load()
	m.applyFocus()
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) applyFocus() {
	m.history.Blur()
	m.collUI.Blur()
	m.method.Blur()
	m.urlBar.Blur()
	m.query.Blur()
	m.auth.Blur()
	m.headers.Blur()
	m.body.Blur()
	m.response.Blur()

	switch m.focus {
	case focusHistory:
		m.history.Focus()
	case focusCollections:
		m.collUI.Focus()
	case focusMethod:
		m.method.Focus()
	case focusURL:
		m.urlBar.Focus()
	case focusQuery:
		m.query.Focus()
	case focusAuth:
		m.auth.Focus()
	case focusHeaders:
		m.headers.Focus()
	case focusBody:
		m.body.Focus()
	case focusResponse:
		m.response.Focus()
	}
}

// setStatus sets a transient message of the given kind. Each call bumps
// statusGen so an older Tick can't wipe a newer message. It also clears any
// pendingUndo — the undo window only lives as long as the "deleted" toast
// itself, so replacing the toast invalidates the undo. Callers that want undo
// must set pendingUndo AFTER calling setStatus.
func (m *Model) setStatus(kind statusKind, msg string) {
	m.statusKind = kind
	m.statusMsg = msg
	m.statusGen++
	m.pendingUndo = nil
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
		if next == focusCollections && !m.showCollections {
			continue
		}
		m.focus = next
		m.applyFocus()
		return
	}
}
