package httpx

import "testing"

func TestIsBinary_TextContentTypes(t *testing.T) {
	cases := []string{
		"text/plain",
		"text/html; charset=utf-8",
		"application/json",
		"application/xml",
		"application/vnd.api+json",
		"application/atom+xml; charset=utf-8",
		"image/svg+xml",
	}
	body := []byte("ascii body with whitespace\n")
	for _, ct := range cases {
		if IsBinary(ct, body) {
			t.Errorf("%q expected text, got binary", ct)
		}
	}
}

func TestIsBinary_TextMIMEButInvalidUTF8(t *testing.T) {
	// text/plain with Latin-1 bytes that are not valid UTF-8 → downgrade to binary
	body := []byte{'h', 'i', 0xe9, 0xe0, 0xff} // é à ÿ in Latin-1
	if !IsBinary("text/plain", body) {
		t.Error("text/plain with invalid UTF-8 should be treated as binary")
	}
}

func TestIsBinary_BinaryContentTypes(t *testing.T) {
	cases := []string{
		"image/png",
		"image/jpeg",
		"application/pdf",
		"application/zip",
		"video/mp4",
		"audio/mpeg",
		"font/woff2",
	}
	body := []byte("hello world")
	for _, ct := range cases {
		if !IsBinary(ct, body) {
			t.Errorf("%q expected binary, got text", ct)
		}
	}
}

func TestIsBinary_OctetStreamSniffs(t *testing.T) {
	// octet-stream with text content → text
	if IsBinary("application/octet-stream", []byte("plain text body, no nulls")) {
		t.Error("octet-stream with text body should not be binary")
	}
	// octet-stream with NUL → binary
	if !IsBinary("application/octet-stream", []byte{0x00, 0x01, 0x02}) {
		t.Error("octet-stream with NUL should be binary")
	}
}

func TestIsBinary_MissingContentTypeSniff(t *testing.T) {
	if IsBinary("", []byte(`{"ok":true}`)) {
		t.Error("JSON-looking body with no Content-Type should sniff as text")
	}
	pngHeader := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	if !IsBinary("", pngHeader) {
		t.Error("PNG bytes with no Content-Type should sniff as binary")
	}
}

func TestIsBinary_EmptyBody(t *testing.T) {
	if IsBinary("", nil) {
		t.Error("empty body with no Content-Type should default to text")
	}
}

func TestParseContentType(t *testing.T) {
	cases := map[string]string{
		"":                                   "",
		"text/plain":                         "text/plain",
		"text/plain; charset=utf-8":          "text/plain",
		"Application/JSON":                   "application/json",
		"application/vnd.api+json; q=0.9":    "application/vnd.api+json",
		"garbage but no semicolon":           "garbage but no semicolon",
	}
	for in, want := range cases {
		if got := ParseContentType(in); got != want {
			t.Errorf("ParseContentType(%q): got %q want %q", in, got, want)
		}
	}
}
