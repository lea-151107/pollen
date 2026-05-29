package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea-151107/pollen/internal/intruder"
)

// sampleResults builds a deterministic mix of statuses/sizes/durations
// for the sort and filter tests. Keep indices in send order so the
// default view (sortIndex, asc) matches the slice order.
func sampleResults() []intruder.Result {
	return []intruder.Result{
		{Index: 0, Payload: "admin", Status: 200, Size: 100, DurationMs: 30, ContentType: "text/html"},
		{Index: 1, Payload: "root", Status: 401, Size: 50, DurationMs: 20, ContentType: "application/json"},
		{Index: 2, Payload: "guest", Status: 200, Size: 200, DurationMs: 10, ContentType: "text/html"},
		{Index: 3, Payload: "test", Status: 500, Size: 80, DurationMs: 40, ContentType: "text/plain"},
		{Index: 4, Payload: "adminuser", Status: 404, Size: 60, DurationMs: 25, ContentType: "text/html"},
	}
}

func withResults(rs []intruder.Result) Intruder {
	in := NewIntruder(5, 0, 1000)
	in.results = rs
	return in
}

func sendKey(in Intruder, s string) Intruder {
	out, _ := in.updateResults(tea.KeyMsg(tea.Key{Type: keyTypeFor(s), Runes: []rune(s)}))
	return out
}

// keyTypeFor maps the small set of named keys the tests press to
// bubbletea's tea.KeyType. tea.KeyMsg.String() reads Type before Runes,
// so the press of "esc" must have Type == tea.KeyEsc, not just Runes.
func keyTypeFor(s string) tea.KeyType {
	switch s {
	case "esc":
		return tea.KeyEsc
	case "enter":
		return tea.KeyEnter
	case "backspace":
		return tea.KeyBackspace
	}
	return tea.KeyRunes
}

func TestParsePayloadInput_Range(t *testing.T) {
	cfg, err := parsePayloadInput(intruder.PayloadRange, "1-100")
	if err != "" {
		t.Fatalf("err: %s", err)
	}
	if cfg.From != 1 || cfg.To != 100 || cfg.Step != 1 {
		t.Errorf("range: %+v", cfg)
	}
}

func TestParsePayloadInput_RangeWithStep(t *testing.T) {
	cfg, err := parsePayloadInput(intruder.PayloadRange, "1-100/5")
	if err != "" {
		t.Fatalf("err: %s", err)
	}
	if cfg.From != 1 || cfg.To != 100 || cfg.Step != 5 {
		t.Errorf("range: %+v", cfg)
	}
}

func TestParsePayloadInput_RangeInvalid(t *testing.T) {
	if _, err := parsePayloadInput(intruder.PayloadRange, "abc"); err == "" {
		t.Errorf("expected error for non-range input")
	}
	if _, err := parsePayloadInput(intruder.PayloadRange, "1-100/0"); err == "" {
		t.Errorf("expected error for step < 1")
	}
}

func TestParsePayloadInput_List(t *testing.T) {
	cfg, err := parsePayloadInput(intruder.PayloadList, "a, b ,c")
	if err != "" {
		t.Fatalf("err: %s", err)
	}
	if strings.Join(cfg.Words, "|") != "a|b|c" {
		t.Errorf("list words: %v", cfg.Words)
	}
}

func TestParsePayloadInput_BruteValid(t *testing.T) {
	cfg, err := parsePayloadInput(intruder.PayloadBrute, "abc 1-3")
	if err != "" {
		t.Fatalf("err: %s", err)
	}
	if cfg.Alphabet != "abc" || cfg.MinLen != 1 || cfg.MaxLen != 3 {
		t.Errorf("brute: %+v", cfg)
	}
}

func TestParsePayloadInput_BruteMalformed(t *testing.T) {
	if _, err := parsePayloadInput(intruder.PayloadBrute, "abc"); err == "" {
		t.Errorf("expected error for missing length range")
	}
}

func TestParsePayloadInput_CaseToggle(t *testing.T) {
	cfg, err := parsePayloadInput(intruder.PayloadCaseToggle, "admin")
	if err != "" {
		t.Fatalf("err: %s", err)
	}
	if cfg.Base != "admin" {
		t.Errorf("base: %q", cfg.Base)
	}
}

func TestParsePositiveInt(t *testing.T) {
	if n, err := parsePositiveInt("5", "x"); err != "" || n != 5 {
		t.Errorf("5: n=%d err=%s", n, err)
	}
	if _, err := parsePositiveInt("0", "x"); err == "" {
		t.Errorf("expected error for 0")
	}
	if _, err := parsePositiveInt("abc", "x"); err == "" {
		t.Errorf("expected error for non-int")
	}
}

func TestIntruder_DefaultViewIsSendOrder(t *testing.T) {
	in := withResults(sampleResults())
	idx := in.view()
	want := []int{0, 1, 2, 3, 4}
	if !equalInts(idx, want) {
		t.Errorf("default view: got %v, want %v", idx, want)
	}
}

func TestIntruder_SortByStatusDescending(t *testing.T) {
	in := withResults(sampleResults())
	in = sendKey(in, "s") // cycle to status; default desc for numeric cols
	if in.sortCol != sortStatus {
		t.Fatalf("sortCol after one s: %v", in.sortCol)
	}
	if in.sortAsc {
		t.Fatalf("expected descending default for status column")
	}
	idx := in.view()
	// 500, 404, 401, 200, 200 → indices 3, 4, 1, then 0, 2 (stable)
	want := []int{3, 4, 1, 0, 2}
	if !equalInts(idx, want) {
		t.Errorf("status desc: got %v (statuses %v), want %v", idx, statuses(in.results, idx), want)
	}
}

func TestIntruder_SortBySizeDescending(t *testing.T) {
	in := withResults(sampleResults())
	in = sendKey(in, "s") // status
	in = sendKey(in, "s") // size
	if in.sortCol != sortSize {
		t.Fatalf("sortCol after two s presses: %v", in.sortCol)
	}
	idx := in.view()
	// sizes desc: 200, 100, 80, 60, 50 → indices 2, 0, 3, 4, 1
	want := []int{2, 0, 3, 4, 1}
	if !equalInts(idx, want) {
		t.Errorf("size desc: got %v, want %v", idx, want)
	}
}

func TestIntruder_SortReverseToggle(t *testing.T) {
	in := withResults(sampleResults())
	in = sendKey(in, "s")       // status desc
	in = sendKey(in, "S")       // status asc
	if !in.sortAsc {
		t.Fatalf("expected ascending after S")
	}
	idx := in.view()
	// statuses asc: 200, 200, 401, 404, 500 → indices 0, 2, 1, 4, 3
	want := []int{0, 2, 1, 4, 3}
	if !equalInts(idx, want) {
		t.Errorf("status asc: got %v, want %v", idx, want)
	}
}

func TestIntruder_SortCycleWrapsToIndex(t *testing.T) {
	in := withResults(sampleResults())
	for i := 0; i < int(numSortColumns); i++ {
		in = sendKey(in, "s")
	}
	if in.sortCol != sortIndex {
		t.Errorf("expected wrap back to sortIndex, got %v", in.sortCol)
	}
}

func TestIntruder_FilterSubstring(t *testing.T) {
	in := withResults(sampleResults())
	in.filter = "admin"
	idx := in.view()
	// "admin" and "adminuser" both contain "admin"
	want := []int{0, 4}
	if !equalInts(idx, want) {
		t.Errorf("filter admin: got %v, want %v", idx, want)
	}
}

func TestIntruder_FilterCaseInsensitive(t *testing.T) {
	in := withResults(sampleResults())
	in.filter = "ADMIN"
	idx := in.view()
	if len(idx) != 2 {
		t.Errorf("uppercase filter should still match: got %v", idx)
	}
}

func TestIntruder_FilterPresetErrors(t *testing.T) {
	in := withResults(sampleResults())
	in.preset = presetErrors
	idx := in.view()
	// errors = status >= 400 or non-empty Error: indices 1, 3, 4
	want := []int{1, 3, 4}
	if !equalInts(idx, want) {
		t.Errorf("preset errors: got %v, want %v", idx, want)
	}
}

func TestIntruder_FilterPresetSuccess(t *testing.T) {
	in := withResults(sampleResults())
	in.preset = presetSuccess
	idx := in.view()
	// 2xx only: indices 0, 2
	want := []int{0, 2}
	if !equalInts(idx, want) {
		t.Errorf("preset success: got %v, want %v", idx, want)
	}
}

func TestIntruder_FilterErrorIncludesNetworkError(t *testing.T) {
	rs := sampleResults()
	rs = append(rs, intruder.Result{Index: 5, Payload: "boom", Status: 0, Error: "dial timeout"})
	in := withResults(rs)
	in.preset = presetErrors
	idx := in.view()
	// network error row (index 5) must be included.
	found := false
	for _, i := range idx {
		if i == 5 {
			found = true
		}
	}
	if !found {
		t.Errorf("network-error row missing from errors preset: %v", idx)
	}
}

func TestIntruder_FilterSlashEntersInputMode(t *testing.T) {
	in := withResults(sampleResults())
	in.state = IntruderResults
	in = sendKey(in, "/")
	if !in.filterMode {
		t.Errorf("expected filterMode true after /")
	}
}

func TestIntruder_FilterAcceptsMultiByteRunes(t *testing.T) {
	in := withResults(sampleResults())
	in.state = IntruderResults
	in = sendKey(in, "/")
	// Send a single CJK kanji as KeyRunes. The character is 3 bytes
	// in UTF-8 but a single rune; the old byte-length check rejected
	// it. Verify it lands in the filter.
	out, _ := in.updateResults(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune("あ")}))
	if out.filter != "あ" {
		t.Errorf("multi-byte rune rejected from filter: %q", out.filter)
	}
}

func TestIntruder_FilterRejectsNamedKeysAndControlChars(t *testing.T) {
	in := withResults(sampleResults())
	in.state = IntruderResults
	in = sendKey(in, "/")
	// "left" is a named key (Type=KeyLeft). After my fix the default
	// branch should still reject it because String() returns "left"
	// (4 runes). Verify nothing is appended.
	out, _ := in.updateResults(tea.KeyMsg(tea.Key{Type: tea.KeyLeft}))
	if out.filter != "" {
		t.Errorf("named key leaked into filter: %q", out.filter)
	}
}

func TestIntruder_FilterInputAcceptsAndCommits(t *testing.T) {
	in := withResults(sampleResults())
	in.state = IntruderResults
	in = sendKey(in, "/")
	in = sendKey(in, "a")
	in = sendKey(in, "d")
	in = sendKey(in, "m")
	in = sendKey(in, "enter")
	if in.filterMode {
		t.Errorf("filterMode should be false after enter")
	}
	if in.filter != "adm" {
		t.Errorf("filter contents: %q", in.filter)
	}
}

func TestIntruder_EscClearsFilterBeforeAborting(t *testing.T) {
	in := withResults(sampleResults())
	in.state = IntruderResults
	in.filter = "admin"
	in = sendKey(in, "esc")
	if in.filter != "" {
		t.Errorf("first Esc should clear filter, got %q", in.filter)
	}
	if in.state != IntruderResults {
		t.Errorf("first Esc should NOT abort; state=%v", in.state)
	}
	in = sendKey(in, "esc")
	if in.state != IntruderHidden {
		t.Errorf("second Esc should abort; state=%v", in.state)
	}
}

func TestIntruder_EscInsideFilterInputDropsFilter(t *testing.T) {
	in := withResults(sampleResults())
	in.state = IntruderResults
	in.filter = "old"
	in = sendKey(in, "/") // re-enter input mode
	in = sendKey(in, "esc")
	if in.filterMode {
		t.Errorf("filterMode should be false after Esc in input mode")
	}
	if in.filter != "" {
		t.Errorf("Esc in input mode should clear filter, got %q", in.filter)
	}
}

func TestIntruder_PresetCyclesAllErrorsSuccess(t *testing.T) {
	in := withResults(sampleResults())
	in.state = IntruderResults
	if in.preset != presetAll {
		t.Fatalf("initial preset: %v", in.preset)
	}
	in = sendKey(in, "f")
	if in.preset != presetErrors {
		t.Errorf("after one f: %v", in.preset)
	}
	in = sendKey(in, "f")
	if in.preset != presetSuccess {
		t.Errorf("after two f: %v", in.preset)
	}
	in = sendKey(in, "f")
	if in.preset != presetAll {
		t.Errorf("after three f (wrap): %v", in.preset)
	}
}

func TestIntruder_ScrollClampsToFilteredView(t *testing.T) {
	in := withResults(sampleResults())
	in.state = IntruderResults
	in.height = 20 // visibleRows = 12
	in.scrollOffset = 4
	in.filter = "guest" // 1 match → max offset is 0
	in = sendKey(in, "down")
	if in.scrollOffset != 0 {
		t.Errorf("scrollOffset should clamp to 0 when filter shows 1 row, got %d", in.scrollOffset)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func statuses(rs []intruder.Result, idx []int) []int {
	out := make([]int, len(idx))
	for i, k := range idx {
		out[i] = rs[k].Status
	}
	return out
}

func TestNewIntruder_DefaultsPopulated(t *testing.T) {
	in := NewIntruder(5, 0, 1000)
	if in.concInput.Value() != "5" {
		t.Errorf("concurrency default: %q", in.concInput.Value())
	}
	if in.delayInput.Value() != "0" {
		t.Errorf("delay default: %q", in.delayInput.Value())
	}
	if in.maxInput.Value() != "1000" {
		t.Errorf("max default: %q", in.maxInput.Value())
	}
	if in.State() != IntruderHidden {
		t.Errorf("initial state: %v", in.State())
	}
}
