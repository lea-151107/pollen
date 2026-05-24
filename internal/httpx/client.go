package httpx

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lea/pollen/internal/history"
)

// MaxResponseBytes caps how much of a response body is read into memory. A
// dev tool that accidentally hits a large download endpoint shouldn't OOM the
// terminal; bytes beyond this cap are discarded and surfaced via Truncated.
const MaxResponseBytes = 32 * 1024 * 1024 // 32 MiB

// Do executes the given request and returns a Response.
func Do(req history.Request) (*history.Response, error) {
	body, contentType, err := buildBody(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(req.Method, req.URL, body)
	if err != nil {
		return nil, err
	}

	// Apply headers, with auto Content-Type fallback when omitted.
	hasContentType := false
	for _, h := range req.Headers {
		if h.Key == "" {
			continue
		}
		if strings.EqualFold(h.Key, "Content-Type") {
			hasContentType = true
		}
		httpReq.Header.Add(h.Key, h.Value)
	}
	if !hasContentType && contentType != "" {
		httpReq.Header.Set("Content-Type", contentType)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	start := time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Cap the body read so a runaway endpoint cannot exhaust memory.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	truncated := false
	if len(respBody) > MaxResponseBytes {
		respBody = respBody[:MaxResponseBytes]
		truncated = true
	}
	elapsed := time.Since(start)

	headers := make([]history.Header, 0, len(resp.Header))
	for k, vs := range resp.Header {
		for _, v := range vs {
			headers = append(headers, history.Header{Key: k, Value: v})
		}
	}

	respCT := resp.Header.Get("Content-Type")
	binary := IsBinary(respCT, respBody)
	bodyText := ""
	if !binary {
		bodyText = string(respBody)
	}

	return &history.Response{
		Status:      resp.StatusCode,
		StatusText:  resp.Status,
		Headers:     headers,
		Body:        bodyText,
		IsBinary:    binary,
		ContentType: ParseContentType(respCT),
		DurationMs:  elapsed.Milliseconds(),
		SizeBytes:   len(respBody),
		Truncated:   truncated,
		BodyBytes:   respBody,
	}, nil
}

func buildBody(req history.Request) (io.Reader, string, error) {
	if req.Body == "" {
		return nil, "", nil
	}
	switch req.BodyType {
	case history.BodyJSON:
		return strings.NewReader(req.Body), "application/json", nil
	case history.BodyForm:
		// Body is expected to be "key=value\nkey2=value2".
		form := url.Values{}
		for _, line := range strings.Split(req.Body, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			form.Add(strings.TrimSpace(k), strings.TrimSpace(v))
		}
		return strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", nil
	default:
		return strings.NewReader(req.Body), "text/plain", nil
	}
}
