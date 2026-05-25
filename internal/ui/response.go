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

	"github.com/lea/pollen/internal/history"
	"github.com/lea/pollen/internal/httpx"
)

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
}

func NewResponse() Response {
	vp := viewport.New(80, 10)
	fi := textinput.New()
	fi.Placeholder = "jq filter  e.g. .items[0].name"
	fi.CharLimit = 256
	return Response{vp: vp, filterInput: fi}
}

func (r *Response) SetResponse(resp *history.Response, reqURL string) {
	r.prevResp = r.resp // capture before replacing
	r.resp = resp
	r.reqURL = reqURL
	r.err = ""
	r.loading = false
	r.resetFilter()
	if r.diffMode && r.prevResp != nil {
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
	r.vp.SetContent(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(err))
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

func (r Response) Update(msg tea.Msg) (Response, tea.Cmd) {
	if !r.focused {
		return r, nil
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case r.filterActive && km.String() == "esc":
			r.resetFilter()
			return r, nil

		case r.filterActive && km.String() == "enter":
			r.filterActive = false
			r.filterInput.Blur()
			return r, nil

		case !r.filterActive && km.String() == "/":
			if r.resp != nil && !r.resp.IsBinary {
				r.filterActive = true
				return r, r.filterInput.Focus()
			}
			return r, nil

		case !r.filterActive && km.String() == "D":
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
			} else if r.filteredBody != "" {
				r.vp.SetContent(r.filteredBody)
			} else {
				r.vp.SetContent(r.formatBody())
			}
			r.vp.GotoTop()
			return r, nil

		case r.filterActive:
			var cmd tea.Cmd
			r.filterInput, cmd = r.filterInput.Update(msg)
			r.applyFilter()
			return r, cmd
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

// computeDiff produces a coloured character-level diff of prevResp.Body vs
// resp.Body. Insertions are green, deletions are red strikethrough.
func (r *Response) computeDiff() string {
	if r.prevResp == nil || r.resp == nil {
		return ""
	}
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(r.prevResp.Body, r.resp.Body, true)
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
const TextPreviewLimit = 100 * 1024 // 100 KiB

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

// CurrentBytes returns the raw response body for the currently displayed
// response, or nil if no body is available (text response or reloaded from history).
func (r Response) CurrentBytes() []byte {
	if r.resp == nil {
		return nil
	}
	return r.resp.BodyBytes
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
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("Response: loading…")
	case r.err != "":
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Response: error")
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
		if r.diffMode {
			statusLine += "   " + lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("[diff]")
		}
		if hdrs := formatHeaders(r.resp.Headers); hdrs != "" {
			statusLine += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(hdrs)
		}
	default:
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Response: (no requests yet)")
	}

	r.vp.Width = inner - 2 // -2 for padding
	filterBarH := 0
	if r.filterActive || r.filteredBody != "" {
		filterBarH = 2 // "\n" + one line of filter bar
	}
	vpH := innerH - lipgloss.Height(statusLine) - 1 - filterBarH
	if vpH < 1 {
		vpH = 1
	}
	r.vp.Height = vpH

	body := statusLine + "\n" + r.vp.View()
	if r.filterActive || r.filteredBody != "" {
		body += "\n" + r.renderFilterBar(inner-2)
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
		lines = append(lines, fmt.Sprintf("%s: %s", h.Key, h.Value))
		if len(lines) >= 5 {
			lines = append(lines, fmt.Sprintf("(+ %d more headers)", len(sorted)-5))
			break
		}
	}
	return strings.Join(lines, "\n")
}
