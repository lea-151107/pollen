package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lea/pollen/internal/history"
)

func TestPickFilename_ContentDisposition(t *testing.T) {
	resp := &history.Response{
		Headers: []history.Header{
			{Key: "Content-Disposition", Value: `attachment; filename="photo.png"`},
		},
	}
	got := pickFilename(resp, "https://example.com/api/whatever")
	if got != "photo.png" {
		t.Errorf("want photo.png, got %q", got)
	}
}

func TestPickFilename_URLPath(t *testing.T) {
	got := pickFilename(nil, "https://example.com/files/report.pdf")
	if got != "report.pdf" {
		t.Errorf("want report.pdf, got %q", got)
	}
}

func TestPickFilename_Fallback(t *testing.T) {
	cases := []string{
		"https://example.com",
		"https://example.com/",
		"not-a-url",
	}
	for _, in := range cases {
		got := pickFilename(nil, in)
		if got != "response.bin" && got != "not-a-url" {
			t.Errorf("%q: unexpected %q", in, got)
		}
	}
}

func TestPickFilename_SanitizePath(t *testing.T) {
	resp := &history.Response{
		Headers: []history.Header{
			{Key: "Content-Disposition", Value: `attachment; filename="../etc/passwd"`},
		},
	}
	got := pickFilename(resp, "")
	if strings.Contains(got, "..") || strings.Contains(got, "/") {
		t.Errorf("filename should be sanitized, got %q", got)
	}
}

// TestSanitizeFilename_StripsControlChars is the regression test for Bug K-3:
// a malicious Content-Disposition with embedded control chars (e.g. ANSI
// escapes) must not survive into the on-disk filename or the "saved to ..."
// status message that gets rendered to the terminal.
func TestSanitizeFilename_StripsControlChars(t *testing.T) {
	got := sanitizeFilename("evil\x1b[2J.txt")
	if strings.ContainsRune(got, 0x1b) {
		t.Errorf("output must not contain raw ESC, got %q", got)
	}
	for _, r := range got {
		if r < 0x20 || r == 0x7f {
			t.Errorf("output must not contain control chars, got %q (rune %U)", got, r)
		}
	}
}

func TestSanitizeFilename_PreservesNormal(t *testing.T) {
	in := "report.pdf"
	if got := sanitizeFilename(in); got != in {
		t.Errorf("normal name should pass through, got %q", got)
	}
}

func TestSanitizeFilename_EmptyAfterSanitize(t *testing.T) {
	// All-control input becomes underscores; trim doesn't strip underscores so
	// the fallback only triggers for truly-empty results — but let's verify.
	got := sanitizeFilename("")
	if got != "response.bin" {
		t.Errorf("empty input should fall back, got %q", got)
	}
}

func TestUniquePath_AvoidsOverwrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "photo.png"), []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "photo(2).png"), []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := uniquePath(dir, "photo.png")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "photo(3).png")
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestEnsureExtension(t *testing.T) {
	cases := []struct {
		name, ct, want string
	}{
		{"photo", "image/png", "photo.png"},
		{"data", "application/json", "data.json"},
		{"already.png", "image/png", "already.png"}, // keep existing
		{"plain", "", "plain"},                      // no CT
		{"x", "totally/unknown-mime-xyz123", "x"},   // unknown MIME
		{"y.bin", "", "y.bin"},                      // already has ext
	}
	for _, c := range cases {
		got := ensureExtension(c.name, c.ct)
		if got != c.want {
			t.Errorf("ensureExtension(%q, %q): got %q want %q", c.name, c.ct, got, c.want)
		}
	}
}

func TestSaveResponseBytes(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	bytes := []byte{0x89, 0x50, 0x4e, 0x47}
	resp := &history.Response{
		Headers: []history.Header{
			{Key: "Content-Disposition", Value: `attachment; filename="test.png"`},
		},
	}
	dest, err := saveResponseBytes(bytes, resp, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(bytes) {
		t.Errorf("content mismatch: got %x", got)
	}
}
