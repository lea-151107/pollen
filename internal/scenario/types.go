// Package scenario persists multi-request workflows in
// ~/.config/pollen/scenarios.json and runs them: each Scenario is an ordered
// list of Steps that share a variable context, so a later step can reference an
// earlier step's response via {{steps.<name>.<path>}} (or {{response.<path>}}
// for the immediately preceding step). See internal/respvars for the path
// grammar. The runner reuses internal/httpx to send each request, so proxy /
// TLS / cookie-jar settings applied to the global httpx config apply here too —
// which is what lets a login step's session cookie carry into later steps.
package scenario

import "github.com/lea-151107/pollen/internal/history"

// AssertKind selects which part of a step's response an Assertion checks.
type AssertKind string

const (
	AssertStatus AssertKind = "status" // numeric HTTP status
	AssertBody   AssertKind = "body"   // a jq path into the JSON body
)

// AssertOp is the comparison an Assertion performs against Want.
type AssertOp string

const (
	OpEq       AssertOp = "eq"
	OpContains AssertOp = "contains"
)

// Assertion is a single post-response check on a step. A failing assertion
// stops the scenario (there is no continue-on-error mode yet).
type Assertion struct {
	Kind AssertKind `json:"kind"`
	// Path is the jq path relative to the body (e.g. "user.id"); empty means
	// the whole body. Ignored for AssertStatus.
	Path string   `json:"path,omitempty"`
	Op   AssertOp `json:"op"`
	Want string   `json:"want"`
}

// Step is one request in a scenario. Name identifies the step's output in the
// {{steps.<name>.*}} namespace, so it should be unique within a scenario and
// free of dots. FromCollectionID records the collection entry a step was built
// from (informational; the Request is always the source of truth on run).
type Step struct {
	Name             string          `json:"name"`
	Request          history.Request `json:"request"`
	FromCollectionID string          `json:"from_collection_id,omitempty"`
	Assert           []Assertion     `json:"assert,omitempty"`
}

// Scenario is a named, ordered sequence of steps.
type Scenario struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Steps []Step `json:"steps"`
}

// File is the on-disk envelope for scenarios.json.
type File struct {
	Version int        `json:"version"`
	Entries []Scenario `json:"entries"`
}
