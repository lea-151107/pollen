package scenario

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lea-151107/pollen/internal/dynvars"
	"github.com/lea-151107/pollen/internal/env"
	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/httpx"
	"github.com/lea-151107/pollen/internal/respvars"
)

// RunOpts carries the variable context for a run.
type RunOpts struct {
	// Env expands {{varName}} tokens. May be nil (no env expansion).
	Env *env.Env
}

// AssertResult records how one Assertion fared.
type AssertResult struct {
	Assertion Assertion
	Pass      bool
	Got       string
}

// StepResult is the outcome of a single step. Exactly one of Response/Err is
// meaningful for an executed step; Skipped steps have neither (they follow an
// earlier failure or a cancelled context).
type StepResult struct {
	Name       string
	Request    history.Request
	Response   *history.Response
	Err        string
	DurationMs int64
	Asserts    []AssertResult
	Skipped    bool
}

// Failed reports whether the step counts as a failure: a transport error or
// any failing assertion.
func (r StepResult) Failed() bool {
	if r.Err != "" {
		return true
	}
	for _, a := range r.Asserts {
		if !a.Pass {
			return true
		}
	}
	return false
}

var stepVarRe = regexp.MustCompile(`\{\{steps\.([^.}]+)\.([^}]+)\}\}`)

// expandStepVars replaces {{steps.<name>.<path>}} tokens using the responses
// captured so far. Unknown step names and unresolvable paths leave the token
// intact, matching respvars/env behaviour for undefined variables.
func expandStepVars(s string, outputs map[string]*history.Response) string {
	if !strings.Contains(s, "{{steps.") {
		return s
	}
	return stepVarRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := stepVarRe.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		resp, ok := outputs[sub[1]]
		if !ok {
			return match
		}
		if v, ok := respvars.Resolve(sub[2], resp); ok {
			return v
		}
		return match
	})
}

// Run executes every step in order and returns their results. It is a
// convenience wrapper over RunStream for callers (CLI, tests) that want the
// whole run at once.
func Run(ctx context.Context, sc Scenario, opts RunOpts) []StepResult {
	results := make([]StepResult, 0, len(sc.Steps))
	for r := range RunStream(ctx, sc, opts) {
		results = append(results, r)
	}
	return results
}

// RunStream executes the scenario in a goroutine and emits each StepResult as
// it completes, closing the channel when the run ends. Once a step fails (or
// ctx is cancelled) the remaining steps are emitted as Skipped. The TUI reads
// this channel one result at a time to update its live table.
func RunStream(ctx context.Context, sc Scenario, opts RunOpts) <-chan StepResult {
	ch := make(chan StepResult)
	go func() {
		defer close(ch)
		// send is ctx-aware: if the consumer has stopped reading after a
		// cancel, fall through to <-ctx.Done() and return instead of blocking
		// forever on the unbuffered channel (defer close wakes the reader).
		send := func(r StepResult) bool {
			select {
			case ch <- r:
				return true
			case <-ctx.Done():
				return false
			}
		}
		outputs := make(map[string]*history.Response, len(sc.Steps))
		var prev *history.Response
		aborted := false
		for _, step := range sc.Steps {
			// Cancelled: stop entirely (don't emit Skipped — nobody's reading).
			if ctx.Err() != nil {
				return
			}
			if aborted {
				if !send(StepResult{Name: step.Name, Skipped: true}) {
					return
				}
				continue
			}
			res := runStep(step, outputs, prev, opts)
			if res.Response != nil {
				if step.Name != "" {
					outputs[step.Name] = res.Response
				}
				prev = res.Response
			}
			if res.Failed() {
				aborted = true
			}
			if !send(res) {
				return
			}
		}
	}()
	return ch
}

// runStep expands and sends a single step, then evaluates its assertions.
func runStep(step Step, outputs map[string]*history.Response, prev *history.Response, opts RunOpts) StepResult {
	req := step.Request
	// Expansion order mirrors the TUI's sendRequest: env → step/response
	// chaining → dynamic vars (so $timestamp/$uuid are freshest and any env
	// value containing a $-token is evaluated last).
	expand := func(s string) string {
		if opts.Env != nil {
			s = opts.Env.Expand(s)
		}
		s = expandStepVars(s, outputs)
		s = respvars.Expand(s, prev)
		return dynvars.Expand(s)
	}
	req.URL = expand(req.URL)
	req.Body = expand(req.Body)
	req.GraphQLVariables = expand(req.GraphQLVariables)
	for i := range req.Headers {
		req.Headers[i].Value = expand(req.Headers[i].Value)
	}

	res := StepResult{Name: step.Name, Request: req}
	start := time.Now()
	resp, err := httpx.Do(req)
	res.DurationMs = time.Since(start).Milliseconds()
	if err != nil {
		res.Err = err.Error()
		return res
	}
	res.Response = resp
	res.Asserts = evalAsserts(step.Assert, resp)
	return res
}

// evalAsserts checks each assertion against resp.
func evalAsserts(asserts []Assertion, resp *history.Response) []AssertResult {
	if len(asserts) == 0 {
		return nil
	}
	out := make([]AssertResult, 0, len(asserts))
	for _, a := range asserts {
		var got string
		switch a.Kind {
		case AssertStatus:
			got = strconv.Itoa(resp.Status)
		case AssertBody:
			path := "body"
			if a.Path != "" {
				path = "body." + a.Path
			}
			got, _ = respvars.Resolve(path, resp)
		}
		pass := false
		switch a.Op {
		case OpEq:
			pass = got == a.Want
		case OpContains:
			pass = strings.Contains(got, a.Want)
		}
		out = append(out, AssertResult{Assertion: a, Pass: pass, Got: got})
	}
	return out
}
