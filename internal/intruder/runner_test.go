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
		Payload:  PayloadConfig{Kind: PayloadRange, From: 1, To: 3},
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
		Payload:     PayloadConfig{Kind: PayloadRange, From: 1, To: 3},
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
		Payload:     PayloadConfig{Kind: PayloadRange, From: 1, To: 20},
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
		Payload:     PayloadConfig{Kind: PayloadRange, From: 1, To: 10000},
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
		Payload:     PayloadConfig{Kind: PayloadRange, From: 1, To: 3},
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
	doer := func(req history.Request) (*history.Response, error) {
		time.Sleep(10 * time.Millisecond)
		return &history.Response{Status: 200}, nil
	}
	ch, err := startWithDoer(ctx, RunConfig{
		Template:    makeTemplate(),
		Payload:     PayloadConfig{Kind: PayloadRange, From: 1, To: 10000},
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
		Payload:     PayloadConfig{Kind: PayloadRange, From: 1, To: 2},
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
