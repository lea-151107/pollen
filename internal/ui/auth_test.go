package ui

import "testing"

func TestAuth_HeaderValue_None(t *testing.T) {
	a := NewAuth()
	if got := a.HeaderValue(); got != "" {
		t.Errorf("None should yield empty, got %q", got)
	}
}

func TestAuth_HeaderValue_BearerEmpty(t *testing.T) {
	a := NewAuth()
	a.authType = AuthBearer
	if got := a.HeaderValue(); got != "" {
		t.Errorf("empty token should yield empty, got %q", got)
	}
}

func TestAuth_HeaderValue_Bearer(t *testing.T) {
	a := NewAuth()
	a.authType = AuthBearer
	a.token.SetValue("sk-abc123")
	want := "Bearer sk-abc123"
	if got := a.HeaderValue(); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestAuth_HeaderValue_BearerTrimmed(t *testing.T) {
	a := NewAuth()
	a.authType = AuthBearer
	a.token.SetValue("  tok  ")
	want := "Bearer tok"
	if got := a.HeaderValue(); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestAuth_HeaderValue_Basic(t *testing.T) {
	a := NewAuth()
	a.authType = AuthBasic
	a.user.SetValue("alice")
	a.pass.SetValue("secret")
	// base64("alice:secret") = "YWxpY2U6c2VjcmV0"
	want := "Basic YWxpY2U6c2VjcmV0"
	if got := a.HeaderValue(); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestAuth_HeaderValue_BasicEmpty(t *testing.T) {
	a := NewAuth()
	a.authType = AuthBasic
	if got := a.HeaderValue(); got != "" {
		t.Errorf("empty user+pass should yield empty, got %q", got)
	}
}

func TestAuth_HeaderValue_BasicUserOnly(t *testing.T) {
	a := NewAuth()
	a.authType = AuthBasic
	a.user.SetValue("alice")
	// base64("alice:") = "YWxpY2U6"
	want := "Basic YWxpY2U6"
	if got := a.HeaderValue(); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestAuth_Reset(t *testing.T) {
	a := NewAuth()
	a.authType = AuthBearer
	a.token.SetValue("tok")
	a.Reset()
	if a.authType != AuthNone {
		t.Errorf("type not reset")
	}
	if a.token.Value() != "" {
		t.Errorf("token not cleared")
	}
}
