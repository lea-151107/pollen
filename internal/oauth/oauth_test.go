package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func httpDoerForTest() Doer {
	c := &http.Client{Timeout: 5 * time.Second}
	return func(req *http.Request) (int, []byte, error) {
		resp, err := c.Do(req)
		if err != nil {
			return 0, nil, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b, nil
	}
}

func TestClientCredentials_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type: %s", got)
		}
		_ = r.ParseForm()
		if got := r.PostForm.Get("grant_type"); got != "client_credentials" {
			t.Errorf("grant_type: %s", got)
		}
		if got := r.PostForm.Get("scope"); got != "read write" {
			t.Errorf("scope: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"Bearer","expires_in":3600,"refresh_token":"rt"}`))
	}))
	defer srv.Close()
	tok, err := ClientCredentials(context.Background(), ClientCredentialsConfig{
		TokenURL:     srv.URL,
		ClientID:     "id",
		ClientSecret: "secret",
		Scope:        "read write",
	}, httpDoerForTest())
	if err != nil {
		t.Fatalf("ClientCredentials: %v", err)
	}
	if tok.AccessToken != "tok" || tok.TokenType != "Bearer" || tok.RefreshToken != "rt" {
		t.Errorf("token: %+v", tok)
	}
	if tok.ExpiresAt.Before(time.Now().Add(50 * time.Minute)) {
		t.Errorf("ExpiresAt too early: %v", tok.ExpiresAt)
	}
}

func TestClientCredentials_BasicAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("missing Basic auth: %q", auth)
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
		if err != nil {
			t.Fatalf("base64 decode: %v", err)
		}
		if string(decoded) != "myid:mysecret" {
			t.Errorf("creds: %q", decoded)
		}
		_, _ = w.Write([]byte(`{"access_token":"t"}`))
	}))
	defer srv.Close()
	_, err := ClientCredentials(context.Background(), ClientCredentialsConfig{
		TokenURL:     srv.URL,
		ClientID:     "myid",
		ClientSecret: "mysecret",
	}, httpDoerForTest())
	if err != nil {
		t.Fatalf("ClientCredentials: %v", err)
	}
}

func TestClientCredentials_ErrorResponseSurfacesDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"unknown client"}`))
	}))
	defer srv.Close()
	_, err := ClientCredentials(context.Background(), ClientCredentialsConfig{
		TokenURL: srv.URL, ClientID: "x", ClientSecret: "y",
	}, httpDoerForTest())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid_client") || !strings.Contains(err.Error(), "unknown client") {
		t.Errorf("error should surface description: %v", err)
	}
}

func TestClientCredentials_MissingFieldsValidate(t *testing.T) {
	if _, err := ClientCredentials(context.Background(), ClientCredentialsConfig{ClientID: "x"}, nil); err == nil {
		t.Error("expected error when TokenURL is empty")
	}
	if _, err := ClientCredentials(context.Background(), ClientCredentialsConfig{TokenURL: "http://x"}, nil); err == nil {
		t.Error("expected error when ClientID is empty")
	}
}

func TestRefresh_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if got := r.PostForm.Get("grant_type"); got != "refresh_token" {
			t.Errorf("grant_type: %s", got)
		}
		if got := r.PostForm.Get("refresh_token"); got != "old-rt" {
			t.Errorf("refresh_token: %s", got)
		}
		_, _ = w.Write([]byte(`{"access_token":"new","refresh_token":"new-rt","expires_in":60}`))
	}))
	defer srv.Close()
	tok, err := Refresh(context.Background(), RefreshConfig{
		TokenURL: srv.URL, RefreshToken: "old-rt",
	}, httpDoerForTest())
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tok.AccessToken != "new" || tok.RefreshToken != "new-rt" {
		t.Errorf("token: %+v", tok)
	}
}

func TestRefresh_MissingFieldsValidate(t *testing.T) {
	if _, err := Refresh(context.Background(), RefreshConfig{}, nil); err == nil {
		t.Error("expected error for empty config")
	}
}

func TestToken_IsExpired(t *testing.T) {
	none := &Token{}
	if none.IsExpired(0) {
		t.Error("zero ExpiresAt should report not-expired")
	}
	future := &Token{ExpiresAt: time.Now().Add(time.Hour)}
	if future.IsExpired(0) {
		t.Error("future ExpiresAt should not be expired")
	}
	past := &Token{ExpiresAt: time.Now().Add(-time.Hour)}
	if !past.IsExpired(0) {
		t.Error("past ExpiresAt should be expired")
	}
	near := &Token{ExpiresAt: time.Now().Add(30 * time.Second)}
	if !near.IsExpired(time.Minute) {
		t.Error("ExpiresAt 30s away should be expired with 60s skew")
	}
}

func TestClientCredentials_MissingAccessToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Server returns 200 but no access_token.
		_, _ = w.Write([]byte(`{"token_type":"Bearer"}`))
	}))
	defer srv.Close()
	_, err := ClientCredentials(context.Background(), ClientCredentialsConfig{
		TokenURL: srv.URL, ClientID: "x", ClientSecret: "y",
	}, httpDoerForTest())
	if err == nil || !strings.Contains(err.Error(), "missing access_token") {
		t.Errorf("expected missing-token error, got %v", err)
	}
}

// Compile-time assertion that DefaultDoer satisfies the Doer
// interface (it's the easiest way to spot a regression if we ever
// change the signature).
var _ Doer = DefaultDoer()

// Silence unused warning if other tests don't reference json.
var _ = json.Marshal
