package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lea/pollen/internal/history"
	"github.com/lea/pollen/internal/httpx"
)

type Response struct {
	vp      viewport.Model
	resp    *history.Response
	reqURL  string // URL of the request that produced resp
	err     string
	loading bool
	focused bool
}

func NewResponse() Response {
	vp := viewport.New(80, 10)
	return Response{vp: vp}
}

func (r *Response) SetResponse(resp *history.Response, reqURL string) {
	r.resp = resp
	r.reqURL = reqURL
	r.err = ""
	r.loading = false
	r.vp.SetContent(r.formatBody())
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

func (r *Response) Focus()       { r.focused = true }
func (r *Response) Blur()        { r.focused = false }
func (r Response) Focused() bool { return r.focused }

func (r Response) Update(msg tea.Msg) (Response, tea.Cmd) {
	if !r.focused {
		return r, nil
	}
	var cmd tea.Cmd
	r.vp, cmd = r.vp.Update(msg)
	return r, cmd
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
	return header + "\n\n" + httpx.HexDump(r.resp.BodyBytes, httpx.DefaultHexDumpLimit)
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
		if hdrs := formatHeaders(r.resp.Headers); hdrs != "" {
			statusLine += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(hdrs)
		}
	default:
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Response: (no requests yet)")
	}

	r.vp.Width = inner - 2 // -2 for padding
	vpH := innerH - lipgloss.Height(statusLine) - 1
	if vpH < 1 {
		vpH = 1
	}
	r.vp.Height = vpH

	return border.Render(statusLine + "\n" + r.vp.View())
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
