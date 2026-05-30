package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea-151107/pollen/internal/oauth"
	"github.com/lea-151107/pollen/internal/ui"
)

// refreshSkew is how early before expiry pollen will refresh on
// send. 30 seconds absorbs the refresh + send RTT so the bearer
// header pollen attaches isn't accepted by the server but then
// rejected mid-call as the clock crosses ExpiresAt.
const refreshSkew = 30 * time.Second

// authRefreshedSendMsg signals that an expiring OAuth token was
// refreshed successfully and the original send should now run.
type authRefreshedSendMsg struct {
	Token *oauth.Token
}

// authRefreshFailedMsg signals that refresh failed; the send is
// aborted so the user can re-authorize.
type authRefreshFailedMsg struct {
	Err string
}

// needsRefresh reports whether the auth panel's current OAuth
// token is near-expiry AND has a refresh_token available. Returns
// the refresh config to use plus true when refresh should run
// before the send; returns false in every other case (no OAuth,
// no token, no refresh_token, or token not expiring).
func needsRefresh(a ui.Auth) (oauth.RefreshConfig, bool) {
	tok, cfg, ok := a.CurrentOAuthToken()
	if !ok {
		return oauth.RefreshConfig{}, false
	}
	if tok.RefreshToken == "" {
		return oauth.RefreshConfig{}, false
	}
	if !tok.IsExpired(refreshSkew) {
		return oauth.RefreshConfig{}, false
	}
	return cfg, true
}

// refreshThenSendCmd performs the refresh in a goroutine and
// dispatches authRefreshedSendMsg on success or
// authRefreshFailedMsg on failure. The app's Update routes the
// success message back into ApplyRefreshedToken + sendRequest.
func refreshThenSendCmd(cfg oauth.RefreshConfig) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		tok, err := oauth.Refresh(ctx, cfg, nil)
		if err != nil {
			return authRefreshFailedMsg{Err: err.Error()}
		}
		return authRefreshedSendMsg{Token: tok}
	}
}
