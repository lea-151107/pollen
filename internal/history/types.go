// Package history persists the request/response history in
// ~/.config/pollen/history.json and defines the Request, Response, and
// Header types shared with other packages.
package history

import "time"

type BodyType string

const (
	BodyJSON BodyType = "json"
	BodyForm BodyType = "form"
	BodyRaw  BodyType = "raw"
)

type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Request struct {
	Method   string   `json:"method"`
	URL      string   `json:"url"`
	Headers  []Header `json:"headers"`
	Body     string   `json:"body"`
	BodyType BodyType `json:"body_type"`
}

type Response struct {
	Status      int      `json:"status"`
	StatusText  string   `json:"status_text"`
	Headers     []Header `json:"headers"`
	Body        string   `json:"body"`                   // text bodies only
	IsBinary    bool     `json:"is_binary,omitempty"`    // true when body was binary
	ContentType string   `json:"content_type,omitempty"` // canonical media type, no params
	DurationMs  int64    `json:"duration_ms"`
	SizeBytes   int      `json:"size_bytes"` // bytes retained (after truncation)
	Truncated   bool     `json:"truncated,omitempty"`

	// BodyBytes holds the raw response bytes for the current session only.
	// Persisted history never carries this field, so reloaded entries cannot
	// reproduce binary content.
	BodyBytes []byte `json:"-"`
}

type Entry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Request   Request   `json:"request"`
	Response  *Response `json:"response,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type File struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}
