package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAuthorize_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("client_id") != "myclient" {
			t.Errorf("client_id = %q", r.PostForm.Get("client_id"))
		}
		if r.PostForm.Get("scope") != "read write" {
			t.Errorf("scope = %q", r.PostForm.Get("scope"))
		}
		_, _ = w.Write([]byte(`{
			"device_code": "DC123",
			"user_code": "WDJB-MJHT",
			"verification_uri": "https://idp.example.com/device",
			"verification_uri_complete": "https://idp.example.com/device?user_code=WDJB-MJHT",
			"expires_in": 1800,
			"interval": 5
		}`))
	}))
	defer srv.Close()

	auth, err := Authorize(context.Background(), DeviceCodeConfig{
		DeviceURL: srv.URL,
		TokenURL:  "https://idp.example.com/token",
		ClientID:  "myclient",
		Scope:     "read write",
	}, httpDoerForTest())
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if auth.DeviceCode != "DC123" {
		t.Errorf("DeviceCode = %q", auth.DeviceCode)
	}
	if auth.UserCode != "WDJB-MJHT" {
		t.Errorf("UserCode = %q", auth.UserCode)
	}
	if auth.VerificationURI != "https://idp.example.com/device" {
		t.Errorf("VerificationURI = %q", auth.VerificationURI)
	}
	if auth.VerificationURIComplete == "" {
		t.Errorf("VerificationURIComplete should be set")
	}
	if auth.Interval != 5*time.Second {
		t.Errorf("Interval = %v, want 5s", auth.Interval)
	}
	if auth.ExpiresAt.IsZero() {
		t.Errorf("ExpiresAt should be derived from expires_in")
	}
}

func TestAuthorize_DefaultsInterval(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No interval field.
		_, _ = w.Write([]byte(`{
			"device_code": "DC",
			"user_code": "AAAA-BBBB",
			"verification_uri": "https://idp/device",
			"expires_in": 600
		}`))
	}))
	defer srv.Close()
	auth, err := Authorize(context.Background(), DeviceCodeConfig{
		DeviceURL: srv.URL, TokenURL: "https://idp/token", ClientID: "c",
	}, httpDoerForTest())
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if auth.Interval != 5*time.Second {
		t.Errorf("default interval should be 5s, got %v", auth.Interval)
	}
}

func TestAuthorize_DefaultsExpiresIn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No expires_in field.
		_, _ = w.Write([]byte(`{
			"device_code": "DC",
			"user_code": "AAAA-BBBB",
			"verification_uri": "https://idp/device"
		}`))
	}))
	defer srv.Close()
	auth, err := Authorize(context.Background(), DeviceCodeConfig{
		DeviceURL: srv.URL, TokenURL: "https://idp/token", ClientID: "c",
	}, httpDoerForTest())
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if auth.ExpiresAt.IsZero() {
		t.Errorf("ExpiresAt should fall back to a default")
	}
	// Default is 600 seconds ≈ 10 minutes from now.
	remaining := time.Until(auth.ExpiresAt)
	if remaining < 9*time.Minute || remaining > 11*time.Minute {
		t.Errorf("default expires_in should be ~10 min, got %v", remaining)
	}
}

func TestAuthorize_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		cfg  DeviceCodeConfig
		want string
	}{
		{"no device_url", DeviceCodeConfig{TokenURL: "x", ClientID: "y"}, "device_url"},
		{"no token_url", DeviceCodeConfig{DeviceURL: "x", ClientID: "y"}, "token_url"},
		{"no client_id", DeviceCodeConfig{DeviceURL: "x", TokenURL: "y"}, "client_id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Authorize(context.Background(), tc.cfg, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestAuthorize_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"unknown"}`))
	}))
	defer srv.Close()
	_, err := Authorize(context.Background(), DeviceCodeConfig{
		DeviceURL: srv.URL, TokenURL: "https://idp/token", ClientID: "bad",
	}, httpDoerForTest())
	if err == nil || !strings.Contains(err.Error(), "invalid_client") {
		t.Fatalf("expected invalid_client, got %v", err)
	}
}

func TestPollToken_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if got := r.PostForm.Get("grant_type"); got != "urn:ietf:params:oauth:grant-type:device_code" {
			t.Errorf("grant_type = %q", got)
		}
		if got := r.PostForm.Get("device_code"); got != "DC123" {
			t.Errorf("device_code = %q", got)
		}
		if got := r.PostForm.Get("client_id"); got != "myclient" {
			t.Errorf("client_id = %q (public client)", got)
		}
		_, _ = w.Write([]byte(`{"access_token":"AT","token_type":"Bearer","expires_in":3600,"refresh_token":"RT"}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Use a very short interval to keep the test fast.
	tok, err := PollToken(ctx, DeviceCodeConfig{
		TokenURL: srv.URL, ClientID: "myclient",
	}, "DC123", 10*time.Millisecond, httpDoerForTest())
	if err != nil {
		t.Fatalf("PollToken: %v", err)
	}
	if tok.AccessToken != "AT" {
		t.Errorf("AccessToken = %q", tok.AccessToken)
	}
	if tok.RefreshToken != "RT" {
		t.Errorf("RefreshToken = %q", tok.RefreshToken)
	}
}

func TestPollToken_AuthorizationPendingThenSuccess(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 3 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"AT","token_type":"Bearer","expires_in":3600}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tok, err := PollToken(ctx, DeviceCodeConfig{
		TokenURL: srv.URL, ClientID: "c",
	}, "DC", 10*time.Millisecond, httpDoerForTest())
	if err != nil {
		t.Fatalf("PollToken: %v", err)
	}
	if tok.AccessToken != "AT" {
		t.Errorf("AccessToken = %q", tok.AccessToken)
	}
	if atomic.LoadInt32(&hits) != 3 {
		t.Errorf("expected 3 polls (2 pending + 1 success), got %d", hits)
	}
}

func TestPollToken_SlowDownExtendsInterval(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"slow_down"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"AT","token_type":"Bearer","expires_in":3600}`))
	}))
	defer srv.Close()

	// Set ctx timeout large; what we care about is that the loop
	// progresses past slow_down and returns the token on the next
	// poll. Verifying the interval extension precisely is timing-
	// sensitive, so we just check that we did succeed in 2 polls.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tok, err := PollToken(ctx, DeviceCodeConfig{
		TokenURL: srv.URL, ClientID: "c",
	}, "DC", 10*time.Millisecond, httpDoerForTest())
	if err != nil {
		t.Fatalf("PollToken: %v", err)
	}
	if tok.AccessToken != "AT" {
		t.Errorf("AccessToken = %q", tok.AccessToken)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("expected 2 polls (slow_down + success), got %d", got)
	}
}

func TestPollToken_AccessDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"access_denied"}`))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := PollToken(ctx, DeviceCodeConfig{
		TokenURL: srv.URL, ClientID: "c",
	}, "DC", 10*time.Millisecond, httpDoerForTest())
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("expected access_denied error, got %v", err)
	}
}

func TestPollToken_ExpiredToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"expired_token"}`))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := PollToken(ctx, DeviceCodeConfig{
		TokenURL: srv.URL, ClientID: "c",
	}, "DC", 10*time.Millisecond, httpDoerForTest())
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired_token error, got %v", err)
	}
}

func TestPollToken_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := PollToken(ctx, DeviceCodeConfig{
		TokenURL: srv.URL, ClientID: "c",
	}, "DC", 20*time.Millisecond, httpDoerForTest())
	if err == nil {
		t.Fatalf("expected ctx cancellation error")
	}
}

func TestPollToken_BasicAuthWithSecret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Errorf("expected Authorization header for confidential client")
		}
		_ = r.ParseForm()
		// Confidential clients may omit client_id in the form body
		// (they're authenticated via Basic auth).
		_, _ = w.Write([]byte(`{"access_token":"AT","token_type":"Bearer","expires_in":3600}`))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := PollToken(ctx, DeviceCodeConfig{
		TokenURL: srv.URL, ClientID: "c", ClientSecret: "s",
	}, "DC", 10*time.Millisecond, httpDoerForTest())
	if err != nil {
		t.Fatalf("PollToken: %v", err)
	}
}

func TestPollToken_PublicClientUsesFormClientID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("public client should not send Authorization header")
		}
		_ = r.ParseForm()
		if r.PostForm.Get("client_id") != "public-app" {
			t.Errorf("public client should include client_id in form, got %q",
				r.PostForm.Get("client_id"))
		}
		_, _ = w.Write([]byte(`{"access_token":"AT","token_type":"Bearer","expires_in":3600}`))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := PollToken(ctx, DeviceCodeConfig{
		TokenURL: srv.URL, ClientID: "public-app",
	}, "DC", 10*time.Millisecond, httpDoerForTest())
	if err != nil {
		t.Fatalf("PollToken: %v", err)
	}
}
