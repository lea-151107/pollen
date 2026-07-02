package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea-151107/pollen/internal/scenario"
)

// scenarioResultMsg carries one completed step from the runner goroutine. gen
// tags the run it belongs to so a late result from a cancelled/superseded run
// is discarded (mirrors sendResultMsg.gen / wsGen).
type scenarioResultMsg struct {
	res scenario.StepResult
	gen int
}

// scenarioDoneMsg signals the runner channel closed for run gen.
type scenarioDoneMsg struct{ gen int }

// startScenarioRun runs the currently-selected saved scenario, expanding each
// step against the active variable environment (plus step/response chaining and
// dynamic vars, inside scenario.Run). It returns the first Cmd that pulls a
// result from the runner channel; the Update loop chains the rest.
func (m *Model) startScenarioRun() tea.Cmd {
	sc, ok := m.scenario.SelectedScenario()
	if !ok {
		return nil
	}
	m.scenarioGen++
	gen := m.scenarioGen
	ctx, cancel := context.WithCancel(context.Background())
	m.scenarioCancel = cancel
	ch := scenario.RunStream(ctx, sc, scenario.RunOpts{Env: m.env})
	m.scenarioCh = ch
	m.scenario.StartResults(sc.Name)
	return nextScenarioResultCmd(ch, gen)
}

// stopScenarioRun cancels an in-flight run and detaches its channel so no
// further results are applied.
func (m *Model) stopScenarioRun() {
	if m.scenarioCancel != nil {
		m.scenarioCancel()
		m.scenarioCancel = nil
	}
	m.scenarioCh = nil
}

// nextScenarioResultCmd reads one StepResult from ch and emits the matching
// message, returning scenarioDoneMsg when the channel closes.
func nextScenarioResultCmd(ch <-chan scenario.StepResult, gen int) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return scenarioDoneMsg{gen: gen}
		}
		return scenarioResultMsg{res: r, gen: gen}
	}
}
