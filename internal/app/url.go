package app

import (
	"net/url"
	"sort"
	"strings"

	"github.com/lea/pollen/internal/ui"
)

// composeURL merges the query parameters from the Query panel into the URL.
// Uses net/url when the URL is parseable; falls back to plain concatenation
// when the URL contains `{{var}}` tokens (env expansion happens later).
func composeURL(rawURL string, params []ui.Param) string {
	if len(params) == 0 {
		return rawURL
	}
	if !strings.Contains(rawURL, "{{") {
		if u, err := url.Parse(rawURL); err == nil {
			q := u.Query()
			for _, p := range params {
				q.Add(p.Key, p.Value)
			}
			u.RawQuery = q.Encode()
			return u.String()
		}
	}
	// Fallback: simple concat with proper escaping. {{...}} tokens stay intact.
	var b strings.Builder
	b.WriteString(rawURL)
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	for i, p := range params {
		if i == 0 {
			b.WriteString(sep)
		} else {
			b.WriteString("&")
		}
		b.WriteString(url.QueryEscape(p.Key))
		b.WriteString("=")
		b.WriteString(url.QueryEscape(p.Value))
	}
	return b.String()
}

// splitURL separates a full URL into the URL-without-query and a slice of
// query parameters, sorted by key for stable display order. If the URL can't
// be parsed (e.g. it contains {{var}} tokens) the full URL is returned as-is
// and no params are extracted.
func splitURL(rawURL string) (string, []ui.Param) {
	if strings.Contains(rawURL, "{{") {
		return rawURL, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.RawQuery == "" {
		return rawURL, nil
	}
	values := u.Query()
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var params []ui.Param
	for _, k := range keys {
		for _, v := range values[k] {
			params = append(params, ui.Param{Key: k, Value: v})
		}
	}
	u.RawQuery = ""
	return u.String(), params
}
