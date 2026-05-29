// Package intruder runs a request template against a sequence of payloads
// (numeric range, wordlist, brute force, or case toggles) with configurable
// concurrency and produces a per-payload result row for the TUI table.
//
// It mirrors the Burp Suite Intruder workflow, scoped to the Sniper mode
// (one payload position, expanded from the reserved {{$payload}} token in
// the request URL, body, and headers).
package intruder

import "github.com/lea-151107/pollen/internal/history"

// PayloadMarker is the reserved variable token replaced with each payload
// value when the runner builds a concrete request from the template.
const PayloadMarker = "{{$payload}}"

// AttackMode selects how payloads are distributed across positions.
// v1.2.0 supports Sniper only; the other modes are reserved for later
// versions and never emitted by the current UI.
type AttackMode int

const (
	Sniper AttackMode = iota
)

// PayloadKind identifies which payload generator to instantiate.
type PayloadKind int

const (
	PayloadRange PayloadKind = iota
	PayloadList
	PayloadBrute
	PayloadCaseToggle
)

// PayloadConfig is a tagged union: only the fields relevant to Kind are
// honoured. The runner copies the value, so mutation after Run() returns
// has no effect.
type PayloadConfig struct {
	Kind PayloadKind

	// Range: From <= n <= To with the given Step (>=1, default 1).
	From, To, Step int

	// List: each element is sent verbatim, in slice order.
	Words []string

	// Brute: every combination of Alphabet runes from MinLen up to MaxLen
	// in lexicographic order. MinLen and MaxLen are inclusive and both
	// >= 1.
	Alphabet       string
	MinLen, MaxLen int

	// CaseToggle: every case permutation of Base's ASCII letters. Non-
	// letters pass through unchanged. With L letters, 2^L payloads are
	// emitted.
	Base string
}

// RunConfig groups everything a single Intruder run needs: the request
// template (already env- and response-expanded by the caller), the
// payload generator config, and the concurrency knobs.
type RunConfig struct {
	Mode        AttackMode
	Payload     PayloadConfig
	Template    history.Request
	Concurrency int
	DelayMs     int
	MaxRequests int
}

// Result is one row in the result table.
type Result struct {
	Index       int
	Payload     string
	Status      int
	StatusText  string
	Size        int
	DurationMs  int64
	ContentType string
	Error       string
}

// Run is the rolling state of a single Intruder execution. Results is
// appended to as workers report back; Done flips true when the runner
// finishes (whether by exhaustion, max-requests, or cancellation).
type Run struct {
	Cfg     RunConfig
	Results []Result
	Done    bool
	Err     error
}
