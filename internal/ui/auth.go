package ui

import (
	"encoding/base64"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AuthType int

const (
	AuthNone AuthType = iota
	AuthBearer
	AuthBasic
)

var authTypes = []AuthType{AuthNone, AuthBearer, AuthBasic}

func (t AuthType) String() string {
	switch t {
	case AuthBearer:
		return "Bearer"
	case AuthBasic:
		return "Basic"
	default:
		return "None"
	}
}

// Auth holds the user-selected authentication scheme and its inputs. When
// non-None, HeaderValue returns the value to inject as `Authorization`.
type Auth struct {
	authType AuthType
	token    textinput.Model // Bearer
	user     textinput.Model // Basic
	pass     textinput.Model // Basic
	focused  bool
	// cursor: 0 = type selector row, 1 = first input, 2 = second input (Basic only)
	cursor int
}

func NewAuth() Auth {
	token := textinput.New()
	token.Placeholder = "bearer token"
	token.CharLimit = 4096

	user := textinput.New()
	user.Placeholder = "username"
	user.CharLimit = 256

	pass := textinput.New()
	pass.Placeholder = "password"
	pass.CharLimit = 256
	pass.EchoMode = textinput.EchoPassword
	pass.EchoCharacter = '•'

	return Auth{
		authType: AuthNone,
		token:    token,
		user:     user,
		pass:     pass,
	}
}

// Type returns the current auth scheme.
func (a Auth) Type() AuthType { return a.authType }

// HeaderValue returns the Authorization header value for the current type,
// or "" when no auth is configured (None, or required fields empty).
func (a Auth) HeaderValue() string {
	switch a.authType {
	case AuthBearer:
		tok := strings.TrimSpace(a.token.Value())
		if tok == "" {
			return ""
		}
		return "Bearer " + tok
	case AuthBasic:
		u := a.user.Value()
		p := a.pass.Value()
		if u == "" && p == "" {
			return ""
		}
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(u+":"+p))
	}
	return ""
}

// Reset clears state, called when a history entry is loaded so the previous
// session's Auth doesn't accidentally apply to an unrelated restored request.
func (a *Auth) Reset() {
	a.authType = AuthNone
	a.token.SetValue("")
	a.user.SetValue("")
	a.pass.SetValue("")
	a.cursor = 0
	a.refreshFocus()
}

func (a *Auth) Focus() {
	a.focused = true
	a.refreshFocus()
}

func (a *Auth) Blur() {
	a.focused = false
	a.token.Blur()
	a.user.Blur()
	a.pass.Blur()
}

func (a Auth) Focused() bool { return a.focused }

func (a *Auth) refreshFocus() {
	a.token.Blur()
	a.user.Blur()
	a.pass.Blur()
	if !a.focused {
		return
	}
	switch a.authType {
	case AuthBearer:
		if a.cursor == 1 {
			a.token.Focus()
		}
	case AuthBasic:
		switch a.cursor {
		case 1:
			a.user.Focus()
		case 2:
			a.pass.Focus()
		}
	}
}

func indexOfAuth(t AuthType) int {
	for i, x := range authTypes {
		if x == t {
			return i
		}
	}
	return 0
}

func (a Auth) Update(msg tea.Msg) (Auth, tea.Cmd) {
	if !a.focused {
		return a, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return a, nil
	}

	// Type selector row.
	if a.cursor == 0 {
		switch {
		case key.Matches(km, key.NewBinding(key.WithKeys("left", "h"))):
			i := indexOfAuth(a.authType)
			a.authType = authTypes[(i-1+len(authTypes))%len(authTypes)]
			a.refreshFocus()
			return a, nil
		case key.Matches(km, key.NewBinding(key.WithKeys("right", "l"))):
			i := indexOfAuth(a.authType)
			a.authType = authTypes[(i+1)%len(authTypes)]
			a.refreshFocus()
			return a, nil
		case key.Matches(km, key.NewBinding(key.WithKeys("down", "enter", "i"))):
			if a.authType != AuthNone {
				a.cursor = 1
				a.refreshFocus()
			}
			return a, nil
		}
		return a, nil
	}

	// Input row. Esc / up moves back toward the type selector.
	switch km.String() {
	case "esc":
		a.cursor = 0
		a.refreshFocus()
		return a, nil
	case "up":
		if a.cursor == 2 {
			a.cursor = 1
		} else {
			a.cursor = 0
		}
		a.refreshFocus()
		return a, nil
	case "down":
		if a.authType == AuthBasic && a.cursor == 1 {
			a.cursor = 2
			a.refreshFocus()
			return a, nil
		}
	}

	var cmd tea.Cmd
	switch {
	case a.authType == AuthBearer && a.cursor == 1:
		a.token, cmd = a.token.Update(msg)
	case a.authType == AuthBasic && a.cursor == 1:
		a.user, cmd = a.user.Update(msg)
	case a.authType == AuthBasic && a.cursor == 2:
		a.pass, cmd = a.pass.Update(msg)
	}
	return a, cmd
}

func (a Auth) View(width int) string {
	inner := width - 2
	if inner < 1 {
		inner = 1
	}
	border := lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		Width(inner)
	if a.focused {
		border = border.BorderForeground(lipgloss.Color("205"))
	} else {
		border = border.BorderForeground(lipgloss.Color("240"))
	}

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Auth"))
	sb.WriteString("  ")
	for i, t := range authTypes {
		label := t.String()
		s := lipgloss.NewStyle().Padding(0, 1)
		switch {
		case t == a.authType && a.focused && a.cursor == 0:
			s = s.Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0"))
		case t == a.authType:
			s = s.Background(lipgloss.Color("240")).Foreground(lipgloss.Color("230"))
		default:
			s = s.Foreground(lipgloss.Color("244"))
		}
		sb.WriteString(s.Render(label))
		if i < len(authTypes)-1 {
			sb.WriteString(" ")
		}
	}

	inputW := inner - 12
	if inputW < 10 {
		inputW = 10
	}

	switch a.authType {
	case AuthBearer:
		a.token.Width = inputW
		sb.WriteString("\n  Token: ")
		sb.WriteString(a.token.View())
	case AuthBasic:
		a.user.Width = inputW
		a.pass.Width = inputW
		sb.WriteString("\n  User:  ")
		sb.WriteString(a.user.View())
		sb.WriteString("\n  Pass:  ")
		sb.WriteString(a.pass.View())
	}

	if a.focused {
		var hint string
		if a.cursor == 0 {
			if a.authType == AuthNone {
				hint = "  ←/→ select type"
			} else {
				hint = "  ←/→ type  •  Enter/↓ to edit"
			}
		} else {
			hint = "  Esc/↑ back to type"
		}
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(hint))
	}

	return border.Render(sb.String())
}
