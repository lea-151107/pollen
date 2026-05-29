package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea-151107/pollen/internal/history"
	intruderpkg "github.com/lea-151107/pollen/internal/intruder"
	"github.com/lea-151107/pollen/internal/ui"
)

// intruderTemplate produces the request template the Intruder runner
// consumes: the current editor state, with env vars and response
// chaining already expanded. The {{$payload}} marker is preserved
// because neither expander matches `$`.
func (m *Model) intruderTemplate() history.Request {
	req := m.currentRequest()
	lastResp := m.response.CurrentResponse()
	expand := func(s string) string {
		return expandResponseVars(m.env.Expand(s), lastResp)
	}
	req.URL = expand(req.URL)
	req.Body = expand(req.Body)
	for i := range req.Headers {
		req.Headers[i].Value = expand(req.Headers[i].Value)
	}
	return req
}

// startIntruderRun builds the run config from the form, starts the
// runner, and returns the first tea.Cmd that pulls from the result
// channel. On validation failure the Intruder modal's formErr field
// is set and nil cmd is returned (the modal stays open).
func (m *Model) startIntruderRun() tea.Cmd {
	template := m.intruderTemplate()
	payload, conc, delay, max, errMsg := m.intruder.BuildConfig()
	if errMsg != "" {
		m.intruder.SetFormErr(errMsg)
		return nil
	}
	cfg := intruderpkg.RunConfig{
		Mode:        intruderpkg.Sniper,
		Payloads:    []intruderpkg.PayloadConfig{payload},
		Template:    template,
		Concurrency: conc,
		DelayMs:     delay,
		MaxRequests: max,
	}
	if err := intruderpkg.HasMarkers(template, cfg.Mode, len(cfg.Payloads)); err != nil {
		m.intruder.SetFormErr(err.Error() + " — add a marker to URL, body, or a header first")
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := intruderpkg.Start(ctx, cfg)
	if err != nil {
		cancel()
		m.intruder.SetFormErr(err.Error())
		return nil
	}
	m.intruder.StartRun(cancel)
	m.intruderCh = ch
	return nextIntruderResultCmd(ch)
}

// nextIntruderResultCmd reads one Result from ch and emits the
// matching tea.Msg, returning IntruderDoneMsg when the channel
// closes. The Update loop chains successive calls so each Result
// triggers a re-render.
func nextIntruderResultCmd(ch <-chan intruderpkg.Result) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return ui.IntruderDoneMsg{}
		}
		return ui.IntruderResultMsg{Result: r}
	}
}
