package ui

import (
	"strings"
	"testing"
)

func TestHexDump_PNGSignature(t *testing.T) {
	b := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	got := HexDump(b, 0)
	want := "00000000  89 50 4e 47 0d 0a 1a 0a  00 00 00 0d              |.PNG........|\n"
	if got != want {
		t.Errorf("HexDump mismatch:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestHexDump_Truncation(t *testing.T) {
	b := make([]byte, 100)
	for i := range b {
		b[i] = byte(i)
	}
	got := HexDump(b, 32)
	if !strings.Contains(got, "truncated, showing 32 of 100 bytes") {
		t.Errorf("expected truncation marker, got:\n%s", got)
	}
	// Should have exactly 2 hex lines (32 bytes / 16 per line).
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 { // 2 dump + 1 truncation
		t.Errorf("expected 3 lines, got %d:\n%s", len(lines), got)
	}
}

func TestHexDump_Empty(t *testing.T) {
	if got := HexDump(nil, 0); got != "" {
		t.Errorf("empty input should yield empty string, got %q", got)
	}
}

func TestHexDump_AsciiPrintable(t *testing.T) {
	got := HexDump([]byte("Hello, World!"), 0)
	if !strings.Contains(got, "|Hello, World!|") {
		t.Errorf("expected ascii rendering in trailer, got:\n%s", got)
	}
}
