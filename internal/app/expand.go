package app

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/itchyny/gojq"

	"github.com/lea-151107/pollen/internal/history"
)

var responseVarRe = regexp.MustCompile(`\{\{response\.([^}]+)\}\}`)

// expandResponseVars replaces {{response.*}} tokens in s using values from
// resp. Unknown paths and evaluation errors leave the token intact, matching
// the behaviour of env.Expand for undefined variables.
func expandResponseVars(s string, resp *history.Response) string {
	if resp == nil || !strings.Contains(s, "{{response.") {
		return s
	}
	return responseVarRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := responseVarRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		path := sub[1]

		switch {
		case path == "status":
			return strconv.Itoa(resp.Status)

		case path == "body":
			return resp.Body

		case strings.HasPrefix(path, "body."):
			jqExpr := "." + path[len("body."):]
			return evalJQ(jqExpr, resp.Body, match)

		case strings.HasPrefix(path, "headers."):
			name := strings.ToLower(path[len("headers."):])
			for _, h := range resp.Headers {
				if strings.ToLower(h.Key) == name {
					return h.Value
				}
			}
			return match
		}
		return match
	})
}

// evalJQ evaluates a jq expression against a JSON string and returns the
// result as a string. Returns fallback on any parse/eval error.
func evalJQ(expr, body, fallback string) string {
	query, err := gojq.Parse(expr)
	if err != nil {
		return fallback
	}
	var input any
	if err := json.Unmarshal([]byte(body), &input); err != nil {
		return fallback
	}
	iter := query.Run(input)
	v, ok := iter.Next()
	if !ok {
		return fallback
	}
	if _, isErr := v.(error); isErr {
		return fallback
	}
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		out, err := json.Marshal(val)
		if err != nil {
			return fallback
		}
		return string(out)
	}
}
