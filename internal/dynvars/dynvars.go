// Package dynvars expands {{$name}} / {{$name:arg}} tokens that pollen
// computes at expansion time, complementing the user-defined {{varName}}
// substitution in env. The two are deliberately disjoint namespaces:
// dynvars all start with `$` so they never collide with env vars, and the
// existing intruder {{$payload}} / {{$payloadN}} markers are passed
// through unchanged (their names aren't in the built-in table).
package dynvars

import (
	"encoding/base64"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// dynRE matches {{$name}} and {{$name:arg}}. name is lower-case
// alphanumeric + underscore; arg is anything except `}`. Numeric-only
// names (like {{$payload1}}) are excluded so the intruder markers
// continue to survive.
var dynRE = regexp.MustCompile(`\{\{\$([a-z][a-z0-9_]*)(?::([^}]*))?\}\}`)

// Expand replaces every recognised {{$name}} / {{$name:arg}} token. Unknown
// names are left intact verbatim, so callers can chain Expand with other
// substitution layers (env, response chaining, intruder markers) in any
// order without one accidentally consuming another's tokens.
//
// Built-ins:
//
//	{{$timestamp}}       Unix epoch seconds
//	{{$timestamp_ms}}    Unix epoch milliseconds
//	{{$datetime}}        RFC3339 UTC timestamp
//	{{$uuid}}            UUID v4
//	{{$random}}          random uint32
//	{{$random:N}}        random 0..N-1
//	{{$random:M-N}}      random M..N (inclusive)
//	{{$base64:VALUE}}    base64-encode VALUE
//	{{$urlencode:VALUE}} URL-encode VALUE
//
// Each call to Expand evaluates dynamics fresh, so {{$uuid}} differs
// between successive requests (essential for the intruder runner, which
// calls Expand inside the worker loop).
func Expand(s string) string {
	if !strings.Contains(s, "{{$") {
		return s // common-case fast path
	}
	return dynRE.ReplaceAllStringFunc(s, func(match string) string {
		sub := dynRE.FindStringSubmatch(match)
		name := sub[1]
		arg := ""
		if len(sub) >= 3 {
			arg = sub[2]
		}
		v, ok := evaluate(name, arg)
		if !ok {
			return match // unknown built-in — pass through verbatim
		}
		return v
	})
}

func evaluate(name, arg string) (string, bool) {
	switch name {
	case "timestamp":
		return strconv.FormatInt(time.Now().Unix(), 10), true
	case "timestamp_ms":
		return strconv.FormatInt(time.Now().UnixMilli(), 10), true
	case "datetime":
		return time.Now().UTC().Format(time.RFC3339), true
	case "uuid":
		return uuid.NewString(), true
	case "random":
		return randomValue(arg)
	case "base64":
		return base64.StdEncoding.EncodeToString([]byte(arg)), true
	case "urlencode":
		return url.QueryEscape(arg), true
	}
	return "", false
}

// randomValue handles the three random forms: no arg, single N, and M-N.
// Invalid args fall back to the no-arg form so a typo in the template
// still yields a number rather than a stray literal.
func randomValue(arg string) (string, bool) {
	if arg == "" {
		return strconv.FormatUint(uint64(rand.Uint32()), 10), true
	}
	if a, b, ok := strings.Cut(arg, "-"); ok {
		lo, err1 := strconv.Atoi(strings.TrimSpace(a))
		hi, err2 := strconv.Atoi(strings.TrimSpace(b))
		if err1 != nil || err2 != nil || hi < lo {
			return strconv.FormatUint(uint64(rand.Uint32()), 10), true
		}
		span := hi - lo + 1
		return strconv.Itoa(lo + rand.Intn(span)), true
	}
	n, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil || n <= 0 {
		return strconv.FormatUint(uint64(rand.Uint32()), 10), true
	}
	return strconv.Itoa(rand.Intn(n)), true
}

