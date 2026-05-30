package app

import (
	"testing"
	"time"

	"github.com/lea-151107/pollen/internal/oauth"
	"github.com/lea-151107/pollen/internal/ui"
)

func TestNeedsRefresh_NonOAuthSkipped(t *testing.T) {
	a := ui.NewAuth()
	if _, ok := needsRefresh(a); ok {
		t.Errorf("AuthNone should never need refresh")
	}
}

func TestNeedsRefresh_NoTokenSkipped(t *testing.T) {
	a := ui.NewAuth()
	a.SetType(ui.AuthOAuth)
	if _, ok := needsRefresh(a); ok {
		t.Errorf("AuthOAuth with no token should not need refresh")
	}
}

func TestNeedsRefresh_NoRefreshTokenSkipped(t *testing.T) {
	a := ui.NewAuth()
	a.SetType(ui.AuthOAuth)
	a.SetOAuthToken(&oauth.Token{
		AccessToken:  "AT",
		RefreshToken: "", // no refresh token
		ExpiresAt:    time.Now().Add(-time.Hour),
	})
	if _, ok := needsRefresh(a); ok {
		t.Errorf("token with no refresh_token should not trigger refresh")
	}
}

func TestNeedsRefresh_NotExpiredSkipped(t *testing.T) {
	a := ui.NewAuth()
	a.SetType(ui.AuthOAuth)
	a.SetOAuthToken(&oauth.Token{
		AccessToken:  "AT",
		RefreshToken: "RT",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	})
	if _, ok := needsRefresh(a); ok {
		t.Errorf("non-expired token should not need refresh")
	}
}

func TestNeedsRefresh_ZeroExpiresAtSkipped(t *testing.T) {
	a := ui.NewAuth()
	a.SetType(ui.AuthOAuth)
	a.SetOAuthToken(&oauth.Token{
		AccessToken:  "AT",
		RefreshToken: "RT",
		// IsZero — server returned no expires_in, IsExpired returns false.
	})
	if _, ok := needsRefresh(a); ok {
		t.Errorf("server-non-expiring token (zero ExpiresAt) should not refresh proactively")
	}
}

func TestNeedsRefresh_ExpiredCC(t *testing.T) {
	a := ui.NewAuth()
	a.SetType(ui.AuthOAuth)
	a.SetOAuthToken(&oauth.Token{
		AccessToken:  "AT",
		RefreshToken: "RT",
		ExpiresAt:    time.Now().Add(-time.Second),
	})
	if _, ok := needsRefresh(a); !ok {
		t.Fatalf("expired CC token with refresh_token should need refresh")
	}
}

func TestNeedsRefresh_ExpiredAC(t *testing.T) {
	a := ui.NewAuth()
	a.SetType(ui.AuthOAuthAC)
	a.SetOAuthACToken(&oauth.Token{
		AccessToken:  "AT",
		RefreshToken: "RT",
		ExpiresAt:    time.Now().Add(-time.Second),
	})
	if _, ok := needsRefresh(a); !ok {
		t.Fatalf("expired AC token with refresh_token should need refresh")
	}
}

func TestNeedsRefresh_NearExpiryWithinSkewTriggers(t *testing.T) {
	a := ui.NewAuth()
	a.SetType(ui.AuthOAuth)
	// 10s remaining — within the 30s skew, so refresh should fire.
	a.SetOAuthToken(&oauth.Token{
		AccessToken:  "AT",
		RefreshToken: "RT",
		ExpiresAt:    time.Now().Add(10 * time.Second),
	})
	if _, ok := needsRefresh(a); !ok {
		t.Errorf("token within refreshSkew should trigger refresh")
	}
}

func TestRefreshSkewIsThirtySeconds(t *testing.T) {
	// Pin the constant so a careless tweak doesn't quietly change
	// when pollen pre-emptively refreshes.
	if refreshSkew != 30*time.Second {
		t.Errorf("refreshSkew = %v, want 30s", refreshSkew)
	}
}
