// Package curlparse converts a `curl` shell command into a
// pollen history.Request. The supported flag set covers the
// arguments that show up in shared-by-engineer / Stack Overflow
// curl commands; the parser is line-noise-tolerant (handles
// backslash-continued lines, ' / " quoting, leading "curl ").
package curlparse

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/lea-151107/pollen/internal/history"
)

// Parse interprets command as a curl invocation and returns the
// corresponding pollen request. Only the most common flags are
// recognised (-X / -H / -d / --data-* / -F / -u / -A / -e
// / --cookie / -G); unsupported flags are surfaced as a parse
// error so the user knows to enter them by hand.
func Parse(command string) (history.Request, error) {
	tokens, err := tokenize(command)
	if err != nil {
		return history.Request{}, err
	}
	// Drop the optional leading "curl".
	if len(tokens) > 0 && tokens[0] == "curl" {
		tokens = tokens[1:]
	}
	if len(tokens) == 0 {
		return history.Request{}, fmt.Errorf("curlparse: empty command")
	}

	req := history.Request{Method: "GET"}
	var (
		rawBodyParts   []string
		formPairs      []string
		multipartParts []string
		methodSet      bool
		bodyImpliesPost bool
		useGet         bool
	)

	i := 0
	for i < len(tokens) {
		t := tokens[i]
		switch {
		case t == "-X" || t == "--request":
			i++
			if i >= len(tokens) {
				return history.Request{}, fmt.Errorf("curlparse: %s requires a value", t)
			}
			req.Method = strings.ToUpper(tokens[i])
			methodSet = true
		case t == "-H" || t == "--header":
			i++
			if i >= len(tokens) {
				return history.Request{}, fmt.Errorf("curlparse: %s requires a value", t)
			}
			k, v, ok := strings.Cut(tokens[i], ":")
			if !ok {
				return history.Request{}, fmt.Errorf("curlparse: malformed header %q", tokens[i])
			}
			req.Headers = append(req.Headers, history.Header{
				Key:   strings.TrimSpace(k),
				Value: strings.TrimSpace(v),
			})
		case t == "-d" || t == "--data" || t == "--data-raw" || t == "--data-binary":
			i++
			if i >= len(tokens) {
				return history.Request{}, fmt.Errorf("curlparse: %s requires a value", t)
			}
			val := tokens[i]
			// curl reads @<path> as file contents for -d/--data/--data-binary
			// (but --data-raw takes its value verbatim, including a leading @).
			// -d/--data additionally strip newlines from the file; --data-binary
			// posts it byte-for-byte.
			if t != "--data-raw" && strings.HasPrefix(val, "@") {
				data, err := os.ReadFile(val[1:])
				if err != nil {
					return history.Request{}, fmt.Errorf("curlparse: %s @%s: %w", t, val[1:], err)
				}
				val = string(data)
				if t != "--data-binary" {
					val = strings.ReplaceAll(val, "\r", "")
					val = strings.ReplaceAll(val, "\n", "")
				}
			}
			rawBodyParts = append(rawBodyParts, val)
			bodyImpliesPost = true
		case t == "--data-urlencode":
			i++
			if i >= len(tokens) {
				return history.Request{}, fmt.Errorf("curlparse: %s requires a value", t)
			}
			formPairs = append(formPairs, tokens[i])
			bodyImpliesPost = true
		case t == "-F" || t == "--form":
			i++
			if i >= len(tokens) {
				return history.Request{}, fmt.Errorf("curlparse: %s requires a value", t)
			}
			multipartParts = append(multipartParts, tokens[i])
			bodyImpliesPost = true
		case t == "-u" || t == "--user":
			i++
			if i >= len(tokens) {
				return history.Request{}, fmt.Errorf("curlparse: %s requires a value", t)
			}
			creds := tokens[i]
			req.Headers = append(req.Headers, history.Header{
				Key:   "Authorization",
				Value: "Basic " + base64.StdEncoding.EncodeToString([]byte(creds)),
			})
		case t == "-A" || t == "--user-agent":
			i++
			if i >= len(tokens) {
				return history.Request{}, fmt.Errorf("curlparse: %s requires a value", t)
			}
			req.Headers = append(req.Headers, history.Header{Key: "User-Agent", Value: tokens[i]})
		case t == "-e" || t == "--referer":
			i++
			if i >= len(tokens) {
				return history.Request{}, fmt.Errorf("curlparse: %s requires a value", t)
			}
			req.Headers = append(req.Headers, history.Header{Key: "Referer", Value: tokens[i]})
		case t == "--cookie" || t == "-b":
			i++
			if i >= len(tokens) {
				return history.Request{}, fmt.Errorf("curlparse: %s requires a value", t)
			}
			req.Headers = append(req.Headers, history.Header{Key: "Cookie", Value: tokens[i]})
		case t == "-G" || t == "--get":
			useGet = true
		case t == "-L" || t == "--location" || t == "-k" || t == "--insecure" ||
			t == "-s" || t == "--silent" || t == "-v" || t == "--verbose" ||
			t == "-i" || t == "--include":
			// Transport / logging flags pollen handles via its own
			// settings or globally — drop them silently rather than
			// erroring on every shared curl that includes -L.
		case looksLikeClumpedShortFlags(t):
			// curl accepts -sLv as a shorthand for -s -L -v. We
			// only split when every letter would be a transport flag
			// in the table above; anything else (e.g. -Hi where -H
			// expects a value) is too ambiguous to handle here.
		case strings.HasPrefix(t, "-"):
			return history.Request{}, fmt.Errorf("curlparse: unsupported flag %q", t)
		default:
			// Positional argument — treated as the URL. The last one
			// wins if there are multiple (rare, but matches cURL's
			// "first non-flag" intuition by accident).
			req.URL = t
		}
		i++
	}

	if req.URL == "" {
		return history.Request{}, fmt.Errorf("curlparse: no URL in command")
	}

	// Method-inference rules:
	//   -X explicit         → keep
	//   -G                  → force GET
	//   any data flag set   → POST (curl's own behaviour)
	//   none of the above   → GET
	switch {
	case useGet:
		req.Method = "GET"
	case methodSet:
		// already set above
	case bodyImpliesPost:
		req.Method = "POST"
	}

	// -G / --get moves any inline data onto the URL as a query string and sends
	// no body (curl's behaviour): `curl -G -d 'q=hi' URL` → `GET URL?q=hi`.
	if useGet {
		var q []string
		q = append(q, rawBodyParts...)
		q = append(q, formPairs...)
		if len(q) > 0 {
			sep := "?"
			if strings.Contains(req.URL, "?") {
				sep = "&"
			}
			req.URL += sep + strings.Join(q, "&")
		}
		return req, nil
	}

	// Body assembly: multipart wins over form-urlencoded wins over raw
	// (an unusual command setting more than one will surface as the
	// most specific kind; users very rarely mix them).
	switch {
	case len(multipartParts) > 0:
		req.BodyType = history.BodyMultipart
		req.Body = strings.Join(multipartFromFFlags(multipartParts), "\n")
	case len(formPairs) > 0:
		req.BodyType = history.BodyForm
		req.Body = strings.Join(formPairs, "\n")
	case len(rawBodyParts) > 0:
		req.Body = strings.Join(rawBodyParts, "&")
		// Default raw, but if Content-Type says JSON keep BodyJSON
		// so the editor opens the JSON tab.
		req.BodyType = history.BodyRaw
		hasContentType := false
		for _, h := range req.Headers {
			if strings.EqualFold(h.Key, "Content-Type") {
				hasContentType = true
				if strings.Contains(strings.ToLower(h.Value), "json") {
					req.BodyType = history.BodyJSON
				}
			}
		}
		// curl's -d/--data defaults the Content-Type to
		// application/x-www-form-urlencoded when the command gives none.
		// Without this pollen would auto-apply text/plain for a raw body,
		// changing the request semantics from what the curl command meant.
		if !hasContentType {
			req.Headers = append(req.Headers, history.Header{
				Key:   "Content-Type",
				Value: "application/x-www-form-urlencoded",
			})
		}
	}

	return req, nil
}

// looksLikeClumpedShortFlags reports whether t is a single token like
// "-sLv" composed of two or more characters from the transport / logging
// short-flag set. The check is intentionally narrow — letters that
// look like value-taking flags (X, H, d, F, ...) disqualify the
// token so we never silently consume a real flag.
func looksLikeClumpedShortFlags(t string) bool {
	if len(t) < 3 || t[0] != '-' || t[1] == '-' {
		return false
	}
	for _, c := range t[1:] {
		switch c {
		case 'L', 'k', 's', 'v', 'i':
			// valueless transport flag — fine
		default:
			return false
		}
	}
	return true
}

// multipartFromFFlags reformats curl's `-F` arguments into pollen's
// `name=value` / `name=@file[;type=ct]` DSL. The shapes are
// compatible — curl uses the same syntax — so the transformation is
// effectively a passthrough; we still split / rejoin to normalise.
func multipartFromFFlags(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		out = append(out, a)
	}
	return out
}
