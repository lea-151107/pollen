package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// AuthCodeConfig parameterises the Authorization Code with PKCE flow
// (RFC 6749 §4.1, RFC 7636, RFC 8252). RedirectURI must be a loopback
// HTTP URI — pollen does not support custom-scheme redirects.
type AuthCodeConfig struct {
	AuthURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string // optional — public clients leave empty
	RedirectURI  string
	Scope        string
	ExtraParams  map[string]string
}

// AuthorizationCode runs the full Auth-Code-with-PKCE handshake:
// generates state + PKCE, starts a loopback callback server, opens
// the user's browser at the authorization URL, waits for the
// callback (or ctx cancellation / timeout), and exchanges the code
// for a token. openBrowser is injected so tests can substitute a
// callback-poking goroutine; production callers pass OpenBrowser.
func AuthorizationCode(ctx context.Context, cfg AuthCodeConfig, doer Doer, openBrowser func(string) error) (*Token, error) {
	if cfg.AuthURL == "" {
		return nil, fmt.Errorf("oauth: auth_url is required")
	}
	if cfg.TokenURL == "" {
		return nil, fmt.Errorf("oauth: token_url is required")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("oauth: client_id is required")
	}
	if cfg.RedirectURI == "" {
		return nil, fmt.Errorf("oauth: redirect_uri is required")
	}
	port, path, err := parseLoopback(cfg.RedirectURI)
	if err != nil {
		return nil, err
	}

	state, err := generateState()
	if err != nil {
		return nil, err
	}
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, err
	}

	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("oauth: bind callback port %d: %w", port, err)
	}

	// Build the authorization URL.
	authURL, err := buildAuthURL(cfg, state, challenge)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}

	code, err := awaitCallback(ctx, ln, path, state, openBrowser, authURL)
	if err != nil {
		return nil, err
	}

	// Exchange code for token.
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", cfg.RedirectURI)
	form.Set("code_verifier", verifier)
	if cfg.ClientSecret == "" {
		// Public client: client_id in form body (RFC 6749 §4.1.3, §2.3.1).
		form.Set("client_id", cfg.ClientID)
	}
	return postForm(ctx, cfg.TokenURL, cfg.ClientID, cfg.ClientSecret, form, doer)
}

func buildAuthURL(cfg AuthCodeConfig, state, challenge string) (string, error) {
	u, err := url.Parse(cfg.AuthURL)
	if err != nil {
		return "", fmt.Errorf("oauth: parse auth_url: %w", err)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.RedirectURI)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	if cfg.Scope != "" {
		q.Set("scope", cfg.Scope)
	}
	for k, v := range cfg.ExtraParams {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

type callbackResult struct {
	code string
	err  error
}

// awaitCallback owns the loopback server lifecycle: serves on ln,
// opens the browser, blocks until the callback path is hit (or ctx
// closes), then shuts the server down. Returns the authorization code
// on success.
func awaitCallback(ctx context.Context, ln net.Listener, path, wantState string, openBrowser func(string) error, authURL string) (string, error) {
	done := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if errStr := q.Get("error"); errStr != "" {
			desc := q.Get("error_description")
			msg := errStr
			if desc != "" {
				msg += ": " + desc
			}
			writeHTML(w, http.StatusOK, callbackFailHTML(msg))
			select {
			case done <- callbackResult{err: fmt.Errorf("oauth: authorization denied: %s", msg)}:
			default:
			}
			return
		}
		gotState := q.Get("state")
		if gotState != wantState {
			writeHTML(w, http.StatusBadRequest, callbackFailHTML("state mismatch (possible CSRF)"))
			select {
			case done <- callbackResult{err: errors.New("oauth: state mismatch (csrf check failed)")}:
			default:
			}
			return
		}
		code := q.Get("code")
		if code == "" {
			writeHTML(w, http.StatusBadRequest, callbackFailHTML("missing code parameter"))
			select {
			case done <- callbackResult{err: errors.New("oauth: callback missing code parameter")}:
			default:
			}
			return
		}
		writeHTML(w, http.StatusOK, callbackSuccessHTML)
		select {
		case done <- callbackResult{code: code}:
		default:
		}
	})

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	serveErr := make(chan error, 1)
	go func() {
		err := srv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	// Best-effort browser launch. If it fails, the URL is still
	// available to the caller via the status line (printed to stderr
	// here as a backstop — the TUI captures stderr-bound output
	// during the session, but the user can always copy from the
	// status line in the panel).
	if openBrowser != nil {
		_ = openBrowser(authURL)
	}

	var result callbackResult
	select {
	case result = <-done:
	case err := <-serveErr:
		return "", fmt.Errorf("oauth: callback server: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = srv.Shutdown(shutdownCtx)
		cancel()
		return "", fmt.Errorf("oauth: callback wait: %w", ctx.Err())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_ = srv.Shutdown(shutdownCtx)
	cancel()

	if result.err != nil {
		return "", result.err
	}
	return result.code, nil
}

func writeHTML(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

const callbackSuccessHTML = `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>pollen — authorized</title>
<style>body{font-family:system-ui,sans-serif;text-align:center;padding:3em;color:#1a1a1a;background:#fafafa}h1{color:#0a7}</style>
</head><body><h1>Authorization complete</h1><p>You may close this tab and return to pollen.</p></body></html>`

func callbackFailHTML(msg string) string {
	return `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>pollen — authorization failed</title>
<style>body{font-family:system-ui,sans-serif;text-align:center;padding:3em;color:#1a1a1a;background:#fafafa}h1{color:#c33}code{background:#eee;padding:.2em .4em;border-radius:3px}</style>
</head><body><h1>Authorization failed</h1><p><code>` + htmlEscape(msg) + `</code></p><p>You may close this tab and return to pollen.</p></body></html>`
}

func htmlEscape(s string) string {
	r := strings.NewReplacer(`&`, `&amp;`, `<`, `&lt;`, `>`, `&gt;`, `"`, `&quot;`, `'`, `&#39;`)
	return r.Replace(s)
}

// generatePKCE produces a code_verifier (43-char base64url of 32
// random bytes — within RFC 7636 §4.1's 43-128 range) and its S256
// code_challenge.
func generatePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("oauth: random: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

// generateState returns a 43-char base64url string suitable for the
// `state` parameter (CSRF protection).
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauth: random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// parseLoopback validates that uri is an http loopback URL and
// returns its port and path. Hosts other than 127.0.0.1 / ::1 /
// localhost are rejected; RFC 8252 §7.3 endorses loopback HTTP and
// pollen does not support custom-scheme redirects.
func parseLoopback(uri string) (port int, path string, err error) {
	u, err := url.Parse(uri)
	if err != nil {
		return 0, "", fmt.Errorf("oauth: parse redirect_uri: %w", err)
	}
	if u.Scheme != "http" {
		return 0, "", fmt.Errorf("oauth: redirect_uri scheme must be http for loopback, got %q", u.Scheme)
	}
	host := u.Hostname()
	if host != "127.0.0.1" && host != "::1" && host != "localhost" {
		return 0, "", fmt.Errorf("oauth: redirect_uri host must be loopback (127.0.0.1 / ::1 / localhost), got %q", host)
	}
	portStr := u.Port()
	if portStr == "" {
		return 0, "", fmt.Errorf("oauth: redirect_uri must include an explicit port")
	}
	port, err = strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return 0, "", fmt.Errorf("oauth: redirect_uri port invalid: %q", portStr)
	}
	path = u.Path
	if path == "" {
		path = "/"
	}
	return port, path, nil
}

// OpenBrowser launches the user's default browser at url. Best-
// effort: a non-nil return means the launch command failed to start,
// in which case the caller should surface the URL to the user for
// manual paste.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default: // linux/freebsd/etc
		// Try wslview first (handles WSL → host browser); fall back
		// to xdg-open for a normal desktop Linux.
		if path, err := exec.LookPath("wslview"); err == nil {
			cmd = exec.Command(path, url)
		} else {
			cmd = exec.Command("xdg-open", url)
		}
	}
	return cmd.Start()
}
