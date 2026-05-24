package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type URLBar struct {
	input   textinput.Model
	focused bool
}

func NewURLBar() URLBar {
	ti := textinput.New()
	ti.Placeholder = "https://example.com/api"
	ti.CharLimit = 2048
	return URLBar{input: ti}
}

func (u URLBar) Value() string { return u.input.Value() }

func (u *URLBar) SetValue(v string) { u.input.SetValue(v) }

func (u *URLBar) Focus() {
	u.focused = true
	u.input.Focus()
}

func (u *URLBar) Blur() {
	u.focused = false
	u.input.Blur()
}

func (u URLBar) Focused() bool { return u.focused }

func (u URLBar) Update(msg tea.Msg) (URLBar, tea.Cmd) {
	if !u.focused {
		return u, nil
	}
	var cmd tea.Cmd
	u.input, cmd = u.input.Update(msg)
	return u, cmd
}

func (u URLBar) View(width int) string {
	// width is the desired outer width (border included).
	inner := width - 2
	if inner < 1 {
		inner = 1
	}
	u.input.Width = inner - 2
	style := lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		Width(inner)
	if u.focused {
		style = style.BorderForeground(lipgloss.Color("205"))
	} else {
		style = style.BorderForeground(lipgloss.Color("240"))
	}
	return style.Render(u.input.View())
}
