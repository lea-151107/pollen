package curlparse

import "fmt"

// tokenize splits a curl command into shell-style tokens. It honours
// single quotes (everything literal inside), double quotes (backslash
// escapes only `"`, `\`, `$`, and newline — matching POSIX sh's most
// common cases), backslash-escaped characters outside quotes, and
// backslash-newline line continuations. This is *not* a full POSIX
// shell parser; it covers the slice of features that show up in
// shared curl commands.
func tokenize(s string) ([]string, error) {
	var (
		out      []string
		cur      []rune
		inSingle bool
		inDouble bool
		hasCur   bool // distinguishes "empty token (e.g. -d '')" from "no token"
	)
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
				continue
			}
			cur = append(cur, c)
			hasCur = true
		case inDouble:
			if c == '\\' && i+1 < len(r) {
				next := r[i+1]
				switch next {
				case '"', '\\', '$', '`':
					cur = append(cur, next)
					i++
					hasCur = true
					continue
				case '\n':
					i++ // swallow the newline
					continue
				}
				// Unknown escape: keep the backslash literally
				// (matches POSIX sh's "any other char" rule).
				cur = append(cur, c)
				hasCur = true
			} else if c == '"' {
				inDouble = false
				continue
			} else {
				cur = append(cur, c)
				hasCur = true
			}
		default:
			switch c {
			case '\'':
				inSingle = true
				hasCur = true
			case '"':
				inDouble = true
				hasCur = true
			case '\\':
				if i+1 < len(r) {
					next := r[i+1]
					if next == '\n' {
						i++
						continue
					}
					cur = append(cur, next)
					i++
					hasCur = true
				}
			case ' ', '\t', '\n', '\r':
				if hasCur {
					out = append(out, string(cur))
					cur = cur[:0]
					hasCur = false
				}
			default:
				cur = append(cur, c)
				hasCur = true
			}
		}
	}
	if inSingle {
		return nil, fmt.Errorf("curlparse: unterminated single quote")
	}
	if inDouble {
		return nil, fmt.Errorf("curlparse: unterminated double quote")
	}
	if hasCur {
		out = append(out, string(cur))
	}
	return out, nil
}
