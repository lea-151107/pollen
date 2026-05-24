package httpx

import (
	"mime"
	"strings"
	"unicode/utf8"
)

// ParseContentType returns the lowercased media type from a Content-Type header,
// stripping parameters like charset. Returns "" if the header is empty or invalid.
func ParseContentType(contentType string) string {
	if contentType == "" {
		return ""
	}
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		// Best-effort: take the part before ';'.
		return strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	}
	return strings.ToLower(mt)
}

// IsBinary reports whether the response body should be treated as binary.
// Content-Type is authoritative when it is clearly binary; for text MIMEs the
// body must additionally be valid UTF-8, otherwise it is downgraded to binary
// so that non-UTF-8 bytes (e.g. Latin-1) cannot corrupt the JSON history file.
func IsBinary(contentType string, body []byte) bool {
	mt := ParseContentType(contentType)

	if isTextMIME(mt) {
		// Text MIMEs occasionally carry non-UTF-8 encodings. Treat as binary
		// so we render hex and skip persistence rather than mangling bytes.
		if !utf8.Valid(body) {
			return true
		}
		return false
	}
	if isBinaryMIME(mt) {
		// application/octet-stream is often a lie — sniff to confirm.
		if mt == "application/octet-stream" {
			return sniffBinary(body)
		}
		return true
	}
	// Unknown / missing — sniff.
	return sniffBinary(body)
}

func isTextMIME(mt string) bool {
	if mt == "" {
		return false
	}
	if strings.HasPrefix(mt, "text/") {
		return true
	}
	switch mt {
	case "application/json",
		"application/xml",
		"application/javascript",
		"application/ecmascript",
		"application/yaml",
		"application/x-yaml",
		"application/x-www-form-urlencoded",
		"application/graphql",
		"application/ld+json",
		"application/problem+json",
		"image/svg+xml":
		return true
	}
	// Structured-syntax suffixes per RFC 6839 / 7807.
	if strings.HasSuffix(mt, "+json") ||
		strings.HasSuffix(mt, "+xml") ||
		strings.HasSuffix(mt, "+yaml") {
		return true
	}
	return false
}

func isBinaryMIME(mt string) bool {
	if mt == "" {
		return false
	}
	if strings.HasPrefix(mt, "image/") ||
		strings.HasPrefix(mt, "video/") ||
		strings.HasPrefix(mt, "audio/") ||
		strings.HasPrefix(mt, "font/") {
		return true
	}
	switch mt {
	case "application/octet-stream",
		"application/pdf",
		"application/zip",
		"application/gzip",
		"application/x-gzip",
		"application/x-tar",
		"application/x-7z-compressed",
		"application/x-bzip2",
		"application/x-protobuf",
		"application/wasm",
		"application/msword",
		"application/vnd.ms-excel":
		return true
	}
	return false
}

// sniffBinary inspects up to the first 512 bytes and reports binary if a NUL
// byte appears or if invalid UTF-8 / non-printable bytes exceed 30%.
func sniffBinary(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	n := len(body)
	if n > 512 {
		n = 512
	}
	sample := body[:n]
	for _, b := range sample {
		if b == 0x00 {
			return true
		}
	}
	if !utf8.Valid(sample) {
		return true
	}
	nonPrintable := 0
	for _, b := range sample {
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
			nonPrintable++
		}
	}
	return nonPrintable*100/len(sample) > 30
}
