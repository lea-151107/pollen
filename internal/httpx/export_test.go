package httpx

import (
	"strings"
	"testing"

	"github.com/lea-151107/pollen/internal/history"
)

func TestToCurl_GET(t *testing.T) {
	req := history.Request{
		Method: "GET",
		URL:    "https://example.com/api",
		Headers: []history.Header{
			{Key: "Accept", Value: "application/json"},
		},
	}
	got := ToCurl(req)
	want := "curl -X GET 'https://example.com/api' \\\n  -H 'Accept: application/json'"
	if got != want {
		t.Errorf("ToCurl GET mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestToCurl_POST_JSON(t *testing.T) {
	req := history.Request{
		Method:   "POST",
		URL:      "https://example.com/api",
		BodyType: history.BodyJSON,
		Body:     `{"name":"test"}`,
	}
	got := ToCurl(req)
	if !strings.Contains(got, "-X POST") {
		t.Errorf("expected -X POST, got: %s", got)
	}
	if !strings.Contains(got, "Content-Type: application/json") {
		t.Errorf("expected auto Content-Type, got: %s", got)
	}
	if !strings.Contains(got, `--data '{"name":"test"}'`) {
		t.Errorf("expected --data with body, got: %s", got)
	}
}

func TestToCurl_GraphQL(t *testing.T) {
	req := history.Request{
		Method:           "POST",
		URL:              "https://example.com/graphql",
		BodyType:         history.BodyGraphQL,
		Body:             "{ ping }",
		GraphQLVariables: `{"id": 1}`,
	}
	got := ToCurl(req)
	if !strings.Contains(got, "-X POST") {
		t.Errorf("expected -X POST, got: %s", got)
	}
	if !strings.Contains(got, "Content-Type: application/json") {
		t.Errorf("expected application/json content type, got: %s", got)
	}
	// The envelope must contain both "query" and "variables".
	if !strings.Contains(got, `"query":"{ ping }"`) {
		t.Errorf("envelope missing query, got: %s", got)
	}
	if !strings.Contains(got, `"variables":{"id":1}`) {
		t.Errorf("envelope missing variables, got: %s", got)
	}
}

func TestToCurl_QuoteEscape(t *testing.T) {
	req := history.Request{
		Method:   "POST",
		URL:      "https://example.com",
		BodyType: history.BodyRaw,
		Body:     "it's a test",
	}
	got := ToCurl(req)
	if !strings.Contains(got, `'it'\''s a test'`) {
		t.Errorf("expected escaped quote, got: %s", got)
	}
}

func TestToFetch_GET(t *testing.T) {
	req := history.Request{
		Method: "GET",
		URL:    "https://example.com/api",
		Headers: []history.Header{
			{Key: "Accept", Value: "application/json"},
		},
	}
	got := ToFetch(req)
	if !strings.HasPrefix(got, `fetch("https://example.com/api"`) {
		t.Errorf("expected fetch with URL, got: %s", got)
	}
	if !strings.Contains(got, `"method": "GET"`) {
		t.Errorf("expected method GET, got: %s", got)
	}
	if !strings.Contains(got, `"Accept": "application/json"`) {
		t.Errorf("expected Accept header, got: %s", got)
	}
}

func TestToFetch_POST_Form(t *testing.T) {
	req := history.Request{
		Method:   "POST",
		URL:      "https://example.com",
		BodyType: history.BodyForm,
		Body:     "name=test\nfoo=bar",
	}
	got := ToFetch(req)
	// JSON escapes & as &, and url.Values.Encode sorts keys.
	if !strings.Contains(got, `"body": "foo=bar&name=test"`) {
		t.Errorf("expected encoded form body, got: %s", got)
	}
	if !strings.Contains(got, "application/x-www-form-urlencoded") {
		t.Errorf("expected form content-type, got: %s", got)
	}
}
