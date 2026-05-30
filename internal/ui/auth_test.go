package ui

import (
	"strings"
	"testing"

	"github.com/lea-151107/pollen/internal/oauth"
)

func TestAuth_Defaults(t *testing.T) {
	a := NewAuth()
	if a.Type() != AuthNone {
		t.Errorf("default type should be AuthNone, got %v", a.Type())
	}
	if a.Token() != "" {
		t.Errorf("default token should be empty, got %q", a.Token())
	}
	u, p := a.Credentials()
	if u != "" || p != "" {
		t.Errorf("default credentials should be empty, got %q/%q", u, p)
	}
}

func TestAuth_TokenTrim(t *testing.T) {
	a := NewAuth()
	a.authType = AuthBearer
	a.token.SetValue("  tok  ")
	if got := a.Token(); got != "tok" {
		t.Errorf("Token should trim whitespace, got %q", got)
	}
}

func TestAuth_Credentials(t *testing.T) {
	a := NewAuth()
	a.authType = AuthBasic
	a.user.SetValue("alice")
	a.pass.SetValue("secret")
	u, p := a.Credentials()
	if u != "alice" || p != "secret" {
		t.Errorf("got %q/%q want alice/secret", u, p)
	}
}

func TestAuth_View_CollapsesTabsAtNarrowWidth(t *testing.T) {
	a := NewAuth()
	a.authType = AuthBearer
	got := a.View(20)
	if !strings.Contains(got, "‹") || !strings.Contains(got, "›") {
		t.Errorf("expected collapsed bar with ‹ ›, got:\n%s", got)
	}
	if !strings.Contains(got, "Bearer") {
		t.Errorf("collapsed bar should still show selected label Bearer, got:\n%s", got)
	}
	// The "Auth" orientation label is kept.
	if !strings.Contains(got, "Auth") {
		t.Errorf("collapsed bar should still show Auth label, got:\n%s", got)
	}
	// Other tabs must NOT appear.
	if strings.Contains(got, "OAuth") || strings.Contains(got, "Basic") {
		t.Errorf("collapsed bar should NOT include other tabs, got:\n%s", got)
	}
}

func TestAuth_View_FullTabsAtWideWidth(t *testing.T) {
	a := NewAuth()
	a.authType = AuthBearer
	got := a.View(120)
	for _, label := range []string{"None", "Bearer", "Basic", "OAuth"} {
		if !strings.Contains(got, label) {
			t.Errorf("wide-width bar missing %q, got:\n%s", label, got)
		}
	}
	if strings.Contains(got, "‹") || strings.Contains(got, "›") {
		t.Errorf("wide-width bar should NOT collapse with ‹ ›, got:\n%s", got)
	}
}

func TestAuth_OAuthPreview_RuneSliceMultibyte(t *testing.T) {
	// "桜" is a 3-byte rune. If the preview chops at byte index 8 it
	// would split a rune mid-sequence and produce invalid UTF-8.
	tok := strings.Repeat("桜", 20)
	a := NewAuth()
	a.authType = AuthOAuth
	a.oauthToken = &oauth.Token{AccessToken: tok}
	got := a.renderOAuthStatus()
	// 8-rune prefix + ellipsis + 4-rune suffix
	wantPrefix := strings.Repeat("桜", 8)
	wantSuffix := strings.Repeat("桜", 4)
	if !strings.Contains(got, wantPrefix+"…"+wantSuffix) {
		t.Errorf("preview did not rune-slice multi-byte token:\n got: %q", got)
	}
	if !strings.ContainsRune(got, '桜') {
		t.Errorf("rendered preview lost the multi-byte rune entirely: %q", got)
	}
}

func TestAuth_OAuthPreview_ShortToken(t *testing.T) {
	// Tokens at or below 16 runes are shown verbatim.
	a := NewAuth()
	a.authType = AuthOAuth
	a.oauthToken = &oauth.Token{AccessToken: "short"}
	got := a.renderOAuthStatus()
	if !strings.Contains(got, "short") {
		t.Errorf("short token should appear verbatim, got: %q", got)
	}
	if strings.Contains(got, "…") {
		t.Errorf("short token should not be elided, got: %q", got)
	}
}

func TestAuth_OAuthAC_DefaultsAndCursorRange(t *testing.T) {
	a := NewAuth()
	if got := a.oauthACRedirect.Value(); got != "http://127.0.0.1:8765/callback" {
		t.Errorf("default Redirect URI = %q, want loopback default", got)
	}
	a.authType = AuthOAuthAC
	if got := a.authLastCursor(); got != 7 {
		t.Errorf("OAuth AC authLastCursor = %d, want 7", got)
	}
}

func TestAuth_OAuthAC_ResetClearsFlowState(t *testing.T) {
	a := NewAuth()
	a.authType = AuthOAuthAC
	a.oauthACAuthURL.SetValue("https://x")
	a.oauthACRedirect.SetValue("http://127.0.0.1:9000/cb")
	a.oauthACError = "boom"
	a.oauthACFetching = true
	a.oauthACStatus = "fetching"
	a.Reset()
	if a.oauthACAuthURL.Value() != "" {
		t.Errorf("Reset should clear Auth URL")
	}
	if a.oauthACRedirect.Value() != "http://127.0.0.1:8765/callback" {
		t.Errorf("Reset should restore default Redirect URI, got %q", a.oauthACRedirect.Value())
	}
	if a.oauthACError != "" || a.oauthACFetching || a.oauthACStatus != "" {
		t.Errorf("Reset should clear flow state")
	}
}

func TestAuth_View_TabBarIncludesOAuthAC(t *testing.T) {
	a := NewAuth()
	got := a.View(120)
	if !strings.Contains(got, "OAuth AC") {
		t.Errorf("wide-width tab strip missing OAuth AC, got:\n%s", got)
	}
}

func TestAuth_Reset(t *testing.T) {
	a := NewAuth()
	a.authType = AuthBearer
	a.token.SetValue("tok")
	a.Reset()
	if a.Type() != AuthNone {
		t.Errorf("type not reset")
	}
	if a.Token() != "" {
		t.Errorf("token not cleared")
	}
}
