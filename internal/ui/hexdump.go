package ui

import (
	"fmt"
	"strings"
)

const DefaultHexDumpLimit = 4096

// HexDump renders a byte slice as a xxd-style dump, truncating to maxBytes
// (passing <= 0 uses DefaultHexDumpLimit). If the input is longer, a trailing
// "... (truncated, showing N of M bytes)" line is appended.
//
// Lives in the ui package because it's a presentation concern; the dump
// format doesn't depend on HTTP at all.
func HexDump(b []byte, maxBytes int) string {
	if maxBytes <= 0 {
		maxBytes = DefaultHexDumpLimit
	}
	total := len(b)
	shown := total
	truncated := false
	if shown > maxBytes {
		shown = maxBytes
		truncated = true
	}

	var sb strings.Builder
	for offset := 0; offset < shown; offset += 16 {
		end := offset + 16
		if end > shown {
			end = shown
		}
		line := b[offset:end]

		fmt.Fprintf(&sb, "%08x  ", offset)

		// 16 hex bytes, split into 2 groups of 8 with a gap.
		for i := 0; i < 16; i++ {
			if i == 8 {
				sb.WriteByte(' ')
			}
			if i < len(line) {
				fmt.Fprintf(&sb, "%02x ", line[i])
			} else {
				sb.WriteString("   ")
			}
		}

		sb.WriteString(" |")
		for _, c := range line {
			if c >= 0x20 && c < 0x7f {
				sb.WriteByte(c)
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteString("|\n")
	}

	if truncated {
		fmt.Fprintf(&sb, "... (truncated, showing %d of %d bytes)\n", shown, total)
	}
	return sb.String()
}
