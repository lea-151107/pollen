package oauth

import (
	"time"

	"github.com/lea-151107/pollen/internal/userconfig"
)

const TokenStoreFile = "oauth_tokens.json"

// Grant constants for StoredToken.Grant.
const (
	GrantClientCredentials = "client_credentials"
	GrantAuthorizationCode = "authorization_code"
)

// StoredToken is one entry in the on-disk token store. It carries
// both the token material and enough config context (TokenURL,
// ClientID, Grant) to identify which Auth panel configuration the
// token belongs to.
type StoredToken struct {
	TokenURL     string    `json:"token_url"`
	ClientID     string    `json:"client_id"`
	Scope        string    `json:"scope,omitempty"`
	Grant        string    `json:"grant"`
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TokenStore is the JSON-serialised form of the persisted token set.
// Keyed by (TokenURL, ClientID, Grant) — Scope is stored but not
// part of the lookup key, so re-fetching with a different scope
// overwrites the existing entry by design.
type TokenStore struct {
	Tokens []StoredToken `json:"tokens"`
}

// LoadTokenStore reads the on-disk token store. A missing file or a
// corrupt one both yield an empty (non-nil) store with no error —
// the goal is to never block startup on a bad persistence file.
func LoadTokenStore() (*TokenStore, error) {
	s := &TokenStore{}
	if _, err := userconfig.LoadJSON(TokenStoreFile, s); err != nil {
		return &TokenStore{}, nil
	}
	return s, nil
}

// Save writes the store back to disk with 0600 mode.
func (s *TokenStore) Save() error {
	return userconfig.SaveJSONSecure(TokenStoreFile, s)
}

// Find returns the entry matching (tokenURL, clientID, grant) and
// true, or (nil, false) when no entry matches. Empty strings in any
// of the key fields never match (callers shouldn't try to hydrate
// with blank panels).
func (s *TokenStore) Find(tokenURL, clientID, grant string) (*StoredToken, bool) {
	if tokenURL == "" || clientID == "" || grant == "" {
		return nil, false
	}
	for i := range s.Tokens {
		t := &s.Tokens[i]
		if t.TokenURL == tokenURL && t.ClientID == clientID && t.Grant == grant {
			return t, true
		}
	}
	return nil, false
}

// Put inserts or replaces a token for the (tokenURL, clientID,
// grant) key. The Scope from the call is recorded (potentially
// overwriting a prior scope on the same key — the latest fetch
// wins). UpdatedAt is set to the wall-clock time of the call.
func (s *TokenStore) Put(tokenURL, clientID, scope, grant string, tok *Token) {
	if tok == nil || tokenURL == "" || clientID == "" || grant == "" {
		return
	}
	entry := StoredToken{
		TokenURL:     tokenURL,
		ClientID:     clientID,
		Scope:        scope,
		Grant:        grant,
		AccessToken:  tok.AccessToken,
		TokenType:    tok.TokenType,
		RefreshToken: tok.RefreshToken,
		ExpiresAt:    tok.ExpiresAt,
		UpdatedAt:    time.Now().UTC(),
	}
	for i := range s.Tokens {
		t := &s.Tokens[i]
		if t.TokenURL == tokenURL && t.ClientID == clientID && t.Grant == grant {
			s.Tokens[i] = entry
			return
		}
	}
	s.Tokens = append(s.Tokens, entry)
}

// Forget removes the entry matching (tokenURL, clientID, grant)
// and returns true; returns false if no entry matched.
func (s *TokenStore) Forget(tokenURL, clientID, grant string) bool {
	for i := range s.Tokens {
		t := &s.Tokens[i]
		if t.TokenURL == tokenURL && t.ClientID == clientID && t.Grant == grant {
			s.Tokens = append(s.Tokens[:i], s.Tokens[i+1:]...)
			return true
		}
	}
	return false
}
