package intruder

import (
	"testing"

	"github.com/lea-151107/pollen/internal/history"
)

func TestApplyPayload_URL(t *testing.T) {
	req := history.Request{URL: "https://api.example.com/users/{{$payload}}/posts"}
	out := ApplyPayload(req, "42")
	if out.URL != "https://api.example.com/users/42/posts" {
		t.Errorf("URL: got %q", out.URL)
	}
}

func TestApplyPayload_Body(t *testing.T) {
	req := history.Request{Body: `{"id":"{{$payload}}"}`}
	out := ApplyPayload(req, "abc")
	if out.Body != `{"id":"abc"}` {
		t.Errorf("Body: got %q", out.Body)
	}
}

func TestApplyPayload_Headers(t *testing.T) {
	req := history.Request{
		Headers: []history.Header{
			{Key: "X-{{$payload}}-Key", Value: "v"},
			{Key: "Authorization", Value: "Bearer {{$payload}}"},
		},
	}
	out := ApplyPayload(req, "abc")
	if out.Headers[0].Key != "X-abc-Key" || out.Headers[0].Value != "v" {
		t.Errorf("header[0]: %+v", out.Headers[0])
	}
	if out.Headers[1].Key != "Authorization" || out.Headers[1].Value != "Bearer abc" {
		t.Errorf("header[1]: %+v", out.Headers[1])
	}
}

func TestApplyPayload_DoesNotMutateInput(t *testing.T) {
	// Header backing array must not be aliased; mutating the result must
	// not corrupt the original template the runner reuses for every
	// iteration.
	req := history.Request{Headers: []history.Header{{Key: "A", Value: "{{$payload}}"}}}
	out := ApplyPayload(req, "x")
	out.Headers[0].Value = "tampered"
	if req.Headers[0].Value != "{{$payload}}" {
		t.Errorf("template header mutated: %q", req.Headers[0].Value)
	}
}

func TestApplyPayload_MultipleOccurrencesInSameField(t *testing.T) {
	req := history.Request{URL: "/{{$payload}}/{{$payload}}"}
	out := ApplyPayload(req, "x")
	if out.URL != "/x/x" {
		t.Errorf("URL: got %q", out.URL)
	}
}

func TestHasMarker(t *testing.T) {
	cases := []struct {
		name string
		req  history.Request
		want bool
	}{
		{"none", history.Request{URL: "/users", Body: "{}"}, false},
		{"url", history.Request{URL: "/users/{{$payload}}"}, true},
		{"body", history.Request{Body: `{"x":"{{$payload}}"}`}, true},
		{"header value", history.Request{Headers: []history.Header{{Key: "k", Value: "{{$payload}}"}}}, true},
		{"header key", history.Request{Headers: []history.Header{{Key: "X-{{$payload}}", Value: "v"}}}, true},
	}
	for _, c := range cases {
		if got := HasMarker(c.req); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
