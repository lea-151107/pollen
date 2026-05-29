package intruder

import (
	"strings"
	"testing"

	"github.com/lea-151107/pollen/internal/history"
)

func TestApplyPayloads_URL(t *testing.T) {
	req := history.Request{URL: "https://api.example.com/users/{{$payload}}/posts"}
	out := ApplyPayloads(req, []string{"42"})
	if out.URL != "https://api.example.com/users/42/posts" {
		t.Errorf("URL: got %q", out.URL)
	}
}

func TestApplyPayloads_Body(t *testing.T) {
	req := history.Request{Body: `{"id":"{{$payload}}"}`}
	out := ApplyPayloads(req, []string{"abc"})
	if out.Body != `{"id":"abc"}` {
		t.Errorf("Body: got %q", out.Body)
	}
}

func TestApplyPayloads_Headers(t *testing.T) {
	req := history.Request{
		Headers: []history.Header{
			{Key: "X-{{$payload}}-Key", Value: "v"},
			{Key: "Authorization", Value: "Bearer {{$payload}}"},
		},
	}
	out := ApplyPayloads(req, []string{"abc"})
	if out.Headers[0].Key != "X-abc-Key" || out.Headers[0].Value != "v" {
		t.Errorf("header[0]: %+v", out.Headers[0])
	}
	if out.Headers[1].Key != "Authorization" || out.Headers[1].Value != "Bearer abc" {
		t.Errorf("header[1]: %+v", out.Headers[1])
	}
}

func TestApplyPayloads_DoesNotMutateInput(t *testing.T) {
	req := history.Request{Headers: []history.Header{{Key: "A", Value: "{{$payload}}"}}}
	out := ApplyPayloads(req, []string{"x"})
	out.Headers[0].Value = "tampered"
	if req.Headers[0].Value != "{{$payload}}" {
		t.Errorf("template header mutated: %q", req.Headers[0].Value)
	}
}

func TestApplyPayloads_MultipleOccurrencesInSameField(t *testing.T) {
	req := history.Request{URL: "/{{$payload}}/{{$payload}}"}
	out := ApplyPayloads(req, []string{"x"})
	if out.URL != "/x/x" {
		t.Errorf("URL: got %q", out.URL)
	}
}

func TestApplyPayloads_NumberedMarkers(t *testing.T) {
	req := history.Request{URL: "/u={{$payload1}}&p={{$payload2}}&r={{$payload3}}"}
	out := ApplyPayloads(req, []string{"alice", "secret", "X"})
	if out.URL != "/u=alice&p=secret&r=X" {
		t.Errorf("URL: got %q", out.URL)
	}
}

func TestApplyPayloads_LegacyMarkerAliasedToPosition1(t *testing.T) {
	// {{$payload}} and {{$payload1}} both map to payloads[0].
	req := history.Request{URL: "/a={{$payload}}&b={{$payload1}}"}
	out := ApplyPayloads(req, []string{"X"})
	if out.URL != "/a=X&b=X" {
		t.Errorf("URL: got %q", out.URL)
	}
}

func TestApplyPayloads_HighNumberDoesNotMatchLowerPrefix(t *testing.T) {
	// {{$payload10}} must NOT be substituted with payloads[0] followed by
	// the literal "0". The regex matches the whole token at once.
	req := history.Request{URL: "/{{$payload1}}|{{$payload10}}"}
	out := ApplyPayloads(req, []string{"one"}) // payload10 has no corresponding entry
	// payload1 substituted, payload10 left as-is (out of range).
	if out.URL != "/one|{{$payload10}}" {
		t.Errorf("URL: got %q", out.URL)
	}
}

func TestApplyPayloads_OutOfRangeLeftIntact(t *testing.T) {
	req := history.Request{URL: "/{{$payload1}}|{{$payload5}}"}
	out := ApplyPayloads(req, []string{"a"}) // only position 1 supplied
	if out.URL != "/a|{{$payload5}}" {
		t.Errorf("URL: got %q", out.URL)
	}
}

func TestPositionsUsed(t *testing.T) {
	req := history.Request{
		URL:  "/{{$payload2}}",
		Body: "{{$payload}}",
		Headers: []history.Header{
			{Key: "k", Value: "{{$payload3}}"},
			{Key: "{{$payload1}}", Value: "v"}, // alias of position 1
		},
	}
	got := PositionsUsed(req)
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("PositionsUsed: got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("PositionsUsed[%d]: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestHasMarkers_SniperAcceptsLegacyOrPayload1(t *testing.T) {
	if err := HasMarkers(history.Request{URL: "/{{$payload}}"}, Sniper, 1); err != nil {
		t.Errorf("legacy marker should be accepted by sniper: %v", err)
	}
	if err := HasMarkers(history.Request{URL: "/{{$payload1}}"}, Sniper, 1); err != nil {
		t.Errorf("{{$payload1}} should be accepted by sniper: %v", err)
	}
}

func TestHasMarkers_SniperRejectsMissingMarker(t *testing.T) {
	if err := HasMarkers(history.Request{URL: "/users"}, Sniper, 1); err == nil {
		t.Errorf("expected error for sniper without marker")
	}
}

func TestHasMarkers_SniperRejectsMultipleLists(t *testing.T) {
	if err := HasMarkers(history.Request{URL: "/{{$payload}}"}, Sniper, 2); err == nil {
		t.Errorf("expected error for sniper with 2 payload lists")
	}
}

func TestHasMarkers_PitchforkRequiresAllPositions(t *testing.T) {
	req := history.Request{URL: "/{{$payload1}}&p={{$payload3}}"}
	err := HasMarkers(req, Pitchfork, 3)
	if err == nil {
		t.Fatalf("expected error for pitchfork missing position 2")
	}
	if !strings.Contains(err.Error(), "{{$payload2}}") {
		t.Errorf("error should mention missing position 2; got %v", err)
	}
}

func TestHasMarkers_PitchforkAcceptsAllPositions(t *testing.T) {
	req := history.Request{URL: "/{{$payload1}}&{{$payload2}}&{{$payload3}}"}
	if err := HasMarkers(req, Pitchfork, 3); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHasMarkers_ClusterBombRequiresMin2Lists(t *testing.T) {
	if err := HasMarkers(history.Request{URL: "/{{$payload1}}"}, ClusterBomb, 1); err == nil {
		t.Errorf("expected error for ClusterBomb with 1 list")
	}
}
