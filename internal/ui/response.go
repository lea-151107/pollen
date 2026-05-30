package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/itchyny/gojq"
	"github.com/sergi/go-diff/diffmatchpatch"

	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/httpx"
)

// ResponseCopyMsg is emitted when the user requests the response body be
// copied to the clipboard.
type ResponseCopyMsg struct{ Text string }

type Response struct {
	vp           viewport.Model
	resp         *history.Response
	prevResp     *history.Response // response before the most recent SetResponse call
	reqURL       string            // URL of the request that produced resp
	err          string
	loading      bool
	focused      bool
	filterInput  textinput.Model
	filterActive bool   // filter input bar is visible
	filterErr    string // last jq evaluation error
	filteredBody string // non-empty when a filter is applied
	diffMode     bool   // showing diff against prevResp
	diffBody     string // rendered diff (cached)
	searchActive bool
	searchInput  textinput.Model
	searchQuery  string
}

func NewResponse() Response {
	vp := viewport.New(80, 10)
	fi := textinput.New()
	fi.Placeholder = "jq filter  e.g. .items[0].name"
	fi.CharLimit = 256
	si := textinput.New()
	si.Placeholder = "search..."
	si.CharLimit = 256
	return Response{vp: vp, filterInput: fi, searchInput: si}
}

func (r *Response) SetResponse(resp *history.Response, reqURL string) {
	r.prevResp = r.resp // capture before replacing
	r.resp = resp
	r.reqURL = reqURL
	r.err = ""
	r.loading = false
	r.resetFilter()
	if r.diffMode && r.prevResp != nil && r.resp != nil && !r.resp.IsBinary && !r.prevResp.IsBinary {
		r.diffBody = r.computeDiff()
		r.vp.SetContent(r.diffBody)
	} else {
		r.diffMode = false
		r.vp.SetContent(r.formatBody())
	}
	r.vp.GotoTop()
}

// RequestURL returns the URL of the request that produced the currently
// displayed response. Used by the save action to derive a stable filename
// even when the user has since edited the URL bar.
func (r Response) RequestURL() string { return r.reqURL }

func (r *Response) SetError(err string) {
	r.resp = nil
	r.err = err
	r.loading = false
	// Drop any leftover view state — without this, an error response
	// would land under a still-visible jq filter bar, a search bar, or
	// a stale "diff" badge from the previous successful response. Keep
	// in sync with resetFilter; searchActive and diffMode need their
	// own clear because the user can't dismiss them once the error
	// view replaces the body.
	r.filterActive = false
	r.filterInput.SetValue("")
	r.filterInput.Blur()
	r.filterErr = ""
	r.filteredBody = ""
	r.searchActive = false
	r.searchQuery = ""
	r.diffMode = false
	r.diffBody = ""
	// Sanitize: err strings can contain server-influenced text (TLS Subject,
	// redirect URL, etc.) and are also re-displayed from history via
	// applyEntry, so a stored injection could fire later.
	r.vp.SetContent(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).
		Render(sanitizeTerminalControl(err)))
}

func (r *Response) SetLoading(loading bool) {
	r.loading = loading
	if loading {
		r.vp.SetContent("loading…")
	}
}

func (r *Response) Focus()            { r.focused = true }
func (r *Response) Blur()             { r.focused = false }
func (r Response) Focused() bool      { return r.focused }
func (r Response) FilterActive() bool { return r.filterActive }

// SearchActive reports whether the in-body search input is currently capturing
// keystrokes. Returns false once Enter "locks" the query (searchQuery is
// preserved but the bar no longer eats input).
func (r Response) SearchActive() bool { return r.searchActive }

func (r Response) Update(msg tea.Msg) (Response, tea.Cmd) {
	if !r.focused {
		return r, nil
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		var cmd tea.Cmd
		switch {
		case r.filterActive && km.String() == "esc":
			r.resetFilter()
			return r, nil

		case r.filterActive && km.String() == "enter":
			r.filterActive = false
			r.filterInput.Blur()
			return r, nil

		case r.filterActive:
			r.filterInput, cmd = r.filterInput.Update(msg)
			r.applyFilter()
			return r, cmd

		case r.searchActive && km.String() == "esc":
			r.searchActive = false
			r.searchQuery = ""
			r.searchInput.SetValue("")
			r.searchInput.Blur()
			r.vp.SetContent(r.currentDisplayBody())
			r.vp.GotoTop()
			return r, nil

		case r.searchActive && km.String() == "enter":
			r.searchActive = false
			r.searchInput.Blur()
			return r, nil

		case r.searchActive:
			r.searchInput, cmd = r.searchInput.Update(msg)
			r.searchQuery = r.searchInput.Value()
			r.vp.SetContent(r.currentDisplayBody())
			return r, cmd

		case !r.filterActive && !r.searchActive && km.String() == "/":
			if r.resp != nil && !r.resp.IsBinary {
				r.filterActive = true
				return r, r.filterInput.Focus()
			}
			return r, nil

		case !r.filterActive && !r.searchActive && km.String() == "ctrl+f":
			if r.resp != nil && !r.resp.IsBinary {
				r.searchActive = true
				return r, r.searchInput.Focus()
			}
			return r, nil

		case !r.filterActive && !r.searchActive && km.String() == "y":
			text := r.copyableText()
			if text == "" {
				return r, nil
			}
			return r, func() tea.Msg { return ResponseCopyMsg{Text: text} }

		case !r.filterActive && !r.searchActive && km.String() == "D":
			if r.resp == nil || r.prevResp == nil {
				return r, nil
			}
			if r.resp.IsBinary || r.prevResp.IsBinary {
				return r, nil
			}
			r.diffMode = !r.diffMode
			if r.diffMode {
				r.diffBody = r.computeDiff()
				r.vp.SetContent(r.diffBody)
			} else {
				// Toggling diff off: defer to currentDisplayBody so a locked
				// search query or jq filter is restored as the visible overlay,
				// matching the documented search > filter > diff > plain priority.
				r.vp.SetContent(r.currentDisplayBody())
			}
			r.vp.GotoTop()
			return r, nil
		}
	}

	var cmd tea.Cmd
	r.vp, cmd = r.vp.Update(msg)
	return r, cmd
}

func (r *Response) applyFilter() {
	expr := strings.TrimSpace(r.filterInput.Value())
	if expr == "" {
		r.filterErr = ""
		r.filteredBody = ""
		r.vp.SetContent(r.formatBody())
		r.vp.GotoTop()
		return
	}

	if r.resp == nil || !isJSONContentType(r.resp.ContentType) {
		r.filterErr = "not a JSON response"
		r.filteredBody = ""
		r.vp.SetContent(r.formatBody())
		return
	}

	query, err := gojq.Parse(expr)
	if err != nil {
		r.filterErr = err.Error()
		r.filteredBody = ""
		r.vp.SetContent(r.formatBody())
		return
	}

	var input any
	if err := json.Unmarshal([]byte(r.resp.Body), &input); err != nil {
		r.filterErr = "invalid JSON body"
		r.filteredBody = ""
		r.vp.SetContent(r.formatBody())
		return
	}

	iter := query.Run(input)
	var results []string
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			r.filterErr = err.Error()
			r.filteredBody = ""
			r.vp.SetContent(r.formatBody())
			return
		}
		out, _ := json.MarshalIndent(v, "", "  ")
		results = append(results, string(out))
	}

	r.filterErr = ""
	r.filteredBody = strings.Join(results, "\n")
	r.vp.SetContent(r.filteredBody)
	r.vp.GotoTop()
}

func (r *Response) resetFilter() {
	r.filterActive = false
	r.filterInput.SetValue("")
	r.filterInput.Blur()
	r.filterErr = ""
	r.filteredBody = ""
	if r.resp != nil {
		if r.diffMode && r.diffBody != "" {
			r.vp.SetContent(r.diffBody)
		} else {
			r.vp.SetContent(r.formatBody())
		}
		r.vp.GotoTop()
	}
}

// copyableText returns the text to copy to the clipboard: jq-filtered body
// when a filter is applied, otherwise the raw body.
func (r Response) copyableText() string {
	if r.filteredBody != "" {
		return r.filteredBody
	}
	if r.resp != nil {
		return r.resp.Body
	}
	return ""
}

// baseDisplayBody returns the body content without search highlighting,
// resolving the content-determining priority: filter > diff > plain.
func (r Response) baseDisplayBody() string {
	if r.filteredBody != "" {
		return r.filteredBody
	}
	if r.diffMode && r.diffBody != "" {
		return r.diffBody
	}
	return r.formatBody()
}

// currentDisplayBody returns what the viewport should render right now: the
// base content (filter > diff > plain) with search highlights composed on top
// when a query is present. This composes rather than masks, so a locked jq
// filter is preserved while the user searches inside it.
func (r Response) currentDisplayBody() string {
	base := r.baseDisplayBody()
	if r.searchQuery != "" {
		return applyLineHighlight(base, r.searchQuery)
	}
	return base
}

// applyLineHighlight wraps the first case-insensitive occurrence of needle in
// each line with bold+underline. Operates line-by-line so highlights don't
// span multi-line content.
func applyLineHighlight(body, needle string) string {
	if needle == "" || body == "" {
		return body
	}
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		lines[i] = highlightMatch(line, needle)
	}
	return strings.Join(lines, "\n")
}

// computeDiff produces a coloured character-level diff of prevResp.Body vs
// resp.Body. Insertions are green, deletions are red strikethrough.
func (r *Response) computeDiff() string {
	if r.prevResp == nil || r.resp == nil {
		return ""
	}
	// Sanitize inputs so the diff output (which preserves equal segments
	// verbatim with lipgloss colors layered on insertions/deletions) can't
	// contain raw terminal-control sequences from a hostile body.
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(sanitizeTerminalControl(r.prevResp.Body), sanitizeTerminalControl(r.resp.Body), true)
	dmp.DiffCleanupSemantic(diffs)
	var sb strings.Builder
	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffInsert:
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(d.Text))
		case diffmatchpatch.DiffDelete:
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Strikethrough(true).Render(d.Text))
		default:
			sb.WriteString(d.Text)
		}
	}
	return sb.String()
}

// TextPreviewLimit caps how much of a text body the viewport renders. Bodies
// larger than this still keep their full bytes in BodyBytes for the `s` save
// action — only the display is truncated to keep rendering responsive.
var TextPreviewLimit = 100 * 1024 // 100 KiB

func (r Response) formatBody() string {
	if r.resp == nil {
		return ""
	}
	if !r.resp.IsBinary {
		body := r.resp.Body
		// Pretty-print JSON responses for display only — `s` still saves the
		// original bytes verbatim (BodyBytes is untouched).
		if isJSONContentType(r.resp.ContentType) {
			if pretty, ok := tryPrettyJSON(body); ok {
				body = pretty
			}
		}
		// Strip terminal-control bytes so a malicious or buggy server can't
		// clear the screen / move the cursor via ANSI escapes in the body.
		body = sanitizeTerminalControl(body)
		if len(body) > TextPreviewLimit {
			// Put the notice at the TOP so the user sees it immediately
			// rather than after scrolling through ~100KB of preview.
			notice := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(
				fmt.Sprintf("(display truncated to %s of %s; press 's' to save full body)",
					formatSize(TextPreviewLimit), formatSize(len(body))),
			)
			return notice + "\n\n" + body[:TextPreviewLimit]
		}
		return body
	}
	header := r.binaryHeader()
	if len(r.resp.BodyBytes) == 0 {
		return header + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("244")).
			Render("(body not preserved in history)")
	}
	return header + "\n\n" + HexDump(r.resp.BodyBytes, DefaultHexDumpLimit)
}

func (r Response) binaryHeader() string {
	ct := r.resp.ContentType
	if ct == "" {
		ct = "unknown"
	}
	// ParseContentType's fallback path can pass through control bytes from a
	// malformed Content-Type — sanitize before mixing into the rendered line.
	ct = sanitizeTerminalControl(ct)
	line1 := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).
		Render(fmt.Sprintf("Binary response (%s, %s)", formatSize(r.resp.SizeBytes), ct))
	if len(r.resp.BodyBytes) == 0 {
		return line1
	}
	line2 := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).
		Render("  press 's' to save to file")
	return line1 + "\n" + line2
}

// tryPrettyJSON returns a 2-space-indented re-rendering of s, or (s,false) if
// the input isn't valid JSON.
func tryPrettyJSON(s string) (string, bool) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(s), "", "  "); err != nil {
		return s, false
	}
	return buf.String(), true
}

// isJSONContentType matches `application/json` and any subtype with a
// `+json` structured-syntax suffix (RFC 6839), e.g. `application/vnd.api+json`.
func isJSONContentType(ct string) bool {
	return ct == "application/json" || strings.HasSuffix(ct, "+json")
}

// sanitizeTerminalControl replaces C0 (except \t, \n, \r), DEL, and C1
// control bytes with visible `\xHH` placeholders so a malicious or buggy
// server response can't smuggle ANSI escape sequences (e.g. `\x1b[2J`) into
// the terminal. Display-only — saved/exported bytes and jq input are untouched.
// Fast path: returns s unchanged if no control bytes are present, so the
// overhead for typical bodies is one pass.
func sanitizeTerminalControl(s string) string {
	if !containsTerminalControl(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, "\\x%02x", r)
		case r >= 0x80 && r < 0xa0:
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func containsTerminalControl(s string) bool {
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			// allowed whitespace
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0):
			return true
		}
	}
	return false
}

// CurrentBytes returns the raw response body for the currently displayed
// response, or nil if no body is available (binary entries past the
// keep-bytes window in history, or freshly reloaded from disk). For text
// bodies it falls back to Body when BodyBytes was dropped to bound memory
// growth — the bytes are byte-identical to the original for valid UTF-8.
func (r Response) CurrentBytes() []byte {
	if r.resp == nil {
		return nil
	}
	if len(r.resp.BodyBytes) > 0 {
		return r.resp.BodyBytes
	}
	if !r.resp.IsBinary && r.resp.Body != "" {
		return []byte(r.resp.Body)
	}
	return nil
}

// CurrentResponse returns the displayed response or nil.
func (r Response) CurrentResponse() *history.Response {
	return r.resp
}

func (r Response) View(width, height int) string {
	inner := width - 2
	if inner < 1 {
		inner = 1
	}
	innerH := height - 2
	if innerH < 1 {
		innerH = 1
	}
	border := lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		Width(inner)
	if r.focused {
		border = border.BorderForeground(lipgloss.Color("205"))
	} else {
		border = border.BorderForeground(lipgloss.Color("240"))
	}

	var statusLine string
	switch {
	case r.loading:
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("loading…")
	case r.err != "":
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("error")
	case r.resp != nil:
		color := lipgloss.Color("10")
		if r.resp.Status >= 400 {
			color = lipgloss.Color("9")
		} else if r.resp.Status >= 300 {
			color = lipgloss.Color("214")
		}
		statusLine = fmt.Sprintf("%s   %dms   %s",
			lipgloss.NewStyle().Foreground(color).Bold(true).Render(r.resp.StatusText),
			r.resp.DurationMs,
			formatSize(r.resp.SizeBytes),
		)
		if r.resp.ContentType != "" {
			statusLine += "   " + lipgloss.NewStyle().Foreground(lipgloss.Color("44")).
				Render(r.resp.ContentType)
		}
		if r.resp.Truncated {
			statusLine += lipgloss.NewStyle().Foreground(lipgloss.Color("214")).
				Render(fmt.Sprintf("   (truncated at %s)", formatSize(httpx.MaxResponseBytes)))
		}
		// Show diff badge only when the diff view is actually what's displayed.
		// When a jq filter is locked, baseDisplayBody returns the filter content
		// (priority filter > diff > plain) so the badge would be misleading.
		if r.diffMode && r.filteredBody == "" {
			statusLine += "   " + lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("[diff]")
		}
		if hdrs := formatHeaders(r.resp.Headers); hdrs != "" {
			statusLine += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(hdrs)
		}
	default:
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("(no requests yet)")
	}

	r.vp.Width = inner - 2 // -2 for padding
	// lipgloss.Height treats "\n" as a line separator, not as an extra row,
	// so each bar contributes only its own rendered row count.
	filterBarH := 0
	if r.filterActive || r.filteredBody != "" {
		filterBarH = 1
	}
	searchBarH := 0
	if r.searchActive || r.searchQuery != "" {
		searchBarH = 1
	}
	vpH := innerH - lipgloss.Height(statusLine) - filterBarH - searchBarH
	if vpH < 1 {
		vpH = 1
	}
	r.vp.Height = vpH

	body := statusLine + "\n" + r.vp.View()
	if r.filterActive || r.filteredBody != "" {
		body += "\n" + r.renderFilterBar(inner-2)
	}
	if r.searchActive || r.searchQuery != "" {
		body += "\n" + r.renderSearchBar(inner-2)
	}
	return border.Render(body)
}

func (r Response) renderFilterBar(width int) string {
	prefix := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("/")
	input := r.filterInput.View()

	var errPart string
	if r.filterErr != "" {
		errPart = "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(r.filterErr)
	}

	bar := prefix + " " + input + errPart
	_ = width // viewport handles wrapping; bar is one logical line
	return bar
}

func (r Response) renderSearchBar(width int) string {
	_ = width
	prefix := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("/")
	color := lipgloss.Color("244")
	if r.searchActive {
		color = lipgloss.Color("214")
	}
	var cursor string
	if r.searchActive {
		cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("█")
	}
	return lipgloss.NewStyle().Foreground(color).Render(prefix+r.searchQuery) + cursor
}

func formatSize(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1fMB", float64(n)/(1024*1024))
	}
}

func formatHeaders(headers []history.Header) string {
	if len(headers) == 0 {
		return ""
	}
	sorted := make([]history.Header, len(headers))
	copy(sorted, headers)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })

	var lines []string
	for _, h := range sorted {
		lines = append(lines, fmt.Sprintf("%s: %s", h.Key, sanitizeTerminalControl(h.Value)))
		if len(lines) >= 5 {
			if len(sorted) > 5 {
				lines = append(lines, fmt.Sprintf("(+ %d more headers)", len(sorted)-5))
			}
			break
		}
	}
	return strings.Join(lines, "\n")
}
