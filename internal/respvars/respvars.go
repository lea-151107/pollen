// Package respvars resolves response-chaining variable paths — "status",
// "body", "body.<jq>", "headers.<name>" — against an HTTP response. It is the
// shared engine behind the TUI's {{response.*}} tokens (internal/app) and the
// scenario runner's {{steps.<name>.*}} tokens (internal/scenario), so both
// interpret a path the same way.
package respvars

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/itchyny/gojq"

	"github.com/lea-151107/pollen/internal/history"
)

var responseVarRe = regexp.MustCompile(`\{\{response\.([^}]+)\}\}`)

// Resolve returns the value for a single response path against resp. ok is
// false when resp is nil, the path is unknown, or jq evaluation fails, so
// callers can leave the originating token intact (matching env.Expand's
// treatment of undefined variables).
func Resolve(path string, resp *history.Response) (string, bool) {
	if resp == nil {
		return "", false
	}
	switch {
	case path == "status":
		return strconv.Itoa(resp.Status), true

	case path == "body":
		return resp.Body, true

	case strings.HasPrefix(path, "body."):
		jqExpr := "." + path[len("body."):]
		return evalJQ(jqExpr, resp.Body)

	case strings.HasPrefix(path, "headers."):
		name := strings.ToLower(path[len("headers."):])
		for _, h := range resp.Headers {
			if strings.ToLower(h.Key) == name {
				return h.Value, true
			}
		}
		return "", false
	}
	return "", false
}

// Expand replaces {{response.<path>}} tokens in s using resp. Unknown paths and
// evaluation errors leave the token intact.
func Expand(s string, resp *history.Response) string {
	if resp == nil || !strings.Contains(s, "{{response.") {
		return s
	}
	return responseVarRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := responseVarRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		if v, ok := Resolve(sub[1], resp); ok {
			return v
		}
		return match
	})
}

// evalJQ evaluates a jq expression against a JSON string. ok is false on any
// parse/eval error so the caller keeps the original token.
func evalJQ(expr, body string) (string, bool) {
	query, err := gojq.Parse(expr)
	if err != nil {
		return "", false
	}
	var input any
	if err := json.Unmarshal([]byte(body), &input); err != nil {
		return "", false
	}
	iter := query.Run(input)
	v, ok := iter.Next()
	if !ok {
		return "", false
	}
	if _, isErr := v.(error); isErr {
		return "", false
	}
	switch val := v.(type) {
	case string:
		return val, true
	case nil:
		return "", true
	default:
		out, err := json.Marshal(val)
		if err != nil {
			return "", false
		}
		return string(out), true
	}
}
