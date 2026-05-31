// Package oauth implements the OAuth 2.0 token flows pollen ships in
// v1.5.0: Client Credentials (RFC 6749 §4.4) and Refresh Token
// (RFC 6749 §6). Authorization Code with PKCE is reserved for v1.6 —
// it needs a localhost callback server and browser launching that
// don't compose cleanly with the rest of pollen's TUI loop.
//
// The package is deliberately decoupled from httpx: callers inject a
// Doer so tests can drive the flows against httptest.NewServer without
// reaching for the real net stack.
package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Token is the parsed result of a successful token endpoint exchange.
// ExpiresAt is computed from expires_in at the moment the response is
// parsed (server-clock independent); a zero value means the server did
// not provide expires_in and the caller should treat the token as
// non-expiring until proven otherwise.
type Token struct {
	AccessToken  string
	TokenType    string
	RefreshToken string
	ExpiresAt    time.Time
	Scope        string
}

// IsExpired reports whether the access token is past its expiry.
// A small skew lets the caller refresh slightly early.
func (t *Token) IsExpired(skew time.Duration) bool {
	if t == nil || t.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().Add(skew).After(t.ExpiresAt)
}

// Doer mirrors a tiny slice of net/http: build a POST and read its
// body. Production callers wrap http.Client.Do; tests substitute a
// closure over an httptest server.
type Doer func(req *http.Request) (status int, body []byte, err error)

// DefaultDoer adapts net/http.DefaultClient (with a sensible timeout)
// to the Doer signature. Callers that need pollen's transport
// settings (TLS skip, proxy, CA cert) should build their own Doer
// over httpx instead.
func DefaultDoer() Doer {
	c := &http.Client{Timeout: 30 * time.Second}
	return func(req *http.Request) (int, []byte, error) {
		resp, err := c.Do(req)
		if err != nil {
			return 0, nil, err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return resp.StatusCode, nil, err
		}
		return resp.StatusCode, b, nil
	}
}

// ClientCredentialsConfig collects the inputs for an RFC 6749 §4.4
// flow. Audience / ExtraParams are optional add-ons for vendors that
// extend the spec (Auth0 audience, Okta resource, etc.).
type ClientCredentialsConfig struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scope        string
	Audience     string
	ExtraParams  map[string]string
}

// ClientCredentials exchanges client credentials for an access token
// per RFC 6749 §4.4. The client id / secret go in the Basic-Auth
// header per §2.3.1; servers that accept body-form credentials also
// accept this form, so it works universally.
func ClientCredentials(ctx context.Context, cfg ClientCredentialsConfig, doer Doer) (*Token, error) {
	if cfg.TokenURL == "" {
		return nil, fmt.Errorf("oauth: token_url is required")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("oauth: client_id is required")
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	if cfg.Scope != "" {
		form.Set("scope", cfg.Scope)
	}
	if cfg.Audience != "" {
		form.Set("audience", cfg.Audience)
	}
	for k, v := range cfg.ExtraParams {
		form.Set(k, v)
	}
	return postForm(ctx, cfg.TokenURL, cfg.ClientID, cfg.ClientSecret, form, doer)
}

// RefreshConfig collects the inputs for an RFC 6749 §6 refresh.
// ClientSecret is optional for public clients (the spec lets public
// clients omit it when authentication isn't required for the client
// type).
type RefreshConfig struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	RefreshToken string
	Scope        string
}

// Refresh exchanges a refresh token for a new access token per
// RFC 6749 §6. The authorization server MAY rotate the refresh
// token: when it does, the new RefreshToken comes back in the
// response and supersedes the old one. When it does NOT (omits the
// refresh_token field, permitted by §6), the client implicitly
// continues using the existing one — Refresh fills the returned
// Token's RefreshToken from cfg so callers (notably the v1.6.4
// disk-persistence path) don't accidentally store an empty value
// and lose the ability to refresh again. Google OAuth is the
// canonical example of a non-rotating IdP.
func Refresh(ctx context.Context, cfg RefreshConfig, doer Doer) (*Token, error) {
	if cfg.TokenURL == "" {
		return nil, fmt.Errorf("oauth: token_url is required")
	}
	if cfg.RefreshToken == "" {
		return nil, fmt.Errorf("oauth: refresh_token is required")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", cfg.RefreshToken)
	if cfg.Scope != "" {
		form.Set("scope", cfg.Scope)
	}
	if cfg.ClientID != "" {
		form.Set("client_id", cfg.ClientID)
	}
	t, err := postForm(ctx, cfg.TokenURL, cfg.ClientID, cfg.ClientSecret, form, doer)
	if err != nil {
		return nil, err
	}
	if t.RefreshToken == "" {
		t.RefreshToken = cfg.RefreshToken
	}
	return t, nil
}

// postForm POSTs application/x-www-form-urlencoded body with optional
// Basic-Auth headers, parses the token JSON response, and surfaces
// errors with the server's error_description when present.
func postForm(ctx context.Context, tokenURL, clientID, clientSecret string, form url.Values, doer Doer) (*Token, error) {
	body := strings.NewReader(form.Encode())
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, body)
	if err != nil {
		return nil, fmt.Errorf("oauth: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if clientID != "" && clientSecret != "" {
		creds := clientID + ":" + clientSecret
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(creds)))
	}
	if doer == nil {
		doer = DefaultDoer()
	}
	status, raw, err := doer(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: %w", err)
	}
	if status < 200 || status >= 300 {
		// RFC 6749 §5.2: error responses are JSON with `error` and
		// optional `error_description`. Surface the description when
		// present so the user sees something useful in the TUI.
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if json.Unmarshal(raw, &errResp) == nil && errResp.Error != "" {
			msg := errResp.Error
			if errResp.ErrorDescription != "" {
				msg += ": " + errResp.ErrorDescription
			}
			return nil, fmt.Errorf("oauth: token endpoint returned %d: %s", status, msg)
		}
		return nil, fmt.Errorf("oauth: token endpoint returned %d", status)
	}
	var parsed struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("oauth: parse token response: %w", err)
	}
	if parsed.AccessToken == "" {
		return nil, fmt.Errorf("oauth: token response missing access_token")
	}
	t := &Token{
		AccessToken:  parsed.AccessToken,
		TokenType:    parsed.TokenType,
		RefreshToken: parsed.RefreshToken,
		Scope:        parsed.Scope,
	}
	if parsed.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second)
	}
	return t, nil
}
