package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lea-151107/pollen/internal/collections"
	"github.com/lea-151107/pollen/internal/scenario"
)

// ScenState selects which view the scenario overlay is rendering.
//
//	ScenHidden   the overlay is not shown
//	ScenList     the saved-scenario list (run / new / delete)
//	ScenBuild    the builder: pick collection entries to compose steps, name it
//	ScenResults  the live per-step result table
type ScenState int

const (
	ScenHidden ScenState = iota
	ScenList
	ScenBuild
	ScenResults
)

// scenBuildFocus is which pane of the builder currently receives input.
type scenBuildFocus int

const (
	buildFocusColl scenBuildFocus = iota // the collection-entry list
	buildFocusName                       // the scenario-name text box
)

// Scenario owns the scenario overlay opened with Ctrl+G. It collects input and
// renders state; running the scenario (network I/O) lives in the app layer,
// which drives this component through StartResults / AppendResult / MarkDone.
type Scenario struct {
	state         ScenState
	width, height int

	// list mode
	saved      []scenario.Scenario
	listCursor int

	// build mode
	collEntries []collections.Entry
	collCursor  int
	steps       []scenario.Step
	nameInput   textinput.Model
	buildFocus  scenBuildFocus
	buildErr    string

	// results mode
	runName string
	results []scenario.StepResult
	running bool
}

// NewScenario constructs a hidden scenario overlay.
func NewScenario() Scenario {
	name := textinput.New()
	name.Placeholder = "scenario name"
	name.CharLimit = 80
	name.Width = 40
	return Scenario{state: ScenHidden, nameInput: name}
}

func (m Scenario) State() ScenState  { return m.state }
func (m *Scenario) SetSize(w, h int) { m.width = w; m.height = h }

// OpenList shows the saved-scenario list.
func (m *Scenario) OpenList(saved []scenario.Scenario) {
	m.state = ScenList
	m.saved = saved
	if m.listCursor >= len(saved) {
		m.listCursor = 0
	}
	m.nameInput.Blur()
}

// OpenBuild starts a fresh builder over the given collection entries.
func (m *Scenario) OpenBuild(colls []collections.Entry) {
	m.state = ScenBuild
	m.collEntries = colls
	m.collCursor = 0
	m.steps = nil
	m.buildErr = ""
	m.buildFocus = buildFocusColl
	m.nameInput.SetValue("")
	m.nameInput.Blur()
}

// Close hides the overlay.
func (m *Scenario) Close() {
	m.state = ScenHidden
	m.nameInput.Blur()
}

// SelectedScenario returns the highlighted saved scenario, if any.
func (m Scenario) SelectedScenario() (scenario.Scenario, bool) {
	if m.listCursor < 0 || m.listCursor >= len(m.saved) {
		return scenario.Scenario{}, false
	}
	return m.saved[m.listCursor], true
}

// SetBuildErr surfaces a validation error in the builder.
func (m *Scenario) SetBuildErr(s string) { m.buildErr = s }

// BuildScenario validates the builder and returns the composed scenario
// (without an ID; the caller assigns one). Step names are derived from the
// source collection names, sanitised into {{steps.<name>}} tokens and made
// unique within the scenario.
func (m Scenario) BuildScenario() (scenario.Scenario, error) {
	name := strings.TrimSpace(m.nameInput.Value())
	if name == "" {
		return scenario.Scenario{}, fmt.Errorf("name is required")
	}
	if len(m.steps) == 0 {
		return scenario.Scenario{}, fmt.Errorf("add at least one step")
	}
	return scenario.Scenario{Name: name, Steps: m.steps}, nil
}

// StartResults switches to the live result view for a run of the named
// scenario.
func (m *Scenario) StartResults(name string) {
	m.state = ScenResults
	m.runName = name
	m.results = nil
	m.running = true
}

// AppendResult records one completed step.
func (m *Scenario) AppendResult(r scenario.StepResult) { m.results = append(m.results, r) }

// MarkDone flags the run as finished so the view stops saying "running".
func (m *Scenario) MarkDone() { m.running = false }

// addStep appends the highlighted collection entry as a step, giving it a
// unique step name.
func (m *Scenario) addStep() {
	if m.collCursor < 0 || m.collCursor >= len(m.collEntries) {
		return
	}
	e := m.collEntries[m.collCursor]
	m.steps = append(m.steps, scenario.Step{
		Name:             m.uniqueStepName(sanitizeToken(e.Name)),
		Request:          e.Request,
		FromCollectionID: e.ID,
	})
	m.buildErr = ""
}

// uniqueStepName ensures base is unique among the current steps, appending
// _2, _3, … as needed. Empty base falls back to "step".
func (m Scenario) uniqueStepName(base string) string {
	if base == "" {
		base = "step"
	}
	taken := make(map[string]bool, len(m.steps))
	for _, s := range m.steps {
		taken[s.Name] = true
	}
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s_%d", base, i)
		if !taken[cand] {
			return cand
		}
	}
}

// sanitizeToken reduces a collection name to a token usable inside
// {{steps.<name>.*}}: lowercase alphanumerics and underscores, no dots.
func sanitizeToken(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_' || r == '.' || r == '/':
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

// --- Update ---

// Update handles input the app does not intercept: list/builder navigation and
// name editing. The app handles the state-changing keys (run / save / delete /
// close) itself.
func (m Scenario) Update(msg tea.Msg) (Scenario, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch m.state {
	case ScenList:
		switch km.String() {
		case "up", "k":
			if m.listCursor > 0 {
				m.listCursor--
			}
		case "down", "j":
			if m.listCursor < len(m.saved)-1 {
				m.listCursor++
			}
		}
		return m, nil

	case ScenBuild:
		if m.buildFocus == buildFocusName {
			switch km.String() {
			case "tab", "up", "down":
				m.buildFocus = buildFocusColl
				m.nameInput.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			return m, cmd
		}
		// buildFocusColl
		switch km.String() {
		case "up", "k":
			if m.collCursor > 0 {
				m.collCursor--
			}
		case "down", "j":
			if m.collCursor < len(m.collEntries)-1 {
				m.collCursor++
			}
		case "a", " ":
			m.addStep()
		case "x", "backspace":
			if len(m.steps) > 0 {
				m.steps = m.steps[:len(m.steps)-1]
			}
		case "tab":
			m.buildFocus = buildFocusName
			m.nameInput.Focus()
		}
		return m, nil
	}
	return m, nil
}

// --- View ---

func (m Scenario) View() string {
	switch m.state {
	case ScenList:
		return m.box(m.viewList())
	case ScenBuild:
		return m.box(m.viewBuild())
	case ScenResults:
		return m.box(m.viewResults())
	}
	return ""
}

func (m Scenario) viewList() string {
	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	sel := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)

	var lines []string
	lines = append(lines, bold.Render("Scenarios"), "")
	if len(m.saved) == 0 {
		lines = append(lines, dim.Render("  (none yet — press n to build one from your collections)"))
	} else {
		for i, s := range m.saved {
			prefix := "  "
			name := s.Name
			line := fmt.Sprintf("%s%s  %s", prefix, name, dim.Render(fmt.Sprintf("(%d steps)", len(s.Steps))))
			if i == m.listCursor {
				line = sel.Render("▶ "+name+" ") + dim.Render(fmt.Sprintf("(%d steps)", len(s.Steps)))
			}
			lines = append(lines, line)
		}
	}
	lines = append(lines, "", dim.Render("Enter: run  ·  n: new  ·  d: delete  ·  Esc: close"))
	return strings.Join(lines, "\n")
}

func (m Scenario) viewBuild() string {
	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	sel := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	var lines []string
	lines = append(lines, bold.Render("New scenario"), "")

	nameLabel := "Name: "
	if m.buildFocus == buildFocusName {
		nameLabel = sel.Render("Name: ")
	}
	lines = append(lines, nameLabel+m.nameInput.View(), "")

	// Steps chosen so far.
	lines = append(lines, bold.Render("Steps:"))
	if len(m.steps) == 0 {
		lines = append(lines, dim.Render("  (none)"))
	} else {
		for i, s := range m.steps {
			lines = append(lines, fmt.Sprintf("  %d. %s  %s", i+1,
				s.Name, dim.Render(s.Request.Method+" "+s.Request.URL)))
		}
	}
	lines = append(lines, "")

	// Available collection entries.
	collHdr := "Collections:"
	if m.buildFocus == buildFocusColl {
		collHdr = sel.Render("Collections:")
	}
	lines = append(lines, collHdr)
	if len(m.collEntries) == 0 {
		lines = append(lines, dim.Render("  (no saved requests — save some with Ctrl+B first)"))
	} else {
		for i, e := range m.collEntries {
			marker := "  "
			label := e.Name
			if i == m.collCursor && m.buildFocus == buildFocusColl {
				marker = "▶ "
				label = sel.Render(e.Name)
			}
			lines = append(lines, marker+label+"  "+dim.Render(e.Request.Method))
		}
	}

	if m.buildErr != "" {
		lines = append(lines, "", errStyle.Render("! "+m.buildErr))
	}
	lines = append(lines, "",
		dim.Render("Tab: switch name/list  ·  a: add step  ·  x: remove last  ·  Ctrl+S/Enter: save  ·  Esc: back"))
	return strings.Join(lines, "\n")
}

func (m Scenario) viewResults() string {
	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	header := bold.Render("Scenario: " + m.runName)
	if m.running {
		header += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("(running…)")
	} else {
		header += "  " + dim.Render("(done)")
	}

	var lines []string
	lines = append(lines, header, "")
	passed := 0
	for i, r := range m.results {
		name := r.Name
		if name == "" {
			name = fmt.Sprintf("step %d", i+1)
		}
		switch {
		case r.Skipped:
			lines = append(lines, dim.Render(fmt.Sprintf("  - %-16s skipped", name)))
		case r.Err != "":
			lines = append(lines, failStyle.Render(fmt.Sprintf("  ✗ %-16s error: %s", name, r.Err)))
		default:
			if r.Failed() {
				lines = append(lines, failStyle.Render(
					fmt.Sprintf("  ✗ %-16s %d  %dms", name, r.Response.Status, r.DurationMs)))
				for _, a := range r.Asserts {
					if !a.Pass {
						lines = append(lines, failStyle.Render(
							fmt.Sprintf("      assert %s: want %q got %q", a.Assertion.Kind, a.Assertion.Want, a.Got)))
					}
				}
			} else {
				passed++
				lines = append(lines, okStyle.Render(
					fmt.Sprintf("  ✓ %-16s %d  %dms", name, r.Response.Status, r.DurationMs)))
			}
		}
	}
	if len(m.results) == 0 {
		lines = append(lines, dim.Render("  (starting…)"))
	}
	if !m.running {
		lines = append(lines, "", fmt.Sprintf("%d/%d steps passed", passed, len(m.results)))
	}
	lines = append(lines, "", dim.Render("Esc: close"))
	return strings.Join(lines, "\n")
}

func (m Scenario) box(body string) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
