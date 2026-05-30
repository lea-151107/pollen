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

	// Multipart bodies use -F flags rather than --data so the cURL
	// command actually performs file uploads. cURL builds the
	// Content-Type with the right boundary itself.
	if req.BodyType == history.BodyMultipart {
		for _, p := range ParseMultipartLines(req.Body) {
			arg := p.Name + "="
			if p.File != "" {
				arg += "@" + p.File
				if p.ContentType != "" {
					arg += ";type=" + p.ContentType
				}
			} else {
				arg += p.Value
			}
			b.WriteString(" \\\n  -F ")
			b.WriteString(shellQuote(arg))
		}
		return b.String()
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
//
// Multipart bodies render as an IIFE that builds a FormData object;
// file parts emit a `/* File from <path> */` placeholder because
// the browser fetch() API needs a real File handle, which the user
// must wire up to a file input in their final code.
func ToFetch(req history.Request) string {
	if req.BodyType == history.BodyMultipart {
		return toFetchMultipart(req)
	}
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

func toFetchMultipart(req history.Request) string {
	var b strings.Builder
	urlJSON := marshalNoEscape(req.URL)
	b.WriteString("fetch(")
	b.WriteString(urlJSON)
	b.WriteString(", {\n  method: ")
	b.WriteString(marshalNoEscape(req.Method))
	if len(req.Headers) > 0 {
		// Content-Type is intentionally omitted — the browser sets
		// it from the FormData object so the right boundary is used.
		headers := map[string]string{}
		for _, h := range req.Headers {
			if h.Key == "" || strings.EqualFold(h.Key, "Content-Type") {
				continue
			}
			headers[h.Key] = h.Value
		}
		if len(headers) > 0 {
			b.WriteString(",\n  headers: ")
			b.WriteString(marshalIndentNoEscape(headers))
		}
	}
	b.WriteString(",\n  body: (() => {\n    const fd = new FormData();\n")
	for _, p := range ParseMultipartLines(req.Body) {
		if p.File != "" {
			b.WriteString("    // For ")
			b.WriteString(p.Name)
			b.WriteString(", attach a File from a real <input> — pollen\n")
			b.WriteString("    // can't materialise a Blob from the literal path:\n    // ")
			b.WriteString(p.File)
			if p.ContentType != "" {
				b.WriteString("  (")
				b.WriteString(p.ContentType)
				b.WriteString(")")
			}
			b.WriteString("\n    fd.append(")
			b.WriteString(marshalNoEscape(p.Name))
			b.WriteString(", /* File */ null);\n")
		} else {
			b.WriteString("    fd.append(")
			b.WriteString(marshalNoEscape(p.Name))
			b.WriteString(", ")
			b.WriteString(marshalNoEscape(p.Value))
			b.WriteString(");\n")
		}
	}
	b.WriteString("    return fd;\n  })()\n})")
	return b.String()
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
	// Multipart is special-cased in ToCurl / ToFetch (it emits -F or
	// FormData rather than reading the buffer back out), so this
	// helper never sees BodyMultipart in practice.
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
