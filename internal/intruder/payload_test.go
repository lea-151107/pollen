package intruder

import (
	"strings"
	"testing"
)

func collect(t *testing.T, cfg PayloadConfig) []string {
	t.Helper()
	it, err := NewIterator(cfg)
	if err != nil {
		t.Fatalf("NewIterator: %v", err)
	}
	var out []string
	for {
		v, ok := it.Next()
		if !ok {
			break
		}
		out = append(out, v)
	}
	return out
}

func TestPayloadRange(t *testing.T) {
	got := collect(t, PayloadConfig{Kind: PayloadRange, From: 1, To: 5})
	want := []string{"1", "2", "3", "4", "5"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("range 1..5: got %v, want %v", got, want)
	}
}

func TestPayloadRange_Step(t *testing.T) {
	got := collect(t, PayloadConfig{Kind: PayloadRange, From: 1, To: 9, Step: 2})
	want := []string{"1", "3", "5", "7", "9"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("range 1..9 step 2: got %v, want %v", got, want)
	}
}

func TestPayloadRange_InvalidBounds(t *testing.T) {
	_, err := NewIterator(PayloadConfig{Kind: PayloadRange, From: 5, To: 1})
	if err == nil {
		t.Errorf("expected error for from > to")
	}
}

func TestPayloadList(t *testing.T) {
	got := collect(t, PayloadConfig{Kind: PayloadList, Words: []string{"a", "b", "c"}})
	want := []string{"a", "b", "c"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("list: got %v, want %v", got, want)
	}
}

func TestPayloadList_SkipsBlankAndTrimsCR(t *testing.T) {
	// Mirrors the kind of input a wordlist file produces on Windows.
	got := collect(t, PayloadConfig{
		Kind:  PayloadList,
		Words: []string{"a\r", "", "b", "  ", "c"},
	})
	// Empty (after trim-right-\r) lines are skipped but trailing whitespace
	// inside an entry is preserved by design (users may want " " as a
	// payload).
	want := []string{"a", "b", "  ", "c"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("list: got %v, want %v", got, want)
	}
}

func TestPayloadList_EmptyErrors(t *testing.T) {
	_, err := NewIterator(PayloadConfig{Kind: PayloadList, Words: []string{"", "\r"}})
	if err == nil {
		t.Errorf("expected error for all-blank list")
	}
}

func TestPayloadBrute_LenOne(t *testing.T) {
	got := collect(t, PayloadConfig{
		Kind: PayloadBrute, Alphabet: "abc", MinLen: 1, MaxLen: 1,
	})
	want := []string{"a", "b", "c"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("brute abc len=1: got %v, want %v", got, want)
	}
}

func TestPayloadBrute_LenOneToTwo(t *testing.T) {
	got := collect(t, PayloadConfig{
		Kind: PayloadBrute, Alphabet: "ab", MinLen: 1, MaxLen: 2,
	})
	want := []string{"a", "b", "aa", "ab", "ba", "bb"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("brute ab len=1..2: got %v, want %v", got, want)
	}
}

func TestPayloadBrute_InvalidLengths(t *testing.T) {
	_, err := NewIterator(PayloadConfig{Kind: PayloadBrute, Alphabet: "ab", MinLen: 0, MaxLen: 2})
	if err == nil {
		t.Errorf("expected error for MinLen < 1")
	}
}

func TestPayloadCaseToggle(t *testing.T) {
	// "aB" has two ASCII letters → 2^2 = 4 permutations.
	got := collect(t, PayloadConfig{Kind: PayloadCaseToggle, Base: "aB"})
	// Bit 0 selects the first letter, bit 1 the second.
	want := []string{"ab", "Ab", "aB", "AB"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("case toggle aB: got %v, want %v", got, want)
	}
}

func TestPayloadCaseToggle_NonLettersUnchanged(t *testing.T) {
	got := collect(t, PayloadConfig{Kind: PayloadCaseToggle, Base: "a1B"})
	// 2 letters → 4 permutations; the '1' stays put in the middle.
	want := []string{"a1b", "A1b", "a1B", "A1B"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("case toggle a1B: got %v, want %v", got, want)
	}
}

func TestPayloadCaseToggle_TooManyLettersErrors(t *testing.T) {
	// 31 ASCII letters would make `1 << 31` exceed what we cap at; the
	// runtime cap protects against 2^L exploding past sane MaxRequests
	// values and against the shift becoming negative on smaller ints.
	base := strings.Repeat("a", 31)
	_, err := NewIterator(PayloadConfig{Kind: PayloadCaseToggle, Base: base})
	if err == nil {
		t.Errorf("expected error for case-toggle base with 31 letters")
	}
}

func TestPayloadCaseToggle_ThirtyLettersOk(t *testing.T) {
	// 30 letters is exactly at the cap and must still construct.
	base := strings.Repeat("a", 30)
	if _, err := NewIterator(PayloadConfig{Kind: PayloadCaseToggle, Base: base}); err != nil {
		t.Errorf("30-letter base should be allowed, got %v", err)
	}
}
