package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lea-151107/pollen/internal/oauth"
)

type AuthType int

const (
	AuthNone AuthType = iota
	AuthBearer
	AuthBasic
	AuthOAuth   // = Client Credentials (legacy name kept for v1.x compat)
	AuthOAuthAC // = Authorization Code with PKCE
)

var authTypes = []AuthType{AuthNone, AuthBearer, AuthBasic, AuthOAuth, AuthOAuthAC}

func (t AuthType) String() string {
	switch t {
	case AuthBearer:
		return "Bearer"
	case AuthBasic:
		return "Basic"
	case AuthOAuth:
		return "OAuth"
	case AuthOAuthAC:
		return "OAuth AC"
	default:
		return "None"
	}
}

// AuthOAuthTokenMsg signals a successful OAuth token fetch back to the
// app's Update loop. The app calls Auth.SetOAuthToken to store it.
type AuthOAuthTokenMsg struct{ Token *oauth.Token }

// AuthOAuthErrorMsg signals an OAuth flow failure.
type AuthOAuthErrorMsg struct{ Err string }

// AuthOAuthACTokenMsg signals a successful Authorization Code with
// PKCE flow back to the app's Update loop.
type AuthOAuthACTokenMsg struct{ Token *oauth.Token }

// AuthOAuthACErrorMsg signals an Authorization Code flow failure or
// user cancellation.
type AuthOAuthACErrorMsg struct{ Err string }

// Auth holds the user-selected authentication scheme and its inputs. When
// non-None, the app's buildAuthFromPanel returns the Authorization header
// value to inject.
type Auth struct {
	authType AuthType
	token    textinput.Model // Bearer
	user     textinput.Model // Basic
	pass     textinput.Model // Basic

	// OAuth Client Credentials fields and fetched token. The token is
	// session-only — not persisted to disk in v1.5.0. The Token URL /
	// Client ID / Client Secret / Scope inputs are shared with the
	// Authorization Code flow below so a user who configures one IdP
	// once doesn't re-type when switching grants.
	oauthTokenURL textinput.Model
	oauthClientID textinput.Model
	oauthSecret   textinput.Model
	oauthScope    textinput.Model
	oauthToken    *oauth.Token
	oauthError    string
	oauthFetching bool

	// OAuth Authorization Code with PKCE fields (AC-specific) and the
	// fetched token. AC reuses oauthTokenURL / oauthClientID /
	// oauthSecret / oauthScope above. Like CC, the token is session-
	// only.
	oauthACAuthURL  textinput.Model
	oauthACRedirect textinput.Model
	oauthACToken    *oauth.Token
	oauthACError    string
	oauthACFetching bool
	oauthACStatus   string             // user-visible progress line
	oauthACCancel   context.CancelFunc // non-nil while a flow is in flight; Esc invokes

	focused bool
	// cursor positions per type:
	//   Bearer:   0=type, 1=token
	//   Basic:    0=type, 1=user, 2=pass
	//   OAuth:    0=type, 1=URL, 2=ID, 3=secret, 4=scope, 5=action
	//   OAuth AC: 0=type, 1=Auth URL, 2=Token URL, 3=ID, 4=secret,
	//             5=Redirect URI, 6=scope, 7=action
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

	mkInput := func(placeholder string, mask bool) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.CharLimit = 1024
		if mask {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '•'
		}
		return ti
	}

	redirect := mkInput("http://127.0.0.1:8765/callback", false)
	redirect.SetValue("http://127.0.0.1:8765/callback")

	return Auth{
		authType:        AuthNone,
		token:           token,
		user:            user,
		pass:            pass,
		oauthTokenURL:   mkInput("https://issuer.example.com/oauth/token", false),
		oauthClientID:   mkInput("client id", false),
		oauthSecret:     mkInput("client secret", true),
		oauthScope:      mkInput("space-separated scopes (optional)", false),
		oauthACAuthURL:  mkInput("https://issuer.example.com/oauth/authorize", false),
		oauthACRedirect: redirect,
	}
}

// Type returns the current auth scheme.
func (a Auth) Type() AuthType { return a.authType }

// SetType sets the current auth scheme directly. Mainly useful for
// tests in other packages that need to drive the panel into a known
// state without simulating ←/→ key events.
func (a *Auth) SetType(t AuthType) {
	a.authType = t
	a.cursor = 0
	a.refreshFocus()
}

// Token returns the Bearer token raw input (trimmed of surrounding whitespace).
// Meaningful only when Type() == AuthBearer.
func (a Auth) Token() string { return strings.TrimSpace(a.token.Value()) }

// Credentials returns the Basic auth username/password raw inputs.
// Meaningful only when Type() == AuthBasic.
func (a Auth) Credentials() (user, pass string) {
	return a.user.Value(), a.pass.Value()
}

// OAuthToken returns the currently-fetched OAuth token (or nil). Used
// by the app's buildAuthFromPanel to assemble the Authorization
// header when AuthOAuth is selected.
func (a Auth) OAuthToken() *oauth.Token { return a.oauthToken }

// SetOAuthToken stores a freshly-fetched token (called from the app's
// Update loop when AuthOAuthTokenMsg arrives).
func (a *Auth) SetOAuthToken(t *oauth.Token) {
	a.oauthToken = t
	a.oauthError = ""
	a.oauthFetching = false
}

// SetOAuthError records a failed fetch so the panel can display it.
func (a *Auth) SetOAuthError(err string) {
	a.oauthError = err
	a.oauthFetching = false
}

// OAuthACToken returns the currently-fetched Authorization Code token
// (or nil). Used by buildAuthFromPanel when AuthOAuthAC is selected.
func (a Auth) OAuthACToken() *oauth.Token { return a.oauthACToken }

// SetOAuthACToken stores a freshly-fetched Authorization Code token.
func (a *Auth) SetOAuthACToken(t *oauth.Token) {
	a.oauthACToken = t
	a.oauthACError = ""
	a.oauthACFetching = false
	a.oauthACStatus = ""
	a.oauthACCancel = nil
}

// SetOAuthACError records a failed Authorization Code flow so the
// panel can display it.
func (a *Auth) SetOAuthACError(err string) {
	a.oauthACError = err
	a.oauthACFetching = false
	a.oauthACStatus = ""
	a.oauthACCancel = nil
}

// CurrentOAuthToken returns the active OAuth token for whichever grant
// (CC or AC) the panel is on, plus a refresh config the caller can use
// to attempt token refresh. Returns nil + zero-value when the current
// auth type is not OAuth, or when no token has been fetched.
func (a Auth) CurrentOAuthToken() (*oauth.Token, oauth.RefreshConfig, bool) {
	switch a.authType {
	case AuthOAuth:
		if a.oauthToken == nil {
			return nil, oauth.RefreshConfig{}, false
		}
		return a.oauthToken, oauth.RefreshConfig{
			TokenURL:     strings.TrimSpace(a.oauthTokenURL.Value()),
			ClientID:     strings.TrimSpace(a.oauthClientID.Value()),
			ClientSecret: strings.TrimSpace(a.oauthSecret.Value()),
			RefreshToken: a.oauthToken.RefreshToken,
			Scope:        strings.TrimSpace(a.oauthScope.Value()),
		}, true
	case AuthOAuthAC:
		if a.oauthACToken == nil {
			return nil, oauth.RefreshConfig{}, false
		}
		return a.oauthACToken, oauth.RefreshConfig{
			TokenURL:     strings.TrimSpace(a.oauthTokenURL.Value()),
			ClientID:     strings.TrimSpace(a.oauthClientID.Value()),
			ClientSecret: strings.TrimSpace(a.oauthSecret.Value()),
			RefreshToken: a.oauthACToken.RefreshToken,
			Scope:        strings.TrimSpace(a.oauthScope.Value()),
		}, true
	}
	return nil, oauth.RefreshConfig{}, false
}

// ApplyRefreshedToken replaces the currently-active OAuth token with a
// freshly-refreshed one. Routes to the CC or AC slot depending on
// authType.
func (a *Auth) ApplyRefreshedToken(t *oauth.Token) {
	switch a.authType {
	case AuthOAuth:
		a.oauthToken = t
	case AuthOAuthAC:
		a.oauthACToken = t
	}
}

// Reset clears state, called when a history entry is loaded so the previous
// session's Auth doesn't accidentally apply to an unrelated restored request.
func (a *Auth) Reset() {
	a.authType = AuthNone
	a.token.SetValue("")
	a.user.SetValue("")
	a.pass.SetValue("")
	a.oauthTokenURL.SetValue("")
	a.oauthClientID.SetValue("")
	a.oauthSecret.SetValue("")
	a.oauthScope.SetValue("")
	a.oauthToken = nil
	a.oauthError = ""
	a.oauthFetching = false
	a.oauthACAuthURL.SetValue("")
	a.oauthACRedirect.SetValue("http://127.0.0.1:8765/callback")
	a.oauthACToken = nil
	a.oauthACError = ""
	a.oauthACFetching = false
	a.oauthACStatus = ""
	if a.oauthACCancel != nil {
		a.oauthACCancel()
		a.oauthACCancel = nil
	}
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
	a.oauthTokenURL.Blur()
	a.oauthClientID.Blur()
	a.oauthSecret.Blur()
	a.oauthScope.Blur()
	a.oauthACAuthURL.Blur()
	a.oauthACRedirect.Blur()
}

func (a Auth) Focused() bool { return a.focused }

func (a *Auth) refreshFocus() {
	a.token.Blur()
	a.user.Blur()
	a.pass.Blur()
	a.oauthTokenURL.Blur()
	a.oauthClientID.Blur()
	a.oauthSecret.Blur()
	a.oauthScope.Blur()
	a.oauthACAuthURL.Blur()
	a.oauthACRedirect.Blur()
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
	case AuthOAuth:
		switch a.cursor {
		case 1:
			a.oauthTokenURL.Focus()
		case 2:
			a.oauthClientID.Focus()
		case 3:
			a.oauthSecret.Focus()
		case 4:
			a.oauthScope.Focus()
		}
		// cursor == 5 is the action row, no input to focus.
	case AuthOAuthAC:
		switch a.cursor {
		case 1:
			a.oauthACAuthURL.Focus()
		case 2:
			a.oauthTokenURL.Focus()
		case 3:
			a.oauthClientID.Focus()
		case 4:
			a.oauthSecret.Focus()
		case 5:
			a.oauthACRedirect.Focus()
		case 6:
			a.oauthScope.Focus()
		}
		// cursor == 7 is the action row.
	}
}

// authLastCursor returns the last valid cursor index for the current
// auth type. Used by Update's up/down navigation.
func (a Auth) authLastCursor() int {
	switch a.authType {
	case AuthBearer:
		return 1
	case AuthBasic:
		return 2
	case AuthOAuth:
		return 5
	case AuthOAuthAC:
		return 7
	}
	return 0
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
		// Special case: Esc on the AC action row while a flow is in
		// flight cancels the flow rather than moving the cursor. The
		// Cmd goroutine sees the cancelled ctx and returns an
		// AuthOAuthACErrorMsg.
		if a.authType == AuthOAuthAC && a.cursor == 7 && a.oauthACFetching && a.oauthACCancel != nil {
			a.oauthACCancel()
			return a, nil
		}
		a.cursor = 0
		a.refreshFocus()
		return a, nil
	case "up":
		if a.cursor > 1 {
			a.cursor--
		} else {
			a.cursor = 0
		}
		a.refreshFocus()
		return a, nil
	case "down":
		if a.cursor < a.authLastCursor() {
			a.cursor++
			a.refreshFocus()
			return a, nil
		}
	}

	// OAuth-specific action: G on the action row triggers a Client
	// Credentials fetch. The Cmd is dispatched up to the app's Update
	// loop, which on AuthOAuthTokenMsg calls SetOAuthToken.
	if a.authType == AuthOAuth && a.cursor == 5 && (km.String() == "g" || km.String() == "G") {
		if a.oauthFetching {
			return a, nil
		}
		cfg := oauth.ClientCredentialsConfig{
			TokenURL:     strings.TrimSpace(a.oauthTokenURL.Value()),
			ClientID:     strings.TrimSpace(a.oauthClientID.Value()),
			ClientSecret: strings.TrimSpace(a.oauthSecret.Value()),
			Scope:        strings.TrimSpace(a.oauthScope.Value()),
		}
		if cfg.TokenURL == "" || cfg.ClientID == "" {
			a.oauthError = "fill in token URL and client id first"
			return a, nil
		}
		a.oauthFetching = true
		a.oauthError = ""
		return a, oauthFetchCmd(cfg)
	}

	// OAuth AC action: G on the action row starts the full
	// Authorization Code + PKCE flow. Esc on this row while
	// fetching cancels the flow via the stored CancelFunc (handled
	// above).
	if a.authType == AuthOAuthAC && a.cursor == 7 && (km.String() == "g" || km.String() == "G") {
		if a.oauthACFetching {
			return a, nil
		}
		cfg := oauth.AuthCodeConfig{
			AuthURL:      strings.TrimSpace(a.oauthACAuthURL.Value()),
			TokenURL:     strings.TrimSpace(a.oauthTokenURL.Value()),
			ClientID:     strings.TrimSpace(a.oauthClientID.Value()),
			ClientSecret: strings.TrimSpace(a.oauthSecret.Value()),
			RedirectURI:  strings.TrimSpace(a.oauthACRedirect.Value()),
			Scope:        strings.TrimSpace(a.oauthScope.Value()),
		}
		if cfg.AuthURL == "" || cfg.TokenURL == "" || cfg.ClientID == "" || cfg.RedirectURI == "" {
			a.oauthACError = "fill in Auth URL, Token URL, Client ID, and Redirect URI first"
			return a, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		a.oauthACFetching = true
		a.oauthACError = ""
		a.oauthACStatus = "opening browser… (authorize within 5 min, Esc to cancel)"
		a.oauthACCancel = cancel
		return a, oauthACFetchCmd(ctx, cfg)
	}

	var cmd tea.Cmd
	switch {
	case a.authType == AuthBearer && a.cursor == 1:
		a.token, cmd = a.token.Update(msg)
	case a.authType == AuthBasic && a.cursor == 1:
		a.user, cmd = a.user.Update(msg)
	case a.authType == AuthBasic && a.cursor == 2:
		a.pass, cmd = a.pass.Update(msg)
	case a.authType == AuthOAuth && a.cursor == 1:
		a.oauthTokenURL, cmd = a.oauthTokenURL.Update(msg)
	case a.authType == AuthOAuth && a.cursor == 2:
		a.oauthClientID, cmd = a.oauthClientID.Update(msg)
	case a.authType == AuthOAuth && a.cursor == 3:
		a.oauthSecret, cmd = a.oauthSecret.Update(msg)
	case a.authType == AuthOAuth && a.cursor == 4:
		a.oauthScope, cmd = a.oauthScope.Update(msg)
	case a.authType == AuthOAuthAC && a.cursor == 1:
		a.oauthACAuthURL, cmd = a.oauthACAuthURL.Update(msg)
	case a.authType == AuthOAuthAC && a.cursor == 2:
		a.oauthTokenURL, cmd = a.oauthTokenURL.Update(msg)
	case a.authType == AuthOAuthAC && a.cursor == 3:
		a.oauthClientID, cmd = a.oauthClientID.Update(msg)
	case a.authType == AuthOAuthAC && a.cursor == 4:
		a.oauthSecret, cmd = a.oauthSecret.Update(msg)
	case a.authType == AuthOAuthAC && a.cursor == 5:
		a.oauthACRedirect, cmd = a.oauthACRedirect.Update(msg)
	case a.authType == AuthOAuthAC && a.cursor == 6:
		a.oauthScope, cmd = a.oauthScope.Update(msg)
	}
	return a, cmd
}

// renderOAuthStatus formats the action / status line that sits below
// the OAuth input fields. The line shows the current token (truncated
// + expiry) when one's been fetched, the last error if any, the
// "fetching" spinner when in flight, or a prompt to press G.
func (a Auth) renderOAuthStatus() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	ok := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	bad := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	hi := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)

	cursorMark := ""
	if a.focused && a.cursor == 5 {
		cursorMark = hi.Render("▶ ")
	}
	switch {
	case a.oauthFetching:
		return cursorMark + dim.Render("fetching token…")
	case a.oauthError != "":
		return cursorMark + bad.Render("error: "+a.oauthError) + dim.Render("  (press g to retry)")
	case a.oauthToken != nil && a.oauthToken.AccessToken != "":
		tok := a.oauthToken.AccessToken
		var preview string
		// Rune-slice rather than byte-slice: RFC 6749 does not constrain
		// access_token to ASCII, and a multi-byte token chopped at a
		// byte boundary would produce invalid UTF-8 that lipgloss/
		// terminal renderers handle poorly.
		runes := []rune(tok)
		if len(runes) > 16 {
			preview = string(runes[:8]) + "…" + string(runes[len(runes)-4:])
		} else {
			preview = tok
		}
		expiry := ""
		if !a.oauthToken.ExpiresAt.IsZero() {
			remaining := time.Until(a.oauthToken.ExpiresAt).Round(time.Second)
			if remaining > 0 {
				expiry = fmt.Sprintf("  (expires in %s)", remaining)
			} else {
				expiry = "  (expired)"
			}
		}
		return cursorMark + ok.Render("Bearer "+preview) + dim.Render(expiry+"  · press g to refresh")
	default:
		return cursorMark + dim.Render("press g to fetch token (Client Credentials)")
	}
}

// oauthFetchCmd runs the Client Credentials flow off the main loop and
// emits an AuthOAuthTokenMsg / AuthOAuthErrorMsg back to Update.
func oauthFetchCmd(cfg oauth.ClientCredentialsConfig) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		tok, err := oauth.ClientCredentials(ctx, cfg, nil)
		if err != nil {
			return AuthOAuthErrorMsg{Err: err.Error()}
		}
		return AuthOAuthTokenMsg{Token: tok}
	}
}

// oauthACFetchCmd runs the Authorization Code + PKCE flow off the
// main UI goroutine. The ctx is owned by the panel (stored as
// oauthACCancel) so Esc on the action row can cancel mid-flight.
func oauthACFetchCmd(ctx context.Context, cfg oauth.AuthCodeConfig) tea.Cmd {
	return func() tea.Msg {
		tok, err := oauth.AuthorizationCode(ctx, cfg, nil, oauth.OpenBrowser)
		if err != nil {
			return AuthOAuthACErrorMsg{Err: err.Error()}
		}
		return AuthOAuthACTokenMsg{Token: tok}
	}
}

// renderOAuthACStatus formats the action / status line for the
// Authorization Code panel. Same structure as renderOAuthStatus —
// fetching / error / token / prompt.
func (a Auth) renderOAuthACStatus() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	ok := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	bad := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	hi := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)

	cursorMark := ""
	if a.focused && a.cursor == 7 {
		cursorMark = hi.Render("▶ ")
	}
	switch {
	case a.oauthACFetching:
		status := a.oauthACStatus
		if status == "" {
			status = "authorizing…"
		}
		return cursorMark + dim.Render(status)
	case a.oauthACError != "":
		return cursorMark + bad.Render("error: "+a.oauthACError) + dim.Render("  (press g to retry)")
	case a.oauthACToken != nil && a.oauthACToken.AccessToken != "":
		tok := a.oauthACToken.AccessToken
		var preview string
		runes := []rune(tok)
		if len(runes) > 16 {
			preview = string(runes[:8]) + "…" + string(runes[len(runes)-4:])
		} else {
			preview = tok
		}
		expiry := ""
		if !a.oauthACToken.ExpiresAt.IsZero() {
			remaining := time.Until(a.oauthACToken.ExpiresAt).Round(time.Second)
			if remaining > 0 {
				expiry = fmt.Sprintf("  (expires in %s)", remaining)
			} else {
				expiry = "  (expired)"
			}
		}
		return cursorMark + ok.Render("Bearer "+preview) + dim.Render(expiry+"  · press g to re-authorize")
	default:
		return cursorMark + dim.Render("press g to authorize (Auth Code + PKCE)")
	}
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
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	prefix := dim.Render("Auth") + "  "

	// Build the full bar first so we can measure it. Collapse to the
	// selected tab wrapped in ‹ › when it would overflow `inner` and
	// wrap onto a second line (which pushes the input fields below out
	// of the panel).
	var tabsBuf strings.Builder
	selStyle := lipgloss.NewStyle().Padding(0, 1)
	for i, t := range authTypes {
		label := t.String()
		s := lipgloss.NewStyle().Padding(0, 1)
		switch {
		case t == a.authType && a.focused && a.cursor == 0:
			s = s.Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0"))
			selStyle = selStyle.Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0"))
		case t == a.authType:
			s = s.Background(lipgloss.Color("240")).Foreground(lipgloss.Color("230"))
			if !(a.focused && a.cursor == 0) {
				selStyle = selStyle.Background(lipgloss.Color("240")).Foreground(lipgloss.Color("230"))
			}
		default:
			s = s.Foreground(lipgloss.Color("244"))
		}
		tabsBuf.WriteString(s.Render(label))
		if i < len(authTypes)-1 {
			tabsBuf.WriteString(" ")
		}
	}
	fullBar := prefix + tabsBuf.String()
	if lipgloss.Width(fullBar) > inner {
		sb.WriteString(prefix)
		sb.WriteString(dim.Render("‹"))
		sb.WriteString(selStyle.Render(a.authType.String()))
		sb.WriteString(dim.Render("›"))
	} else {
		sb.WriteString(fullBar)
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
	case AuthOAuth:
		a.oauthTokenURL.Width = inputW
		a.oauthClientID.Width = inputW
		a.oauthSecret.Width = inputW
		a.oauthScope.Width = inputW
		sb.WriteString("\n  Token URL:    ")
		sb.WriteString(a.oauthTokenURL.View())
		sb.WriteString("\n  Client ID:    ")
		sb.WriteString(a.oauthClientID.View())
		sb.WriteString("\n  Client Secret:")
		sb.WriteString(a.oauthSecret.View())
		sb.WriteString("\n  Scope:        ")
		sb.WriteString(a.oauthScope.View())
		sb.WriteString("\n  ")
		sb.WriteString(a.renderOAuthStatus())
	case AuthOAuthAC:
		a.oauthACAuthURL.Width = inputW
		a.oauthTokenURL.Width = inputW
		a.oauthClientID.Width = inputW
		a.oauthSecret.Width = inputW
		a.oauthACRedirect.Width = inputW
		a.oauthScope.Width = inputW
		sb.WriteString("\n  Auth URL:     ")
		sb.WriteString(a.oauthACAuthURL.View())
		sb.WriteString("\n  Token URL:    ")
		sb.WriteString(a.oauthTokenURL.View())
		sb.WriteString("\n  Client ID:    ")
		sb.WriteString(a.oauthClientID.View())
		sb.WriteString("\n  Client Secret:")
		sb.WriteString(a.oauthSecret.View())
		sb.WriteString("\n  Redirect URI: ")
		sb.WriteString(a.oauthACRedirect.View())
		sb.WriteString("\n  Scope:        ")
		sb.WriteString(a.oauthScope.View())
		sb.WriteString("\n  ")
		sb.WriteString(a.renderOAuthACStatus())
	}

	var hint string
	if a.focused {
		switch {
		case a.cursor == 0 && a.authType == AuthNone:
			hint = "  ←/→: select type"
		case a.cursor == 0:
			hint = "  ←/→: type  ·  Enter/↓: edit"
		case a.authType == AuthOAuth && a.cursor == 5:
			hint = "  g: fetch token (Client Credentials)  ·  Esc/↑: back"
		case a.authType == AuthOAuthAC && a.cursor == 7 && a.oauthACFetching:
			hint = "  Esc: cancel"
		case a.authType == AuthOAuthAC && a.cursor == 7:
			hint = "  g: authorize (Auth Code + PKCE)  ·  Esc/↑: back"
		default:
			hint = "  Esc/↑: back to type  ·  ↓: next field"
		}
	}
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(hint))

	return border.Render(sb.String())
}
