package oauth

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/lea-151107/pollen/internal/userconfig"
)

func withTempUserconfig(t *testing.T) {
	t.Helper()
	userconfig.SetOverride(t.TempDir())
	t.Cleanup(func() { userconfig.SetOverride("") })
}

func sampleToken() *Token {
	return &Token{
		AccessToken:  "AT",
		TokenType:    "Bearer",
		RefreshToken: "RT",
		ExpiresAt:    time.Now().Add(1 * time.Hour).UTC(),
		Scope:        "read",
	}
}

func TestTokenStore_PutFindRoundTrip(t *testing.T) {
	withTempUserconfig(t)

	s := &TokenStore{}
	s.Put("https://idp.example.com/token", "myclient", "read", GrantClientCredentials, sampleToken())
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadTokenStore()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, ok := loaded.Find("https://idp.example.com/token", "myclient", GrantClientCredentials)
	if !ok {
		t.Fatalf("Find did not match after roundtrip")
	}
	if got.AccessToken != "AT" || got.RefreshToken != "RT" || got.Scope != "read" {
		t.Errorf("roundtrip lost data: %+v", got)
	}
}

func TestTokenStore_Put_ReplacesExistingByKey(t *testing.T) {
	s := &TokenStore{}
	s.Put("u", "c", "s1", GrantClientCredentials, &Token{AccessToken: "v1"})
	s.Put("u", "c", "s2", GrantClientCredentials, &Token{AccessToken: "v2"})
	if len(s.Tokens) != 1 {
		t.Fatalf("expected 1 entry after replacement, got %d", len(s.Tokens))
	}
	if s.Tokens[0].AccessToken != "v2" || s.Tokens[0].Scope != "s2" {
		t.Errorf("entry not replaced: %+v", s.Tokens[0])
	}
}

func TestTokenStore_Put_GrantDifferentiates(t *testing.T) {
	s := &TokenStore{}
	s.Put("u", "c", "", GrantClientCredentials, &Token{AccessToken: "CC"})
	s.Put("u", "c", "", GrantAuthorizationCode, &Token{AccessToken: "AC"})
	if len(s.Tokens) != 2 {
		t.Fatalf("CC and AC tokens for same URL/client should coexist, got %d", len(s.Tokens))
	}
}

func TestTokenStore_Forget_RemovesAndReturnsTrue(t *testing.T) {
	s := &TokenStore{}
	s.Put("u", "c", "", GrantClientCredentials, &Token{AccessToken: "v"})
	if !s.Forget("u", "c", GrantClientCredentials) {
		t.Errorf("Forget should return true when entry exists")
	}
	if len(s.Tokens) != 0 {
		t.Errorf("entry not removed after Forget")
	}
}

func TestTokenStore_Forget_AbsentReturnsFalse(t *testing.T) {
	s := &TokenStore{}
	if s.Forget("u", "c", GrantClientCredentials) {
		t.Errorf("Forget should return false when no entry")
	}
}

func TestTokenStore_Find_EmptyKeyNeverMatches(t *testing.T) {
	s := &TokenStore{}
	s.Put("u", "c", "", GrantClientCredentials, &Token{AccessToken: "v"})
	if _, ok := s.Find("", "c", GrantClientCredentials); ok {
		t.Errorf("empty TokenURL should never match")
	}
	if _, ok := s.Find("u", "", GrantClientCredentials); ok {
		t.Errorf("empty ClientID should never match")
	}
	if _, ok := s.Find("u", "c", ""); ok {
		t.Errorf("empty Grant should never match")
	}
}

func TestTokenStore_Save_Uses0600Mode(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Windows file permissions are ACL-based; Go's os.WriteFile
		// reports 0o666 regardless of the requested mode. The 0o600
		// guarantee from SaveJSONSecure is meaningful only on POSIX
		// systems, where this test continues to pin it.
		t.Skip("POSIX mode bits don't apply on Windows (ACL-based)")
	}
	withTempUserconfig(t)

	s := &TokenStore{}
	s.Put("u", "c", "", GrantClientCredentials, &Token{AccessToken: "secret"})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path, _ := userconfig.Path(TokenStoreFile)
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Errorf("token store file mode = %o, want 0600", perm)
	}
}

func TestTokenStore_Load_MissingFileReturnsEmpty(t *testing.T) {
	withTempUserconfig(t)

	s, err := LoadTokenStore()
	if err != nil {
		t.Fatalf("Load on missing file should not error, got %v", err)
	}
	if s == nil {
		t.Fatalf("Load should return non-nil empty store, not nil")
	}
	if len(s.Tokens) != 0 {
		t.Errorf("expected empty store, got %d tokens", len(s.Tokens))
	}
}

func TestTokenStore_Load_CorruptFileReturnsEmpty(t *testing.T) {
	withTempUserconfig(t)

	dir := t.TempDir()
	userconfig.SetOverride(dir)
	path := filepath.Join(dir, TokenStoreFile)
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	s, err := LoadTokenStore()
	if err != nil {
		t.Fatalf("Load on corrupt file should not error, got %v", err)
	}
	if len(s.Tokens) != 0 {
		t.Errorf("expected empty store on corrupt file, got %d tokens", len(s.Tokens))
	}
}

func TestTokenStore_Put_NilTokenIsNoop(t *testing.T) {
	s := &TokenStore{}
	s.Put("u", "c", "", GrantClientCredentials, nil)
	if len(s.Tokens) != 0 {
		t.Errorf("nil token should be ignored")
	}
}
