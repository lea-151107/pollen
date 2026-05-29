// Package app holds the Bubble Tea Model, the Update message dispatch,
// the View composition, and pollen-specific helpers (variable expansion,
// clipboard, save) that glue the UI panels to the storage and HTTP layers.
package app

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea-151107/pollen/internal/collections"
	"github.com/lea-151107/pollen/internal/env"
	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/httpx"
	"github.com/lea-151107/pollen/internal/intruder"
	"github.com/lea-151107/pollen/internal/settings"
	"github.com/lea-151107/pollen/internal/ui"
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

	// collUpdatePromptOpen is true when the user pressed Ctrl+B after loading
	// a collection entry — prompts to update in-place or save as new.
	collUpdatePromptOpen bool
	collUpdateTargetID   string
	collUpdateTargetName string

	// lastLoadedCollID tracks the last collection entry that was loaded into the
	// editor via Enter. Used to offer update-in-place when Ctrl+B is pressed.
	lastLoadedCollID string

	renamingColl   bool
	renameInput    textinput.Model
	renameTargetID string

	importingFile bool
	importInput   textinput.Model

	// tlsInsecure mirrors httpx.SkipTLSVerify so the view layer doesn't have
	// to reach into the http package's globals just to draw a badge.
	tlsInsecure bool

	// responsePanelRatio is the fraction of available width given to the
	// response panel. Loaded from settings.json at startup (default 0.5).
	responsePanelRatio float64

	// sidebarMaxWidth is the maximum width of the left sidebar (history /
	// collections panel). Loaded from settings.json at startup (default 40).
	sidebarMaxWidth int

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

	// intruder owns the Intruder overlay (config modal + result table). The
	// runner channel intruderCh stays non-nil while a run is in flight so
	// Update knows to schedule a follow-up nextResultCmd after each Result.
	intruder   ui.Intruder
	intruderCh <-chan intruder.Result
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

// Options carries optional startup configuration passed from main.
type Options struct {
	// StartCollection opens the collections sidebar at startup and pre-filters
	// it by this name (mirrors the --collection CLI flag).
	StartCollection string
}

func New(store *history.Store, collStore *collections.Store, e *env.Env, opts Options) Model {
	if e == nil {
		e = env.New()
	}
	saveInput := textinput.New()
	saveInput.Placeholder = "collection name"
	saveInput.CharLimit = 80
	renameInput := textinput.New()
	renameInput.Placeholder = "new name"
	renameInput.CharLimit = 80
	importInput := textinput.New()
	importInput.Placeholder = "~/projects/api/openapi.yaml"
	importInput.CharLimit = 512

	showHistory := true
	showCollections := false
	focus := focusURL
	if opts.StartCollection != "" {
		showHistory = false
		showCollections = true
		focus = focusCollections
	}

	m := Model{
		keys:            DefaultKeyMap(),
		store:           store,
		collStore:       collStore,
		env:             e,
		method:          ui.NewMethod(),
		urlBar:          ui.NewURLBar(),
		query:           ui.NewQuery(),
		auth:            ui.NewAuth(),
		headers:         ui.NewHeaders(),
		body:            ui.NewBody(),
		response:        ui.NewResponse(),
		history:         ui.NewHistory(),
		collUI:          ui.NewCollections(),
		focus:           focus,
		showHistory:     showHistory,
		showCollections: showCollections,
		saveCollInput:   saveInput,
		renameInput:     renameInput,
		importInput:     importInput,
	}
	m.history.SetEntries(store.Entries())
	m.collUI.SetEntries(collStore.Entries())
	if opts.StartCollection != "" {
		m.collUI.SetFilter(opts.StartCollection)
	}
	// Seed view-visible TLS state from the httpx global (loaded by main.go).
	m.tlsInsecure = httpx.SkipTLSVerify.Load()
	// Load per-panel settings (defaults applied inside settings.Load).
	intruderConc, intruderDelay, intruderMax := 5, 0, 1000
	if s, err := settings.Load(); err == nil {
		m.responsePanelRatio = s.ResponsePanelRatio
		m.sidebarMaxWidth = s.SidebarMaxWidth
		intruderConc = s.IntruderConcurrency
		intruderDelay = s.IntruderDelayMs
		intruderMax = s.IntruderMaxRequests
	}
	m.intruder = ui.NewIntruder(intruderConc, intruderDelay, intruderMax)
	if m.responsePanelRatio <= 0 || m.responsePanelRatio >= 1 {
		m.responsePanelRatio = 0.5
	}
	if m.sidebarMaxWidth < 20 {
		m.sidebarMaxWidth = 40
	}
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
