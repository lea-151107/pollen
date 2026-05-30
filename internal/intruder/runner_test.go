package intruder

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lea-151107/pollen/internal/history"
)

func makeTemplate() history.Request {
	return history.Request{
		Method: "GET",
		URL:    "https://api.example.com/users/{{$payload}}",
	}
}

func TestStart_RejectsTemplateWithoutMarker(t *testing.T) {
	ctx := context.Background()
	_, err := startWithDoer(ctx, RunConfig{
		Template: history.Request{URL: "/no-marker"},
		Payloads: []PayloadConfig{{Kind: PayloadRange, From: 1, To: 3}},
	}, fakeDoerOK)
	if err == nil {
		t.Errorf("expected error for template without {{$payload}}")
	}
}

func TestStart_RangeFiresAllPayloads(t *testing.T) {
	ctx := context.Background()
	var seen sync.Map
	doer := func(req history.Request) (*history.Response, error) {
		seen.Store(req.URL, true)
		return &history.Response{Status: 200, StatusText: "200 OK", SizeBytes: 5}, nil
	}
	ch, err := startWithDoer(ctx, RunConfig{
		Template:    makeTemplate(),
		Payloads:    []PayloadConfig{{Kind: PayloadRange, From: 1, To: 3}},
		Concurrency: 1,
	}, doer)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	var results []Result
	for r := range ch {
		results = append(results, r)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	for _, want := range []string{"https://api.example.com/users/1",
		"https://api.example.com/users/2",
		"https://api.example.com/users/3"} {
		if _, ok := seen.Load(want); !ok {
			t.Errorf("payload not sent: %s", want)
		}
	}
}

func TestStart_ConcurrencyCap(t *testing.T) {
	// Workers should never exceed the configured concurrency. Use atomic
	// counters and a doer that blocks long enough to stack workers up.
	ctx := context.Background()
	var inFlight, peak int32
	doer := func(req history.Request) (*history.Response, error) {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			cur := atomic.LoadInt32(&peak)
			if n <= cur || atomic.CompareAndSwapInt32(&peak, cur, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		return &history.Response{Status: 200}, nil
	}
	ch, err := startWithDoer(ctx, RunConfig{
		Template:    makeTemplate(),
		Payloads:    []PayloadConfig{{Kind: PayloadRange, From: 1, To: 20}},
		Concurrency: 3,
	}, doer)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	for range ch {
	}
	if peak > 3 {
		t.Errorf("peak in-flight %d exceeded concurrency cap 3", peak)
	}
	if peak < 2 {
		t.Errorf("peak in-flight %d looks too low; expected to actually parallelise", peak)
	}
}

func TestStart_MaxRequestsStops(t *testing.T) {
	ctx := context.Background()
	var count int32
	doer := func(req history.Request) (*history.Response, error) {
		atomic.AddInt32(&count, 1)
		return &history.Response{Status: 200}, nil
	}
	ch, err := startWithDoer(ctx, RunConfig{
		Template:    makeTemplate(),
		Payloads:    []PayloadConfig{{Kind: PayloadRange, From: 1, To: 10000}},
		Concurrency: 4,
		MaxRequests: 7,
	}, doer)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	for range ch {
	}
	if got := atomic.LoadInt32(&count); got != 7 {
		t.Errorf("expected exactly 7 requests, got %d", got)
	}
}

func TestStart_DelayAppliesBetweenWorkerJobs(t *testing.T) {
	ctx := context.Background()
	doer := func(req history.Request) (*history.Response, error) {
		return &history.Response{Status: 200}, nil
	}
	start := time.Now()
	ch, err := startWithDoer(ctx, RunConfig{
		Template:    makeTemplate(),
		Payloads:    []PayloadConfig{{Kind: PayloadRange, From: 1, To: 3}},
		Concurrency: 1,
		DelayMs:     30,
	}, doer)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	for range ch {
	}
	elapsed := time.Since(start)
	// Two delays between three requests in a single worker → at least 60ms.
	if elapsed < 60*time.Millisecond {
		t.Errorf("delay not applied: total %v", elapsed)
	}
}

func TestStart_CancelStopsRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // guard against early returns; the loop below also calls it.
	doer := func(req history.Request) (*history.Response, error) {
		time.Sleep(10 * time.Millisecond)
		return &history.Response{Status: 200}, nil
	}
	ch, err := startWithDoer(ctx, RunConfig{
		Template:    makeTemplate(),
		Payloads:    []PayloadConfig{{Kind: PayloadRange, From: 1, To: 10000}},
		Concurrency: 2,
		MaxRequests: 10000,
	}, doer)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Let a few results through, then cancel and confirm the channel
	// closes promptly.
	var n int
	for r := range ch {
		_ = r
		n++
		if n == 5 {
			cancel()
		}
	}
	// If cancel didn't shut things down, the test would block forever; if
	// the range loop returns we already proved the channel closed.
	if n < 5 {
		t.Errorf("expected at least 5 results before cancellation, got %d", n)
	}
}

func TestStart_RecordsErrorResponses(t *testing.T) {
	ctx := context.Background()
	doer := func(req history.Request) (*history.Response, error) {
		return nil, fmt.Errorf("simulated network error")
	}
	ch, err := startWithDoer(ctx, RunConfig{
		Template:    makeTemplate(),
		Payloads:    []PayloadConfig{{Kind: PayloadRange, From: 1, To: 2}},
		Concurrency: 1,
	}, doer)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	for r := range ch {
		if r.Error == "" {
			t.Errorf("expected non-empty Error on failed request, got %+v", r)
		}
	}
}

func fakeDoerOK(req history.Request) (*history.Response, error) {
	return &history.Response{Status: 200, StatusText: "200 OK"}, nil
}

func TestStart_StoresResponseBodyWithinCap(t *testing.T) {
	ctx := context.Background()
	// 1 KiB body, run with cap = 256 bytes.
	bigBody := make([]byte, 1024)
	for i := range bigBody {
		bigBody[i] = 'A'
	}
	doer := func(req history.Request) (*history.Response, error) {
		return &history.Response{
			Status:    200,
			StatusText: "200 OK",
			Body:      string(bigBody),
			BodyBytes: bigBody,
			SizeBytes: 1024,
		}, nil
	}
	ch, err := startWithDoer(ctx, RunConfig{
		Template:        makeTemplate(),
		Payloads:        []PayloadConfig{{Kind: PayloadRange, From: 1, To: 1}},
		Concurrency:     1,
		ResponseBodyCap: 256,
	}, doer)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	var results []Result
	for r := range ch {
		results = append(results, r)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Response == nil {
		t.Fatalf("Response should be stored")
	}
	if len(r.Response.BodyBytes) != 256 {
		t.Errorf("BodyBytes should be capped to 256, got %d", len(r.Response.BodyBytes))
	}
	if !r.Response.Truncated {
		t.Errorf("Response.Truncated should be true after cap")
	}
	// Original response struct from the doer must not be mutated.
	if len(bigBody) != 1024 {
		t.Errorf("the original body slice was mutated: len=%d", len(bigBody))
	}
}

func TestStart_NoBodyCapKeepsFullResponse(t *testing.T) {
	ctx := context.Background()
	body := []byte("hello world")
	doer := func(req history.Request) (*history.Response, error) {
		return &history.Response{
			Status:    200,
			StatusText: "200 OK",
			Body:      string(body),
			BodyBytes: body,
			SizeBytes: len(body),
		}, nil
	}
	ch, err := startWithDoer(ctx, RunConfig{
		Template:    makeTemplate(),
		Payloads:    []PayloadConfig{{Kind: PayloadRange, From: 1, To: 1}},
		Concurrency: 1,
		// ResponseBodyCap defaults to 0 → no extra cap.
	}, doer)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	for r := range ch {
		if r.Response == nil || string(r.Response.BodyBytes) != "hello world" {
			t.Errorf("expected full body retained, got %+v", r.Response)
		}
	}
}

func TestStart_PitchforkRunsZipped(t *testing.T) {
	ctx := context.Background()
	var seen sync.Map
	doer := func(req history.Request) (*history.Response, error) {
		seen.Store(req.URL, true)
		return &history.Response{Status: 200, StatusText: "200 OK"}, nil
	}
	ch, err := startWithDoer(ctx, RunConfig{
		Mode:     Pitchfork,
		Template: history.Request{URL: "/u={{$payload1}}&p={{$payload2}}"},
		Payloads: []PayloadConfig{
			{Kind: PayloadList, Words: []string{"alice", "bob", "carol"}},
			{Kind: PayloadList, Words: []string{"p1", "p2", "p3"}},
		},
		Concurrency: 1,
	}, doer)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	var results []Result
	for r := range ch {
		results = append(results, r)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 zipped results, got %d", len(results))
	}
	for _, want := range []string{
		"/u=alice&p=p1",
		"/u=bob&p=p2",
		"/u=carol&p=p3",
	} {
		if _, ok := seen.Load(want); !ok {
			t.Errorf("missing URL: %s", want)
		}
	}
	// Result.Payload should reflect the joined vector.
	wantPayloads := map[string]bool{
		"alice | p1": false,
		"bob | p2":   false,
		"carol | p3": false,
	}
	for _, r := range results {
		if _, ok := wantPayloads[r.Payload]; !ok {
			t.Errorf("unexpected joined payload: %q", r.Payload)
		}
		wantPayloads[r.Payload] = true
	}
	for k, v := range wantPayloads {
		if !v {
			t.Errorf("missing joined payload: %s", k)
		}
	}
}

func TestStart_ClusterBombRunsProduct(t *testing.T) {
	ctx := context.Background()
	var seen sync.Map
	doer := func(req history.Request) (*history.Response, error) {
		seen.Store(req.URL, true)
		return &history.Response{Status: 200, StatusText: "200 OK"}, nil
	}
	ch, err := startWithDoer(ctx, RunConfig{
		Mode:     ClusterBomb,
		Template: history.Request{URL: "/u={{$payload1}}&p={{$payload2}}"},
		Payloads: []PayloadConfig{
			{Kind: PayloadList, Words: []string{"alice", "bob"}},
			{Kind: PayloadList, Words: []string{"p1", "p2"}},
		},
		Concurrency: 1,
	}, doer)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	var results []Result
	for r := range ch {
		results = append(results, r)
	}
	if len(results) != 4 {
		t.Fatalf("expected 2*2=4 product results, got %d", len(results))
	}
	for _, want := range []string{
		"/u=alice&p=p1", "/u=alice&p=p2",
		"/u=bob&p=p1", "/u=bob&p=p2",
	} {
		if _, ok := seen.Load(want); !ok {
			t.Errorf("missing URL: %s", want)
		}
	}
}

func TestStart_ClusterBombRespectsMaxRequests(t *testing.T) {
	ctx := context.Background()
	var count int32
	doer := func(req history.Request) (*history.Response, error) {
		atomic.AddInt32(&count, 1)
		return &history.Response{Status: 200, StatusText: "200 OK"}, nil
	}
	ch, err := startWithDoer(ctx, RunConfig{
		Mode:     ClusterBomb,
		Template: history.Request{URL: "/u={{$payload1}}&p={{$payload2}}"},
		Payloads: []PayloadConfig{
			{Kind: PayloadRange, From: 1, To: 100},
			{Kind: PayloadRange, From: 1, To: 100}, // 10,000 product
		},
		Concurrency: 4,
		MaxRequests: 25,
	}, doer)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	for range ch {
	}
	got := atomic.LoadInt32(&count)
	if got > 25 {
		t.Errorf("MaxRequests not enforced: got %d > 25", got)
	}
}

func TestStart_SniperRejectsTemplateWithExtraPosition(t *testing.T) {
	// Verify the runner's pre-flight HasMarkers call catches the
	// out-of-range marker case end-to-end — the v1.3.0 bug would
	// have let this through and silently dispatched a request with
	// {{$payload2}} as a URL literal.
	ctx := context.Background()
	_, err := startWithDoer(ctx, RunConfig{
		Mode:        Sniper,
		Template:    history.Request{URL: "/{{$payload}}/{{$payload2}}"},
		Payloads:    []PayloadConfig{{Kind: PayloadList, Words: []string{"a"}}},
		Concurrency: 1,
	}, fakeDoerOK)
	if err == nil {
		t.Errorf("expected error for sniper template referencing position 2")
	}
}

func TestStart_PitchforkRejectsMissingMarker(t *testing.T) {
	ctx := context.Background()
	_, err := startWithDoer(ctx, RunConfig{
		Mode:     Pitchfork,
		Template: history.Request{URL: "/u={{$payload1}}"}, // missing payload2
		Payloads: []PayloadConfig{
			{Kind: PayloadList, Words: []string{"a"}},
			{Kind: PayloadList, Words: []string{"b"}},
		},
		Concurrency: 1,
	}, fakeDoerOK)
	if err == nil {
		t.Errorf("expected error for pitchfork with missing marker")
	}
}
