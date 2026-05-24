package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var Methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

type Method struct {
	idx     int
	focused bool
}

func NewMethod() Method {
	return Method{idx: 0}
}

func (m Method) Value() string { return Methods[m.idx] }

func (m *Method) Set(method string) {
	for i, v := range Methods {
		if v == method {
			m.idx = i
			return
		}
	}
}

func (m *Method) Focus() { m.focused = true }
func (m *Method) Blur()  { m.focused = false }
func (m Method) Focused() bool { return m.focused }

func (m Method) Update(msg tea.Msg) (Method, tea.Cmd) {
	if !m.focused {
		return m, nil
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(km, key.NewBinding(key.WithKeys("up", "k"))):
			m.idx = (m.idx - 1 + len(Methods)) % len(Methods)
		case key.Matches(km, key.NewBinding(key.WithKeys("down", "j", " "))):
			m.idx = (m.idx + 1) % len(Methods)
		}
	}
	return m, nil
}

func (m Method) View() string {
	style := lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		Width(10).
		Align(lipgloss.Center)
	if m.focused {
		style = style.BorderForeground(lipgloss.Color("205"))
	} else {
		style = style.BorderForeground(lipgloss.Color("240"))
	}
	return style.Render(m.Value())
}
