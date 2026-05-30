package intruder

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/httpx"
)

// httpDoer is the HTTP fan-out signature the runner depends on. Tests
// inject a stub here; production code wires it to httpx.Do via Start.
type httpDoer func(history.Request) (*history.Response, error)

// Start kicks off an Intruder run and returns a channel that receives
// Result values as workers complete each request. The channel closes
// when the run finishes — by payload exhaustion, MaxRequests, or ctx
// cancellation. Callers should range over the channel until it closes
// to learn when the run is done.
//
// Result.Index reflects send order, not completion order: workers may
// finish out-of-order under concurrency > 1.
func Start(ctx context.Context, cfg RunConfig) (<-chan Result, error) {
	return startWithDoer(ctx, cfg, httpx.Do)
}

// startWithDoer is the testable entry point. The production wrapper Start
// uses httpx.Do; tests swap in a stub that records concurrency and
// returns canned responses.
func startWithDoer(ctx context.Context, cfg RunConfig, do httpDoer) (<-chan Result, error) {
	if err := HasMarkers(cfg.Template, cfg.Mode, len(cfg.Payloads)); err != nil {
		return nil, err
	}
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 1
	}
	if cfg.MaxRequests < 1 {
		cfg.MaxRequests = 1000
	}
	if cfg.DelayMs < 0 {
		cfg.DelayMs = 0
	}

	vecIter, err := NewVectorIterator(cfg.Mode, cfg.Payloads)
	if err != nil {
		return nil, err
	}

	// payloads is unbuffered so the dispatcher gates on worker readiness;
	// this lets ctx cancellation drain quickly without piling up work.
	// results is buffered so a slow consumer doesn't stall a fast worker.
	payloads := make(chan vectorJob)
	results := make(chan Result, cfg.Concurrency*2)

	// Dispatcher: pull payload vectors from the iterator and feed workers.
	go func() {
		defer close(payloads)
		idx := 0
		for {
			if idx >= cfg.MaxRequests {
				return
			}
			vec, ok := vecIter.Next()
			if !ok {
				return
			}
			select {
			case <-ctx.Done():
				return
			case payloads <- vectorJob{Index: idx, Vector: vec}:
				idx++
			}
		}
	}()

	// Workers: each pulls jobs, builds the request, executes it, sends a
	// Result. delay_ms is applied between jobs within a single worker.
	var wg sync.WaitGroup
	wg.Add(cfg.Concurrency)
	for i := 0; i < cfg.Concurrency; i++ {
		go func() {
			defer wg.Done()
			first := true
			for job := range payloads {
				if !first && cfg.DelayMs > 0 {
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Duration(cfg.DelayMs) * time.Millisecond):
					}
				}
				first = false
				select {
				case <-ctx.Done():
					return
				default:
				}
				req := ApplyPayloads(cfg.Template, job.Vector)
				start := time.Now()
				resp, err := do(req)
				r := Result{
					Index:      job.Index,
					Payload:    joinVector(job.Vector),
					DurationMs: time.Since(start).Milliseconds(),
				}
				if err != nil {
					r.Error = err.Error()
				} else if resp != nil {
					r.Status = resp.Status
					r.StatusText = resp.StatusText
					r.Size = resp.SizeBytes
					r.ContentType = stripContentTypeParams(resp.ContentType)
					r.Response = capResponseBody(resp, cfg.ResponseBodyCap)
				}
				select {
				case <-ctx.Done():
					return
				case results <- r:
				}
			}
		}()
	}

	// Close the results channel once every worker has finished. This is
	// the channel-of-channels-of-channels pattern: callers detect "done"
	// by the close signal rather than by counting Result values.
	go func() {
		wg.Wait()
		close(results)
	}()

	return results, nil
}

// vectorJob carries one per-position payload vector along with the
// send-order index for the eventual Result row.
type vectorJob struct {
	Index  int
	Vector []string
}

// joinVector renders a payload vector as the display string used for
// the result-table row and CSV/JSON exports. For Sniper (length 1) the
// raw value is preserved unchanged so v1.2.x consumers see no diff;
// for Pitchfork / ClusterBomb positions are " | "-separated.
func joinVector(v []string) string {
	if len(v) == 1 {
		return v[0]
	}
	return strings.Join(v, " | ")
}

// capResponseBody returns a shallow copy of resp with its BodyBytes
// (and Body string) trimmed to cap bytes. cap == 0 means "no extra
// trimming". The returned pointer is safe to retain in Result even
// when the runner reuses the original *resp.
func capResponseBody(resp *history.Response, cap int) *history.Response {
	if resp == nil {
		return nil
	}
	out := *resp
	if cap <= 0 || len(out.BodyBytes) <= cap {
		return &out
	}
	out.BodyBytes = append([]byte(nil), out.BodyBytes[:cap]...)
	if !out.IsBinary {
		out.Body = string(out.BodyBytes)
	}
	out.Truncated = true
	return &out
}

// stripContentTypeParams returns just the media type, dropping any
// "; charset=..." style parameter. The result table is narrow, so
// trimming keeps rows aligned.
func stripContentTypeParams(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(ct)
}
