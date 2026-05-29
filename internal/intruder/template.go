package intruder

import (
	"strings"

	"github.com/lea-151107/pollen/internal/history"
)

// ApplyPayload returns req with every occurrence of PayloadMarker
// ({{$payload}}) in the URL, body, and header keys/values replaced with
// payload. The input req is not mutated; Headers are deep-copied because
// strings.ReplaceAll is otherwise applied to shared backing data.
func ApplyPayload(req history.Request, payload string) history.Request {
	out := req
	out.URL = strings.ReplaceAll(req.URL, PayloadMarker, payload)
	out.Body = strings.ReplaceAll(req.Body, PayloadMarker, payload)
	if len(req.Headers) > 0 {
		out.Headers = make([]history.Header, len(req.Headers))
		for i, h := range req.Headers {
			out.Headers[i] = history.Header{
				Key:   strings.ReplaceAll(h.Key, PayloadMarker, payload),
				Value: strings.ReplaceAll(h.Value, PayloadMarker, payload),
			}
		}
	}
	return out
}

// HasMarker reports whether the request template contains at least one
// payload marker — used by the UI to refuse a run that wouldn't actually
// substitute anything.
func HasMarker(req history.Request) bool {
	if strings.Contains(req.URL, PayloadMarker) {
		return true
	}
	if strings.Contains(req.Body, PayloadMarker) {
		return true
	}
	for _, h := range req.Headers {
		if strings.Contains(h.Key, PayloadMarker) || strings.Contains(h.Value, PayloadMarker) {
			return true
		}
	}
	return false
}
