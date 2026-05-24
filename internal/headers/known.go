package headers

import "strings"

var Known = []string{
	"Accept",
	"Accept-Charset",
	"Accept-Encoding",
	"Accept-Language",
	"Accept-Ranges",
	"Access-Control-Allow-Credentials",
	"Access-Control-Allow-Headers",
	"Access-Control-Allow-Methods",
	"Access-Control-Allow-Origin",
	"Access-Control-Expose-Headers",
	"Access-Control-Max-Age",
	"Access-Control-Request-Headers",
	"Access-Control-Request-Method",
	"Age",
	"Allow",
	"Authorization",
	"Cache-Control",
	"Connection",
	"Content-Disposition",
	"Content-Encoding",
	"Content-Language",
	"Content-Length",
	"Content-Location",
	"Content-Range",
	"Content-Security-Policy",
	"Content-Type",
	"Cookie",
	"DNT",
	"Date",
	"ETag",
	"Expect",
	"Expires",
	"Forwarded",
	"From",
	"Host",
	"If-Match",
	"If-Modified-Since",
	"If-None-Match",
	"If-Range",
	"If-Unmodified-Since",
	"Keep-Alive",
	"Last-Modified",
	"Link",
	"Location",
	"Origin",
	"Pragma",
	"Range",
	"Referer",
	"Retry-After",
	"Server",
	"Set-Cookie",
	"Strict-Transport-Security",
	"TE",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
	"User-Agent",
	"Vary",
	"Via",
	"WWW-Authenticate",
	"Warning",
	"X-Api-Key",
	"X-Csrf-Token",
	"X-Forwarded-For",
	"X-Forwarded-Host",
	"X-Forwarded-Proto",
	"X-Frame-Options",
	"X-Real-Ip",
	"X-Request-Id",
	"X-Requested-With",
}

// Suggest returns header names matching the given prefix (case-insensitive).
// An empty prefix returns nil.
func Suggest(prefix string) []string {
	if prefix == "" {
		return nil
	}
	p := strings.ToLower(prefix)
	var out []string
	for _, h := range Known {
		if strings.HasPrefix(strings.ToLower(h), p) {
			out = append(out, h)
		}
	}
	return out
}
