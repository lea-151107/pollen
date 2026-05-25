package ui

import (
	"strings"
	"testing"
)

func TestTryPrettyJSON_ValidObject(t *testing.T) {
	got, ok := tryPrettyJSON(`{"a":1,"b":[2,3]}`)
	if !ok {
		t.Fatal("valid JSON should be pretty-printed")
	}
	want := "{\n  \"a\": 1,\n  \"b\": [\n    2,\n    3\n  ]\n}"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestTryPrettyJSON_InvalidPassthrough(t *testing.T) {
	in := "not json"
	got, ok := tryPrettyJSON(in)
	if ok {
		t.Errorf("invalid JSON should return ok=false")
	}
	if got != in {
		t.Errorf("invalid input should be returned unchanged, got %q", got)
	}
}

func TestTryPrettyJSON_AlreadyFormatted(t *testing.T) {
	// json.Indent normalizes whitespace regardless of input formatting.
	got, ok := tryPrettyJSON("{\n  \"a\":  1  \n}")
	if !ok {
		t.Fatal("should still be valid")
	}
	if !strings.Contains(got, `"a": 1`) {
		t.Errorf("expected normalized indent, got %q", got)
	}
}

func TestIsJSONContentType(t *testing.T) {
	cases := map[string]bool{
		"application/json":         true,
		"application/vnd.api+json": true,
		"application/ld+json":      true,
		"application/problem+json": true,
		"text/json":                false, // non-standard, not matched
		"application/xml":          false,
		"":                         false,
		"application/jsonl":        false, // not strictly JSON
	}
	for ct, want := range cases {
		if got := isJSONContentType(ct); got != want {
			t.Errorf("isJSONContentType(%q): got %v want %v", ct, got, want)
		}
	}
}
