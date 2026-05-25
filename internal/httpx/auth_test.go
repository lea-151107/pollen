package httpx

import "testing"

func TestBuildAuthHeader_None(t *testing.T) {
	if got := BuildAuthHeader(AuthNone, "x", "u", "p"); got != "" {
		t.Errorf("None should yield empty, got %q", got)
	}
}

func TestBuildAuthHeader_Bearer(t *testing.T) {
	if got := BuildAuthHeader(AuthBearer, "sk-abc", "", ""); got != "Bearer sk-abc" {
		t.Errorf("got %q", got)
	}
}

func TestBuildAuthHeader_BearerEmpty(t *testing.T) {
	if got := BuildAuthHeader(AuthBearer, "  ", "", ""); got != "" {
		t.Errorf("blank/whitespace token should yield empty, got %q", got)
	}
}

func TestBuildAuthHeader_BearerTrimmed(t *testing.T) {
	if got := BuildAuthHeader(AuthBearer, "  tok  ", "", ""); got != "Bearer tok" {
		t.Errorf("got %q", got)
	}
}

func TestBuildAuthHeader_Basic(t *testing.T) {
	// base64("alice:secret") = "YWxpY2U6c2VjcmV0"
	if got := BuildAuthHeader(AuthBasic, "", "alice", "secret"); got != "Basic YWxpY2U6c2VjcmV0" {
		t.Errorf("got %q", got)
	}
}

func TestBuildAuthHeader_BasicEmpty(t *testing.T) {
	if got := BuildAuthHeader(AuthBasic, "", "", ""); got != "" {
		t.Errorf("empty creds should yield empty, got %q", got)
	}
}

func TestBuildAuthHeader_BasicUserOnly(t *testing.T) {
	// base64("alice:") = "YWxpY2U6"
	if got := BuildAuthHeader(AuthBasic, "", "alice", ""); got != "Basic YWxpY2U6" {
		t.Errorf("got %q", got)
	}
}
