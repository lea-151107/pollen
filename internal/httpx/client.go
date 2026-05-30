// Package httpx executes HTTP requests with pollen-specific transport
// options (TLS skip, custom CA pool, proxy, redirect control, cookie jar,
// byte-capped responses) and builds the request body from pollen's
// BodyType variants.
package httpx

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
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
		} else {
			// Malformed proxy_url: the user clearly intended to send via
			// a specific proxy. Falling through to http.DefaultTransport's
			// ProxyFromEnvironment would silently route via $HTTP_PROXY
			// (or direct) instead, which is the opposite of what they
			// asked for. Force tr.Proxy = nil so the request goes direct
			// and the failure is at least loud (DNS / connect error) rather
			// than wrong-proxy.
			tr.Proxy = nil
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
	if req.BodyType == history.BodyGraphQL {
		return buildGraphQLBody(req)
	}
	if req.BodyType == history.BodyMultipart {
		return buildMultipartBody(req)
	}
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

// MultipartPart is one entry parsed from a BodyMultipart body string.
// Value is used when File is empty; otherwise File is read off disk
// and streamed into the form. ContentType is optional (file parts
// default to application/octet-stream).
type MultipartPart struct {
	Name        string
	Value       string
	File        string
	ContentType string
}

// ParseMultipartLines parses a BodyMultipart body string. Each non-blank
// line describes one part in the format:
//
//	name=value             text part
//	name=@/path/to/file    file part (defaults to application/octet-stream)
//	name=@/path;type=ct    file part with explicit content-type
//
// Leading/trailing whitespace on the line is trimmed; whitespace inside
// values is preserved (the user may want trailing spaces in a text
// value). Lines that don't contain `=` are skipped.
func ParseMultipartLines(body string) []MultipartPart {
	var out []MultipartPart
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		p := MultipartPart{Name: k}
		if strings.HasPrefix(v, "@") {
			rest := v[1:]
			// Optional ";type=ct" suffix is split off before the path.
			if i := strings.Index(rest, ";type="); i >= 0 {
				p.ContentType = strings.TrimSpace(rest[i+len(";type="):])
				rest = rest[:i]
			}
			p.File = strings.TrimSpace(rest)
		} else {
			p.Value = v
		}
		out = append(out, p)
	}
	return out
}

// buildMultipartBody assembles a multipart/form-data body from req.Body's
// line-based encoding. File parts stream their contents into the form
// writer; text parts are written as-is.
func buildMultipartBody(req history.Request) (io.Reader, string, error) {
	parts := ParseMultipartLines(req.Body)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, p := range parts {
		if p.File != "" {
			mh := textproto.MIMEHeader{}
			mh.Set("Content-Disposition",
				`form-data; name="`+escapeMimeQuoted(p.Name)+
					`"; filename="`+escapeMimeQuoted(filepath.Base(p.File))+`"`)
			ct := p.ContentType
			if ct == "" {
				ct = "application/octet-stream"
			}
			mh.Set("Content-Type", ct)
			fw, err := w.CreatePart(mh)
			if err != nil {
				return nil, "", err
			}
			f, err := os.Open(p.File)
			if err != nil {
				return nil, "", fmt.Errorf("multipart: open %s: %w", p.File, err)
			}
			if _, err := io.Copy(fw, f); err != nil {
				_ = f.Close()
				return nil, "", err
			}
			if err := f.Close(); err != nil {
				return nil, "", err
			}
		} else {
			if err := w.WriteField(p.Name, p.Value); err != nil {
				return nil, "", err
			}
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return &buf, w.FormDataContentType(), nil
}

// escapeMimeQuoted minimally escapes characters that would break a
// `name="..."` parameter inside a multipart Content-Disposition header.
// Production code typically copes with cleverer schemes (RFC 5987)
// but for pollen's API-test use case escaping " and \ is enough.
func escapeMimeQuoted(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return r.Replace(s)
}

// buildGraphQLBody assembles the JSON envelope GraphQL servers expect
// from req.Body (the query) and req.GraphQLVariables (a JSON object
// string). Invalid variables JSON is dropped silently — the server
// will surface that as an error on its own and pollen doesn't want to
// block the user from inspecting the response.
func buildGraphQLBody(req history.Request) (io.Reader, string, error) {
	envelope := map[string]any{"query": req.Body}
	if v := strings.TrimSpace(req.GraphQLVariables); v != "" {
		var parsed any
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			envelope["variables"] = parsed
		}
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, "", err
	}
	return bytes.NewReader(data), "application/json", nil
}
