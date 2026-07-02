package scenario

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lea-151107/pollen/internal/history"
)

// newChainServer returns a server where /login hands out a token and /me
// echoes back whichever Authorization header it received, so a test can verify
// that a token extracted from step 1 flows into step 2.
func newChainServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"token":"abc123"}`))
	})
	mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer abc123" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"bad token"}`))
			return
		}
		w.Write([]byte(`{"name":"lea"}`))
	})
	return httptest.NewServer(mux)
}

func TestRun_ChainsStepOutput(t *testing.T) {
	srv := newChainServer()
	defer srv.Close()

	sc := Scenario{
		Name: "login flow",
		Steps: []Step{
			{
				Name:    "login",
				Request: history.Request{Method: "POST", URL: srv.URL + "/login"},
				Assert:  []Assertion{{Kind: AssertStatus, Op: OpEq, Want: "200"}},
			},
			{
				Name: "me",
				Request: history.Request{
					Method: "GET",
					URL:    srv.URL + "/me",
					Headers: []history.Header{
						{Key: "Authorization", Value: "Bearer {{steps.login.body.token}}"},
					},
				},
				Assert: []Assertion{
					{Kind: AssertStatus, Op: OpEq, Want: "200"},
					{Kind: AssertBody, Path: "name", Op: OpEq, Want: "lea"},
				},
			},
		},
	}

	results := Run(context.Background(), sc, RunOpts{})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	for _, r := range results {
		if r.Failed() {
			t.Errorf("step %q failed: err=%q asserts=%+v", r.Name, r.Err, r.Asserts)
		}
	}
}

func TestRun_PreviousStepViaResponseToken(t *testing.T) {
	srv := newChainServer()
	defer srv.Close()

	sc := Scenario{
		Name: "response token",
		Steps: []Step{
			{Name: "login", Request: history.Request{Method: "POST", URL: srv.URL + "/login"}},
			{
				Name: "me",
				Request: history.Request{
					Method:  "GET",
					URL:     srv.URL + "/me",
					Headers: []history.Header{{Key: "Authorization", Value: "Bearer {{response.body.token}}"}},
				},
				Assert: []Assertion{{Kind: AssertStatus, Op: OpEq, Want: "200"}},
			},
		},
	}
	results := Run(context.Background(), sc, RunOpts{})
	if results[1].Failed() {
		t.Fatalf("me step failed: status=%d asserts=%+v", results[1].Response.Status, results[1].Asserts)
	}
}

func TestRun_StopsAfterFailure(t *testing.T) {
	srv := newChainServer()
	defer srv.Close()

	sc := Scenario{
		Name: "abort",
		Steps: []Step{
			{
				Name:    "login",
				Request: history.Request{Method: "POST", URL: srv.URL + "/login"},
				Assert:  []Assertion{{Kind: AssertStatus, Op: OpEq, Want: "500"}}, // will fail
			},
			{Name: "me", Request: history.Request{Method: "GET", URL: srv.URL + "/me"}},
		},
	}
	results := Run(context.Background(), sc, RunOpts{})
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if !results[0].Failed() {
		t.Error("step 0 should have failed its status assertion")
	}
	if !results[1].Skipped {
		t.Error("step 1 should have been skipped after step 0 failed")
	}
}

func TestRun_ContainsAssertion(t *testing.T) {
	srv := newChainServer()
	defer srv.Close()

	sc := Scenario{
		Steps: []Step{{
			Name:    "login",
			Request: history.Request{Method: "POST", URL: srv.URL + "/login"},
			Assert:  []Assertion{{Kind: AssertBody, Op: OpContains, Want: "abc123"}},
		}},
	}
	results := Run(context.Background(), sc, RunOpts{})
	if results[0].Failed() {
		t.Errorf("contains assertion should pass, got %+v", results[0].Asserts)
	}
}

func TestExpandStepVars_UnknownLeftIntact(t *testing.T) {
	got := expandStepVars("{{steps.missing.body.x}}", map[string]*history.Response{})
	if got != "{{steps.missing.body.x}}" {
		t.Errorf("unknown step token should be left intact, got %q", got)
	}
}
