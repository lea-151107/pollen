package httpx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lea-151107/pollen/internal/history"
)

// ToCurl renders the request as a POSIX-compatible cURL command.
func ToCurl(req history.Request) string {
	var b strings.Builder
	b.WriteString("curl -X ")
	b.WriteString(req.Method)
	b.WriteString(" ")
	b.WriteString(shellQuote(req.URL))

	for _, h := range req.Headers {
		if h.Key == "" {
			continue
		}
		b.WriteString(" \\\n  -H ")
		b.WriteString(shellQuote(h.Key + ": " + h.Value))
	}

	body, contentType, _ := buildBody(req)
	if body != nil {
		bodyStr := readerToString(req)
		// If Content-Type isn't set and we know one, add it.
		if !hasHeader(req.Headers, "Content-Type") && contentType != "" {
			b.WriteString(" \\\n  -H ")
			b.WriteString(shellQuote("Content-Type: " + contentType))
		}
		b.WriteString(" \\\n  --data ")
		b.WriteString(shellQuote(bodyStr))
	}

	return b.String()
}

// ToFetch renders the request as a JavaScript fetch() call.
func ToFetch(req history.Request) string {
	opts := map[string]any{
		"method": req.Method,
	}

	headers := map[string]string{}
	for _, h := range req.Headers {
		if h.Key == "" {
			continue
		}
		headers[h.Key] = h.Value
	}

	body, contentType, _ := buildBody(req)
	if body != nil {
		bodyStr := readerToString(req)
		if !hasHeader(req.Headers, "Content-Type") && contentType != "" {
			headers["Content-Type"] = contentType
		}
		opts["body"] = bodyStr
	}

	if len(headers) > 0 {
		opts["headers"] = headers
	}

	optsJSON := marshalIndentNoEscape(opts)
	urlJSON := marshalNoEscape(req.URL)
	return fmt.Sprintf("fetch(%s, %s)", urlJSON, optsJSON)
}

func marshalIndentNoEscape(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
	return strings.TrimRight(buf.String(), "\n")
}

func marshalNoEscape(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimRight(buf.String(), "\n")
}

func readerToString(req history.Request) string {
	switch req.BodyType {
	case history.BodyForm, history.BodyGraphQL:
		// Reformat through buildBody so the exported cURL / fetch
		// command sends exactly what pollen's runtime would send.
		// GraphQL especially needs the JSON envelope assembly.
		r, _, _ := buildBody(req)
		if r == nil {
			return ""
		}
		var sb strings.Builder
		buf := make([]byte, 1024)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		return sb.String()
	default:
		return req.Body
	}
}

func hasHeader(headers []history.Header, key string) bool {
	for _, h := range headers {
		if strings.EqualFold(h.Key, key) {
			return true
		}
	}
	return false
}

// shellQuote single-quotes a string for POSIX shells.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
