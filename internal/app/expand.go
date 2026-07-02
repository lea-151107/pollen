package app

import (
	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/respvars"
)

// expandResponseVars replaces {{response.*}} tokens in s using values from
// resp. Unknown paths and evaluation errors leave the token intact, matching
// the behaviour of env.Expand for undefined variables. The path grammar
// ("status", "body", "body.<jq>", "headers.<name>") lives in internal/respvars
// so the scenario runner's {{steps.<name>.*}} tokens resolve identically.
func expandResponseVars(s string, resp *history.Response) string {
	return respvars.Expand(s, resp)
}
