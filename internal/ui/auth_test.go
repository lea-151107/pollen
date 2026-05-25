package ui

import "testing"

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
