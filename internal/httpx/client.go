// Package httpx executes HTTP requests with pollen-specific transport
// options (TLS skip, custom CA pool, proxy, redirect control, cookie jar,
// byte-capped responses) and builds the request body from pollen's
// BodyType variants.
package httpx

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lea-151107/pollen/internal/history"
)

// SkipTLSVerify controls whether HTTP requests skip TLS certificate verification.
// Atomic so the UI can toggle it from any goroutine safely.
var SkipTLSVerify atomic.Bool

// MaxResponseBytes caps how much of a response body is read into memory. A
// dev tool that accidentally hits a large download endpoint shouldn't OOM the
// terminal; bytes beyond this cap are discarded and surfaced via Truncated.
var MaxResponseBytes = 32 * 1024 * 1024 // 32 MiB

// RequestTimeout is the HTTP client timeout applied to every request.
var RequestTimeout = 60 * time.Second

// ProxyURL, when non-empty, routes all requests through the given proxy.
var ProxyURL string

// CACertPool, when non-nil, is used as the trusted CA pool for TLS verification.
var CACertPool *x509.CertPool

// DisableRedirects, when true, prevents the HTTP client from following redirects.
var DisableRedirects bool

// CookieJar, when non-nil, stores and sends cookies across requests.
var CookieJar http.CookieJar

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

	// Always clone DefaultTransport so each request gets independent TLS/proxy
	// config without mutating the shared default.
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tlsCfg := &tls.Config{}
	if SkipTLSVerify.Load() {
		tlsCfg.InsecureSkipVerify = true //nolint:gosec // user-opt-in for self-signed testing
	}
	if CACertPool != nil {
		tlsCfg.RootCAs = CACertPool
	}
	tr.TLSClientConfig = tlsCfg
	if ProxyURL != "" {
		if u, err := url.Parse(ProxyURL); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	client := &http.Client{Timeout: RequestTimeout, Transport: tr, Jar: CookieJar}
	if DisableRedirects {
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	start := time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Cap the body read so a runaway endpoint cannot exhaust memory.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, int64(MaxResponseBytes)+1))
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
