package scenario

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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

// TestRun_DoesNotMutateTemplateHeaders guards against the header-aliasing bug:
// runStep must not expand {{...}} tokens into the caller's scenario in place,
// or a second run (or the editor) would see the stale, already-expanded values.
func TestRun_DoesNotMutateTemplateHeaders(t *testing.T) {
	srv := newChainServer()
	defer srv.Close()

	sc := Scenario{
		Name: "hdr",
		Steps: []Step{
			{Name: "login", Request: history.Request{Method: "POST", URL: srv.URL + "/login"}},
			{
				Name: "me",
				Request: history.Request{
					Method:  "GET",
					URL:     srv.URL + "/me",
					Headers: []history.Header{{Key: "Authorization", Value: "Bearer {{steps.login.body.token}}"}},
				},
				Assert: []Assertion{{Kind: AssertStatus, Op: OpEq, Want: "200"}},
			},
		},
	}

	const want = "Bearer {{steps.login.body.token}}"
	// Run twice; both must succeed and the template must survive unchanged.
	for i := 0; i < 2; i++ {
		results := Run(context.Background(), sc, RunOpts{})
		if results[1].Failed() {
			t.Fatalf("run %d: me step failed: status=%d asserts=%+v", i, results[1].Response.Status, results[1].Asserts)
		}
		if got := sc.Steps[1].Request.Headers[0].Value; got != want {
			t.Fatalf("run %d: template header was mutated: got %q want %q", i, got, want)
		}
	}
}

func TestExpandStepVars_UnknownLeftIntact(t *testing.T) {
	got := expandStepVars("{{steps.missing.body.x}}", map[string]*history.Response{})
	if got != "{{steps.missing.body.x}}" {
		t.Errorf("unknown step token should be left intact, got %q", got)
	}
}

// TestRunStream_CancelUnblocksProducer guards against a goroutine leak: when
// the consumer cancels and stops reading mid-run, the producer must not block
// forever on an unread send. It exploits the fact that a blocked send can be
// paired by a receive (ok==true) whereas a cleanly-exited producer closes the
// channel (ok==false).
func TestRunStream_CancelUnblocksProducer(t *testing.T) {
	release := make(chan struct{})
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) >= 2 {
			<-release // hold step 2 (and later) until the test lets it return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	sc := Scenario{
		Name: "cancel",
		Steps: []Step{
			{Name: "a", Request: history.Request{Method: "GET", URL: srv.URL}},
			{Name: "b", Request: history.Request{Method: "GET", URL: srv.URL}},
			{Name: "c", Request: history.Request{Method: "GET", URL: srv.URL}},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	ch := RunStream(ctx, sc, RunOpts{})

	<-ch           // consume step a's result; producer is now in step b (blocked)
	cancel()       // user cancels
	close(release) // let step b's HTTP call return so the producer reaches its send
	time.Sleep(300 * time.Millisecond)

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("producer sent after cancel with no reader — it was blocked (goroutine leak)")
		}
		// ok == false: channel closed → producer exited cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("channel neither closed nor readable after cancel (goroutine leak)")
	}
}
