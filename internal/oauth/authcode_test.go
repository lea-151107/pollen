package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGeneratePKCE_VerifierIs43Chars(t *testing.T) {
	v, c, err := generatePKCE()
	if err != nil {
		t.Fatalf("generatePKCE: %v", err)
	}
	if len(v) != 43 {
		t.Errorf("verifier should be 43 chars (base64url of 32B), got %d", len(v))
	}
	if len(c) != 43 {
		t.Errorf("challenge should be 43 chars, got %d", len(c))
	}
	// challenge must equal base64url(sha256(verifier))
	sum := sha256.Sum256([]byte(v))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if c != want {
		t.Errorf("challenge does not match S256 of verifier")
	}
}

func TestGeneratePKCE_DistinctRuns(t *testing.T) {
	v1, _, _ := generatePKCE()
	v2, _, _ := generatePKCE()
	if v1 == v2 {
		t.Errorf("two PKCE generations produced identical verifier")
	}
}

func TestGenerateState_Length(t *testing.T) {
	s, err := generateState()
	if err != nil {
		t.Fatalf("generateState: %v", err)
	}
	if len(s) != 43 {
		t.Errorf("state should be 43 chars, got %d", len(s))
	}
}

func TestParseLoopback_Variants(t *testing.T) {
	cases := []struct {
		in       string
		wantPort int
		wantPath string
		wantErr  bool
	}{
		{"http://127.0.0.1:8765/callback", 8765, "/callback", false},
		{"http://localhost:9000/cb", 9000, "/cb", false},
		{"http://[::1]:8000/", 8000, "/", false},
		{"http://127.0.0.1:8765", 8765, "/", false},
		// no port
		{"http://127.0.0.1/callback", 0, "", true},
		// non-loopback host
		{"http://example.com:8765/cb", 0, "", true},
		// https
		{"https://127.0.0.1:8765/cb", 0, "", true},
		// bad URL
		{"::not-a-url", 0, "", true},
		// port out of range
		{"http://127.0.0.1:99999/", 0, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			port, path, err := parseLoopback(tc.in)
			gotErr := err != nil
			if gotErr != tc.wantErr {
				t.Fatalf("parseLoopback(%q): err=%v wantErr=%v", tc.in, err, tc.wantErr)
			}
			if !tc.wantErr {
				if port != tc.wantPort {
					t.Errorf("port = %d, want %d", port, tc.wantPort)
				}
				if path != tc.wantPath {
					t.Errorf("path = %q, want %q", path, tc.wantPath)
				}
			}
		})
	}
}

// freeLoopbackPort returns a port that was free at the moment of
// the call. There's a tiny race between Close and the caller's
// re-bind but it's stable enough for unit tests.
func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func TestAuthorizationCode_EndToEnd(t *testing.T) {
	port := freeLoopbackPort(t)
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// Fake IdP token endpoint. Verifies that the POST body carries
	// grant_type=authorization_code, the redirect_uri pollen used,
	// the code we (as the IdP) handed out, and a code_verifier whose
	// SHA-256 base64url equals the challenge we'll observe in the
	// browser-launch URL.
	var capturedChallenge string
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if got := r.PostForm.Get("grant_type"); got != "authorization_code" {
			t.Errorf("token POST grant_type = %q, want authorization_code", got)
		}
		if got := r.PostForm.Get("code"); got != "the-code" {
			t.Errorf("token POST code = %q, want the-code", got)
		}
		if got := r.PostForm.Get("redirect_uri"); got != redirectURI {
			t.Errorf("token POST redirect_uri = %q, want %q", got, redirectURI)
		}
		verifier := r.PostForm.Get("code_verifier")
		if verifier == "" {
			t.Errorf("token POST missing code_verifier")
		}
		sum := sha256.Sum256([]byte(verifier))
		if got := base64.RawURLEncoding.EncodeToString(sum[:]); got != capturedChallenge {
			t.Errorf("verifier does not hash to captured challenge")
		}
		// Public client: client_id in form (we set ClientSecret="").
		if got := r.PostForm.Get("client_id"); got != "test-client" {
			t.Errorf("client_id = %q, want test-client", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"AT","token_type":"Bearer","expires_in":3600,"refresh_token":"RT"}`))
	}))
	defer idp.Close()

	cfg := AuthCodeConfig{
		AuthURL:     "https://idp.example.com/authorize",
		TokenURL:    idp.URL,
		ClientID:    "test-client",
		RedirectURI: redirectURI,
		Scope:       "read",
	}

	// "openBrowser" substitute: parse the auth URL, extract state &
	// code_challenge, fire a GET at our callback with code=the-code
	// and the same state — simulating the IdP's redirect after the
	// user authenticates.
	browserHit := make(chan struct{})
	openBrowser := func(authURL string) error {
		u, err := url.Parse(authURL)
		if err != nil {
			return err
		}
		q := u.Query()
		capturedChallenge = q.Get("code_challenge")
		if q.Get("code_challenge_method") != "S256" {
			t.Errorf("code_challenge_method = %q, want S256", q.Get("code_challenge_method"))
		}
		state := q.Get("state")
		go func() {
			defer close(browserHit)
			// Tiny delay so the listener is definitely up; in practice
			// the goroutine that hits ln has to race with srv.Serve.
			// The retry loop below handles port-not-yet-listening.
			callbackURL := fmt.Sprintf("%s?code=the-code&state=%s", redirectURI, url.QueryEscape(state))
			var resp *http.Response
			var err error
			for i := 0; i < 50; i++ {
				resp, err = http.Get(callbackURL)
				if err == nil {
					_ = resp.Body.Close()
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Errorf("could not hit callback: %v", err)
		}()
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tok, err := AuthorizationCode(ctx, cfg, nil, openBrowser)
	if err != nil {
		t.Fatalf("AuthorizationCode: %v", err)
	}
	<-browserHit
	if tok.AccessToken != "AT" {
		t.Errorf("access_token = %q, want AT", tok.AccessToken)
	}
	if tok.RefreshToken != "RT" {
		t.Errorf("refresh_token = %q, want RT", tok.RefreshToken)
	}
	if tok.ExpiresAt.IsZero() {
		t.Errorf("ExpiresAt should be set from expires_in")
	}
}

func TestAuthorizationCode_StateMismatch(t *testing.T) {
	port := freeLoopbackPort(t)
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/cb", port)
	cfg := AuthCodeConfig{
		AuthURL:     "https://idp.example.com/authorize",
		TokenURL:    "https://idp.example.com/token",
		ClientID:    "x",
		RedirectURI: redirectURI,
	}
	openBrowser := func(authURL string) error {
		go func() {
			for i := 0; i < 50; i++ {
				resp, err := http.Get(fmt.Sprintf("%s?code=c&state=WRONG", redirectURI))
				if err == nil {
					_ = resp.Body.Close()
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := AuthorizationCode(ctx, cfg, nil, openBrowser)
	if err == nil || !strings.Contains(err.Error(), "state mismatch") {
		t.Fatalf("expected state mismatch error, got %v", err)
	}
}

func TestAuthorizationCode_Cancelled(t *testing.T) {
	port := freeLoopbackPort(t)
	cfg := AuthCodeConfig{
		AuthURL:     "https://idp.example.com/authorize",
		TokenURL:    "https://idp.example.com/token",
		ClientID:    "x",
		RedirectURI: fmt.Sprintf("http://127.0.0.1:%d/cb", port),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	noopOpen := func(string) error { return nil }
	_, err := AuthorizationCode(ctx, cfg, nil, noopOpen)
	if err == nil || !strings.Contains(err.Error(), "callback wait") {
		t.Fatalf("expected callback wait error on cancellation, got %v", err)
	}
}

func TestAuthorizationCode_IdPErrorResponse(t *testing.T) {
	port := freeLoopbackPort(t)
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/cb", port)
	cfg := AuthCodeConfig{
		AuthURL:     "https://idp.example.com/authorize",
		TokenURL:    "https://idp.example.com/token",
		ClientID:    "x",
		RedirectURI: redirectURI,
	}
	openBrowser := func(authURL string) error {
		u, _ := url.Parse(authURL)
		state := u.Query().Get("state")
		go func() {
			for i := 0; i < 50; i++ {
				resp, err := http.Get(fmt.Sprintf("%s?error=access_denied&error_description=user+rejected&state=%s",
					redirectURI, url.QueryEscape(state)))
				if err == nil {
					_ = resp.Body.Close()
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := AuthorizationCode(ctx, cfg, nil, openBrowser)
	if err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Fatalf("expected access_denied error, got %v", err)
	}
}

func TestAuthorizationCode_ValidationErrors(t *testing.T) {
	noop := func(string) error { return nil }
	ctx := context.Background()
	cases := []struct {
		name string
		cfg  AuthCodeConfig
		want string
	}{
		{"no auth url", AuthCodeConfig{TokenURL: "x", ClientID: "y", RedirectURI: "http://127.0.0.1:1/"}, "auth_url"},
		{"no token url", AuthCodeConfig{AuthURL: "x", ClientID: "y", RedirectURI: "http://127.0.0.1:1/"}, "token_url"},
		{"no client id", AuthCodeConfig{AuthURL: "x", TokenURL: "y", RedirectURI: "http://127.0.0.1:1/"}, "client_id"},
		{"no redirect", AuthCodeConfig{AuthURL: "x", TokenURL: "y", ClientID: "z"}, "redirect_uri"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AuthorizationCode(ctx, tc.cfg, nil, noop)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

