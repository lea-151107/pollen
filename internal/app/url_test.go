package app

import (
	"testing"

	"github.com/lea/pollen/internal/ui"
)

func TestComposeURL_NoParams(t *testing.T) {
	got := composeURL("https://example.com/api", nil)
	if got != "https://example.com/api" {
		t.Errorf("no params should passthrough, got %q", got)
	}
}

func TestComposeURL_AppendsWhenNoExistingQuery(t *testing.T) {
	got := composeURL("https://example.com/api", []ui.Param{
		{Key: "limit", Value: "10"},
		{Key: "q", Value: "go"},
	})
	// url.Values.Encode sorts keys alphabetically.
	want := "https://example.com/api?limit=10&q=go"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestComposeURL_StripsURLQueryWhenComponentHasParams(t *testing.T) {
	// When the query component has params, it is the authoritative source.
	// Any query string already in the URL bar is discarded to prevent doubling.
	got := composeURL("https://example.com/api?page=1", []ui.Param{
		{Key: "limit", Value: "10"},
	})
	want := "https://example.com/api?limit=10"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestComposeURL_EscapesSpecialChars(t *testing.T) {
	got := composeURL("https://example.com/api", []ui.Param{
		{Key: "q", Value: "hello world"},
	})
	want := "https://example.com/api?q=hello+world"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestComposeURL_FallbackForTemplateURL(t *testing.T) {
	// {{vars}} make the URL un-parseable; the fallback path uses concat.
	got := composeURL("{{baseUrl}}/users", []ui.Param{
		{Key: "limit", Value: "10"},
	})
	want := "{{baseUrl}}/users?limit=10"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestComposeURL_FallbackStripsURLQueryWhenComponentHasParams(t *testing.T) {
	// Same stripping behaviour for the {{var}} fallback path.
	got := composeURL("{{baseUrl}}/users?page=1", []ui.Param{
		{Key: "limit", Value: "10"},
	})
	want := "{{baseUrl}}/users?limit=10"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestSplitURL_NoQuery(t *testing.T) {
	url, params := splitURL("https://example.com/api")
	if url != "https://example.com/api" || params != nil {
		t.Errorf("got (%q, %v)", url, params)
	}
}

func TestSplitURL_WithQuery(t *testing.T) {
	url, params := splitURL("https://example.com/api?limit=10&q=go")
	if url != "https://example.com/api" {
		t.Errorf("url: got %q want %q", url, "https://example.com/api")
	}
	// Should be sorted by key.
	if len(params) != 2 || params[0].Key != "limit" || params[1].Key != "q" {
		t.Errorf("params: got %+v", params)
	}
}

func TestSplitURL_TemplateLeavesUntouched(t *testing.T) {
	url, params := splitURL("{{baseUrl}}/users?limit=10")
	if url != "{{baseUrl}}/users?limit=10" || params != nil {
		t.Errorf("template URL should not be split, got (%q, %v)", url, params)
	}
}

func TestComposeSplit_Roundtrip(t *testing.T) {
	base := "https://example.com/api"
	params := []ui.Param{
		{Key: "a", Value: "1"},
		{Key: "b", Value: "two"},
	}
	composed := composeURL(base, params)
	gotBase, gotParams := splitURL(composed)
	if gotBase != base {
		t.Errorf("base: got %q want %q", gotBase, base)
	}
	if len(gotParams) != 2 {
		t.Fatalf("params: got %d want 2", len(gotParams))
	}
	if gotParams[0].Key != "a" || gotParams[0].Value != "1" ||
		gotParams[1].Key != "b" || gotParams[1].Value != "two" {
		t.Errorf("roundtrip mismatch: %+v", gotParams)
	}
}
