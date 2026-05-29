package intruder

import (
	"fmt"
	"strings"
)

// PayloadIterator yields payload strings one at a time. Next returns the
// next payload and true; on exhaustion it returns "" and false. Iterators
// are not safe for concurrent use; the runner pulls from a single iterator
// in the dispatcher goroutine and fans payloads out via a channel.
type PayloadIterator interface {
	Next() (string, bool)
}

// NewIterator constructs the iterator for cfg.Kind. Returns an error when
// the config is structurally invalid (empty list, From > To, etc.).
func NewIterator(cfg PayloadConfig) (PayloadIterator, error) {
	switch cfg.Kind {
	case PayloadRange:
		step := cfg.Step
		if step == 0 {
			step = 1
		}
		if step < 0 {
			return nil, fmt.Errorf("range step must be >= 1")
		}
		if cfg.From > cfg.To {
			return nil, fmt.Errorf("range from > to")
		}
		return &rangeIter{next: cfg.From, to: cfg.To, step: step}, nil
	case PayloadList:
		words := make([]string, 0, len(cfg.Words))
		for _, w := range cfg.Words {
			w = strings.TrimRight(w, "\r")
			if w == "" {
				continue
			}
			words = append(words, w)
		}
		if len(words) == 0 {
			return nil, fmt.Errorf("list payload is empty")
		}
		return &listIter{words: words}, nil
	case PayloadBrute:
		if cfg.Alphabet == "" {
			return nil, fmt.Errorf("brute alphabet is empty")
		}
		if cfg.MinLen < 1 || cfg.MaxLen < cfg.MinLen {
			return nil, fmt.Errorf("brute lengths must satisfy 1 <= min <= max")
		}
		runes := []rune(cfg.Alphabet)
		return &bruteIter{
			alphabet: runes,
			minLen:   cfg.MinLen,
			maxLen:   cfg.MaxLen,
			curLen:   cfg.MinLen,
			indices:  make([]int, cfg.MinLen),
		}, nil
	case PayloadCaseToggle:
		if cfg.Base == "" {
			return nil, fmt.Errorf("case-toggle base is empty")
		}
		runes := []rune(cfg.Base)
		letterPositions := []int{}
		for i, r := range runes {
			if isASCIILetter(r) {
				letterPositions = append(letterPositions, i)
			}
		}
		// 2^L combinations are stored in an int; cap L to keep the shift
		// in range and the total bounded. 30 still allows ~1B permutations,
		// well past anything MaxRequests would let through.
		if len(letterPositions) > 30 {
			return nil, fmt.Errorf("case-toggle base has %d ASCII letters; max 30 (2^30 combinations)", len(letterPositions))
		}
		return &caseToggleIter{
			runes:     runes,
			positions: letterPositions,
			total:     1 << len(letterPositions),
		}, nil
	default:
		return nil, fmt.Errorf("unknown payload kind: %d", cfg.Kind)
	}
}

// rangeIter walks From..To stepping by Step (inclusive).
type rangeIter struct {
	next, to, step int
}

func (r *rangeIter) Next() (string, bool) {
	if r.next > r.to {
		return "", false
	}
	v := r.next
	r.next += r.step
	return fmt.Sprintf("%d", v), true
}

// listIter walks the words slice once.
type listIter struct {
	words []string
	i     int
}

func (l *listIter) Next() (string, bool) {
	if l.i >= len(l.words) {
		return "", false
	}
	w := l.words[l.i]
	l.i++
	return w, true
}

// bruteIter enumerates every alphabet combination from minLen to maxLen in
// lexicographic order. indices is a fixed-size positional counter; when
// every slot maxes out we step up to the next length.
type bruteIter struct {
	alphabet       []rune
	minLen, maxLen int
	curLen         int
	indices        []int
	started        bool
}

func (b *bruteIter) Next() (string, bool) {
	if b.curLen > b.maxLen {
		return "", false
	}
	if !b.started {
		b.started = true
		return b.current(), true
	}
	// Increment the rightmost position; carry over to longer lengths when
	// every slot wraps.
	n := len(b.alphabet)
	for i := len(b.indices) - 1; i >= 0; i-- {
		b.indices[i]++
		if b.indices[i] < n {
			return b.current(), true
		}
		b.indices[i] = 0
	}
	// Overflow → next length.
	b.curLen++
	if b.curLen > b.maxLen {
		return "", false
	}
	b.indices = make([]int, b.curLen)
	return b.current(), true
}

func (b *bruteIter) current() string {
	var sb strings.Builder
	sb.Grow(b.curLen)
	for _, idx := range b.indices {
		sb.WriteRune(b.alphabet[idx])
	}
	return sb.String()
}

// caseToggleIter enumerates 2^L permutations of the L ASCII letters in
// runes. Bit i of n selects upper (1) or lower (0) for the i-th letter.
type caseToggleIter struct {
	runes     []rune
	positions []int
	total     int
	n         int
}

func (c *caseToggleIter) Next() (string, bool) {
	if c.n >= c.total {
		return "", false
	}
	out := make([]rune, len(c.runes))
	copy(out, c.runes)
	for bit, pos := range c.positions {
		if c.n&(1<<bit) != 0 {
			out[pos] = toUpperASCII(out[pos])
		} else {
			out[pos] = toLowerASCII(out[pos])
		}
	}
	c.n++
	return string(out), true
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func toUpperASCII(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - ('a' - 'A')
	}
	return r
}

func toLowerASCII(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}
