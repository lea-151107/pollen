package ui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea-151107/pollen/internal/history"
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
	in.state = IntruderResults // most tests want to drive the results table
	return in
}

func sendKey(in Intruder, s string) Intruder {
	// Dispatch through Update so the right per-state handler runs (results,
	// detail, config). Tests can set in.state directly and the key will
	// land in the correct branch.
	out, _ := in.Update(tea.KeyMsg(tea.Key{Type: keyTypeFor(s), Runes: []rune(s)}))
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

// TestParsePayloadInput_WordlistFile guards against the trailing-empty-payload
// and CRLF bugs: a file ending in a newline must not add a blank payload, and
// CRLF line endings must be trimmed.
func TestParsePayloadInput_WordlistFile(t *testing.T) {
	path := t.TempDir() + "/words.txt"
	// CRLF endings, a blank line, and a trailing newline.
	if err := os.WriteFile(path, []byte("alpha\r\nbeta\r\n\r\ngamma\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, errS := parsePayloadInput(intruder.PayloadList, "@"+path)
	if errS != "" {
		t.Fatalf("err: %s", errS)
	}
	if got := strings.Join(cfg.Words, "|"); got != "alpha|beta|gamma" {
		t.Errorf("wordlist words: got %q want alpha|beta|gamma", got)
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

func TestIntruder_EmptyViewMessageDistinguishesFilter(t *testing.T) {
	// Empty results: should say "waiting".
	emptyResults := withResults(nil)
	emptyResults.state = IntruderResults
	emptyResults.width = 80
	emptyResults.height = 30
	out := emptyResults.viewResults()
	if !strings.Contains(out, "waiting for first response") {
		t.Errorf("empty results should show waiting message; got:\n%s", out)
	}
	// Results present but filter excludes all: should say "no matches".
	filtered := withResults(sampleResults())
	filtered.state = IntruderResults
	filtered.filter = "zzzz-never-match"
	filtered.width = 80
	filtered.height = 30
	out2 := filtered.viewResults()
	if !strings.Contains(out2, "no results match filter") {
		t.Errorf("filter-excludes-all should show no-match message; got:\n%s", out2)
	}
	if strings.Contains(out2, "waiting for first response") {
		t.Errorf("filter-excludes-all should NOT show waiting message; got:\n%s", out2)
	}
}

func TestIntruder_EnterTransitionsToDetail(t *testing.T) {
	in := withResults(sampleResults())
	in.state = IntruderResults
	// Default cursor=0 → idx=0 (admin). After Enter, state=IntruderDetail
	// and detailIdx points at the original result index 0.
	in = sendKey(in, "enter")
	if in.state != IntruderDetail {
		t.Errorf("expected IntruderDetail after Enter, got %v", in.state)
	}
	if in.detailIdx != 0 {
		t.Errorf("expected detailIdx=0, got %d", in.detailIdx)
	}
}

func TestIntruder_EnterTargetsCurrentCursor(t *testing.T) {
	in := withResults(sampleResults())
	in.state = IntruderResults
	in.cursor = 3 // row 3 (test, status 500)
	in = sendKey(in, "enter")
	if in.state != IntruderDetail {
		t.Errorf("expected IntruderDetail")
	}
	if in.detailIdx != 3 {
		t.Errorf("expected detailIdx=3, got %d", in.detailIdx)
	}
}

func TestIntruder_DetailScrollClampsAtEOF(t *testing.T) {
	// A short body (just "--- body ---" header line + maybe one or two
	// content lines) shouldn't let the user scroll arbitrarily far past
	// EOF — pre-v1.4.1 down/PgDn had no upper bound, so the window
	// silently went blank.
	rs := []intruder.Result{{
		Index:     0,
		Payload:   "x",
		Status:    200,
		Response:  &history.Response{Status: 200, Body: "hi", BodyBytes: []byte("hi")},
	}}
	in := withResults(rs)
	in.state = IntruderDetail
	in.detailIdx = 0
	in.height = 30
	// Press PgDn 50 times — way more than the body has lines.
	for i := 0; i < 50; i++ {
		in = sendKey(in, "pgdown")
	}
	// maxDetailScroll is len(bodyLines) - visible; for the small body
	// that's 0. detailScroll must be clamped accordingly.
	if in.detailScroll != 0 {
		t.Errorf("detailScroll should clamp to 0 for short body, got %d", in.detailScroll)
	}
}

func TestIntruder_DetailEscReturnsToResults(t *testing.T) {
	in := withResults(sampleResults())
	in.state = IntruderDetail
	in.detailIdx = 2
	in = sendKey(in, "esc")
	if in.state != IntruderResults {
		t.Errorf("expected IntruderResults after Esc in detail, got %v", in.state)
	}
}

func TestIntruder_DownAdvancesCursor(t *testing.T) {
	in := withResults(sampleResults())
	in.state = IntruderResults
	in.height = 20
	if in.cursor != 0 {
		t.Fatalf("initial cursor: %d", in.cursor)
	}
	in = sendKey(in, "down")
	if in.cursor != 1 {
		t.Errorf("expected cursor 1 after Down, got %d", in.cursor)
	}
}

func TestIntruder_DownStopsAtLastRow(t *testing.T) {
	in := withResults(sampleResults())
	in.state = IntruderResults
	in.height = 20
	in.cursor = len(sampleResults()) - 1
	in = sendKey(in, "down")
	if in.cursor != len(sampleResults())-1 {
		t.Errorf("cursor should not pass last row, got %d", in.cursor)
	}
}

func TestParseNumExpr(t *testing.T) {
	cases := []struct {
		in     string
		op     string
		lo, hi int
		ok     bool
	}{
		{"42", "eq", 42, 0, true},
		{">100", "gt", 100, 0, true},
		{">=200", "ge", 200, 0, true},
		{"<50", "lt", 50, 0, true},
		{"<=99", "le", 99, 0, true},
		{"100-200", "range", 100, 200, true},
		{"abc", "", 0, 0, false},
		{">", "", 0, 0, false},
		{"", "", 0, 0, false},
	}
	for _, c := range cases {
		lo, hi, op, ok := parseNumExpr(c.in)
		if ok != c.ok || op != c.op || lo != c.lo || hi != c.hi {
			t.Errorf("%q: got (%d,%d,%q,%v), want (%d,%d,%q,%v)",
				c.in, lo, hi, op, ok, c.lo, c.hi, c.op, c.ok)
		}
	}
}

func TestIntruder_FilterSizeRange(t *testing.T) {
	in := withResults(sampleResults())
	in.filter = "size:>=100"
	idx := in.view()
	// sizes: 100, 50, 200, 80, 60 → keep 100, 200 (indices 0 and 2)
	want := []int{0, 2}
	if !equalInts(idx, want) {
		t.Errorf("size:>=100 view: got %v, want %v", idx, want)
	}
}

func TestIntruder_FilterSizeRangeMinMax(t *testing.T) {
	in := withResults(sampleResults())
	in.filter = "size:50-100"
	idx := in.view()
	// indices with size in [50,100]: 0 (100), 1 (50), 3 (80), 4 (60)
	want := []int{0, 1, 3, 4}
	if !equalInts(idx, want) {
		t.Errorf("size:50-100 view: got %v, want %v", idx, want)
	}
}

func TestIntruder_FilterDurRange(t *testing.T) {
	in := withResults(sampleResults())
	in.filter = "dur:>20"
	// durations: 30, 20, 10, 40, 25 → >20 keeps 30, 40, 25 → indices 0, 3, 4
	idx := in.view()
	want := []int{0, 3, 4}
	if !equalInts(idx, want) {
		t.Errorf("dur:>20 view: got %v, want %v", idx, want)
	}
}

func TestIntruder_FilterStatusClass(t *testing.T) {
	in := withResults(sampleResults())
	in.filter = "s:4xx"
	// statuses: 200, 401, 200, 500, 404 → 4xx keeps 401 (1), 404 (4)
	idx := in.view()
	want := []int{1, 4}
	if !equalInts(idx, want) {
		t.Errorf("s:4xx view: got %v, want %v", idx, want)
	}
}

func TestIntruder_FilterStatusExact(t *testing.T) {
	in := withResults(sampleResults())
	in.filter = "s:500"
	idx := in.view()
	want := []int{3}
	if !equalInts(idx, want) {
		t.Errorf("s:500 view: got %v, want %v", idx, want)
	}
}

func TestIntruder_FilterComposes(t *testing.T) {
	in := withResults(sampleResults())
	// admin AND size>=100 → indices 0 (admin/100) only (4 is adminuser/60)
	in.filter = "admin size:>=100"
	idx := in.view()
	want := []int{0}
	if !equalInts(idx, want) {
		t.Errorf("composed filter: got %v, want %v", idx, want)
	}
}

func TestIntruder_FilterBareSubstring(t *testing.T) {
	in := withResults(sampleResults())
	in.filter = "guest"
	idx := in.view()
	want := []int{2}
	if !equalInts(idx, want) {
		t.Errorf("bare filter: got %v, want %v", idx, want)
	}
}

func TestSizeMedian_OddCount(t *testing.T) {
	rs := []intruder.Result{
		{Size: 10}, {Size: 20}, {Size: 30}, {Size: 40}, {Size: 50},
	}
	idx := []int{0, 1, 2, 3, 4}
	if got := sizeMedian(rs, idx); got != 30 {
		t.Errorf("median of 5 sizes: got %d, want 30", got)
	}
}

func TestSizeMedian_EvenCount(t *testing.T) {
	rs := []intruder.Result{{Size: 10}, {Size: 20}, {Size: 30}, {Size: 40}}
	if got := sizeMedian(rs, []int{0, 1, 2, 3}); got != 25 {
		t.Errorf("median of 4 sizes: got %d, want 25", got)
	}
}

func TestSizeMedian_EmptyZero(t *testing.T) {
	if got := sizeMedian([]intruder.Result{{Size: 10}}, nil); got != 0 {
		t.Errorf("empty idx should give median 0, got %d", got)
	}
}

func TestIsSizeOutlier_50PercentRule(t *testing.T) {
	cases := []struct {
		name   string
		size   int
		median int
		want   bool
	}{
		{"within band low", 600, 1000, false},  // 40% below
		{"within band high", 1400, 1000, false}, // 40% above
		{"on the edge low", 500, 1000, false},  // 50% exactly → not outlier
		{"on the edge high", 1500, 1000, false}, // 50% exactly → not outlier
		{"below band", 400, 1000, true},
		{"above band", 1600, 1000, true},
		{"zero median means no outliers", 5000, 0, false},
	}
	for _, c := range cases {
		got := isSizeOutlier(intruder.Result{Size: c.size}, c.median)
		if got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestStatusTint_4xxYellow5xxRed(t *testing.T) {
	cases := []struct {
		name string
		r    intruder.Result
		want string
	}{
		{"2xx no tint", intruder.Result{Status: 200}, ""},
		{"3xx no tint", intruder.Result{Status: 301}, ""},
		{"401 yellow", intruder.Result{Status: 401}, "11"},
		{"404 yellow", intruder.Result{Status: 404}, "11"},
		{"499 yellow", intruder.Result{Status: 499}, "11"},
		{"500 red", intruder.Result{Status: 500}, "9"},
		{"503 red", intruder.Result{Status: 503}, "9"},
		{"network error red", intruder.Result{Status: 0, Error: "dial tcp"}, "9"},
	}
	for _, c := range cases {
		got := string(statusTint(c.r))
		if got != c.want {
			t.Errorf("%s: got tint %q, want %q", c.name, got, c.want)
		}
	}
}

func TestIntruder_ExportPromptOpensWithE(t *testing.T) {
	in := withResults(sampleResults())
	in = sendKey(in, "e")
	if !in.exportMode {
		t.Errorf("expected exportMode true after 'e' key")
	}
	if in.exportInput.Value() == "" {
		t.Errorf("export input should have a default path suggestion")
	}
}

func TestIntruder_ExportPromptIgnoredWhenNoResults(t *testing.T) {
	in := withResults(nil)
	in = sendKey(in, "e")
	if in.exportMode {
		t.Errorf("export prompt should not open with no results")
	}
}

func TestIntruder_ExportPromptEscCancels(t *testing.T) {
	in := withResults(sampleResults())
	in = sendKey(in, "e")
	if !in.exportMode {
		t.Fatalf("setup: exportMode should be true")
	}
	in = sendKey(in, "esc")
	if in.exportMode {
		t.Errorf("Esc inside export prompt should cancel")
	}
}

func TestIntruder_ExportPromptEnterEmitsCmd(t *testing.T) {
	in := withResults(sampleResults())
	in = sendKey(in, "e")
	in.exportInput.SetValue("/tmp/test.csv")
	out, cmd := in.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	if out.exportMode {
		t.Errorf("Enter should leave export mode")
	}
	if cmd == nil {
		t.Fatalf("Enter with non-empty path should emit a Cmd")
	}
	msg := cmd()
	em, ok := msg.(IntruderExportMsg)
	if !ok {
		t.Fatalf("expected IntruderExportMsg, got %T", msg)
	}
	if em.Path != "/tmp/test.csv" {
		t.Errorf("unexpected path: %q", em.Path)
	}
}

func TestIntruder_EnterOnEmptyViewIsNoop(t *testing.T) {
	// Filter excludes everything → Enter should not transition or panic.
	in := withResults(sampleResults())
	in.state = IntruderResults
	in.filter = "zzz-never-matches"
	in = sendKey(in, "enter")
	if in.state != IntruderResults {
		t.Errorf("Enter on empty view should not change state, got %v", in.state)
	}
}

func TestIntruder_CursorClampsToFilteredView(t *testing.T) {
	// Reproduces a v1.4.0 UX bug: typing a filter narrowed the view
	// but didn't update cursor, so the ▶ marker disappeared and the
	// Down key became a no-op (the cursor sat past rows-1).
	in := withResults(sampleResults())
	in.state = IntruderResults
	in.height = 20
	in.cursor = 4 // last row of the 5-row sample
	// Apply a filter that narrows the view to a single matching row.
	in.filter = "guest"
	// Send any nav key (e.g., a no-op key); the clamp runs at the top.
	in = sendKey(in, "down")
	if in.cursor != 0 {
		t.Errorf("cursor should have been clamped to last visible row, got %d", in.cursor)
	}
}

func TestIntruder_CursorClampsAfterPresetNarrowing(t *testing.T) {
	in := withResults(sampleResults())
	in.state = IntruderResults
	in.height = 20
	in.cursor = 4
	// Directly set preset to errors, simulating a state change that
	// hasn't gone through the f-key reset path (also covers any future
	// caller that sets preset directly).
	in.preset = presetSuccess // narrows to 2 results: indices 0 and 2
	in = sendKey(in, "down")  // triggers the clamp
	if in.cursor > 1 {
		t.Errorf("cursor should have been clamped to <=1, got %d", in.cursor)
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
