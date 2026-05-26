package httpx

import (
	"encoding/base64"
	"strings"
)

// AuthScheme identifies the kind of Authorization header to build. Defined
// here, in the HTTP layer, because the encoding (Bearer prefix, Base64 of
// "user:pass") is HTTP-protocol knowledge.
type AuthScheme int

const (
	AuthNone AuthScheme = iota
	AuthBearer
	AuthBasic
)

// BuildAuthHeader returns the Authorization header value for the given
// scheme and raw inputs, or "" when no header should be set.
//
//   - Bearer: returns "Bearer <token>" with leading/trailing whitespace
//     trimmed from token. Empty token → "".
//   - Basic: returns "Basic <base64(user:pass)>" with whitespace trimmed
//     from user and pass. Both fields empty after trimming → "".
//   - None: always "".
func BuildAuthHeader(scheme AuthScheme, token, user, pass string) string {
	switch scheme {
	case AuthBearer:
		tok := strings.TrimSpace(token)
		if tok == "" {
			return ""
		}
		return "Bearer " + tok
	case AuthBasic:
		user = strings.TrimSpace(user)
		pass = strings.TrimSpace(pass)
		if user == "" && pass == "" {
			return ""
		}
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
	}
	return ""
}
