package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeviceCodeConfig parameterises the Device Authorization Grant
// (RFC 8628). It targets two endpoints — the device authorization
// endpoint (DeviceURL) for the initial request, and the standard
// token endpoint (TokenURL) for polling.
type DeviceCodeConfig struct {
	DeviceURL    string
	TokenURL     string
	ClientID     string
	ClientSecret string // optional — public clients leave empty
	Scope        string
	ExtraParams  map[string]string
}

// DeviceAuthorization is the parsed result of the device
// authorization endpoint response (RFC 8628 §3.2). Interval and
// ExpiresAt are derived locally so callers don't have to keep
// wall-clock state.
type DeviceAuthorization struct {
	DeviceCode              string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	ExpiresAt               time.Time
	Interval                time.Duration
}

// Authorize issues a device authorization request (RFC 8628 §3.1)
// and parses the IdP's response into a DeviceAuthorization. The
// returned UserCode + VerificationURI are what the caller must
// display so the user can complete the grant on a separate device.
func Authorize(ctx context.Context, cfg DeviceCodeConfig, doer Doer) (*DeviceAuthorization, error) {
	if cfg.DeviceURL == "" {
		return nil, fmt.Errorf("oauth: device_url is required")
	}
	if cfg.TokenURL == "" {
		return nil, fmt.Errorf("oauth: token_url is required")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("oauth: client_id is required")
	}

	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	if cfg.Scope != "" {
		form.Set("scope", cfg.Scope)
	}
	for k, v := range cfg.ExtraParams {
		form.Set(k, v)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.DeviceURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("oauth: build device request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	// Confidential clients authenticate the device authorization
	// request via Basic auth (RFC 8628 §3.1). Public clients send
	// only client_id in the form body.
	if cfg.ClientSecret != "" {
		creds := cfg.ClientID + ":" + cfg.ClientSecret
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(creds)))
	}

	if doer == nil {
		doer = DefaultDoer()
	}
	status, raw, err := doer(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: %w", err)
	}
	if status < 200 || status >= 300 {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if json.Unmarshal(raw, &errResp) == nil && errResp.Error != "" {
			msg := errResp.Error
			if errResp.ErrorDescription != "" {
				msg += ": " + errResp.ErrorDescription
			}
			return nil, fmt.Errorf("oauth: device authorization returned %d: %s", status, msg)
		}
		return nil, fmt.Errorf("oauth: device authorization returned %d", status)
	}

	var parsed struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		ExpiresIn               int    `json:"expires_in"`
		Interval                int    `json:"interval"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("oauth: parse device response: %w", err)
	}
	if parsed.DeviceCode == "" {
		return nil, fmt.Errorf("oauth: device response missing device_code")
	}
	if parsed.UserCode == "" || parsed.VerificationURI == "" {
		return nil, fmt.Errorf("oauth: device response missing user_code or verification_uri")
	}
	interval := parsed.Interval
	if interval < 1 {
		// RFC 8628 §3.2: interval is optional, default to 5 seconds.
		interval = 5
	}
	expiresIn := parsed.ExpiresIn
	if expiresIn < 1 {
		// Fallback when the server didn't specify; 10 minutes is a
		// common-enough default that gives the user time to complete
		// the verification step.
		expiresIn = 600
	}
	return &DeviceAuthorization{
		DeviceCode:              parsed.DeviceCode,
		UserCode:                parsed.UserCode,
		VerificationURI:         parsed.VerificationURI,
		VerificationURIComplete: parsed.VerificationURIComplete,
		ExpiresAt:               time.Now().Add(time.Duration(expiresIn) * time.Second),
		Interval:                time.Duration(interval) * time.Second,
	}, nil
}

// PollToken polls the token endpoint until the user authorizes or
// denies, the device_code expires, or ctx is cancelled (RFC 8628
// §3.4–3.5). The initial interval comes from Authorize; on
// slow_down responses the interval is increased by 5 seconds per
// RFC 8628 §3.5. authorization_pending continues polling at the
// current interval. The success path returns a Token with whatever
// fields the IdP supplied (RefreshToken may be empty per the IdP's
// rotation policy; the v1.6.5 Refresh preserves the old value
// when callers later refresh).
func PollToken(ctx context.Context, cfg DeviceCodeConfig, deviceCode string, interval time.Duration, doer Doer) (*Token, error) {
	if cfg.TokenURL == "" {
		return nil, fmt.Errorf("oauth: token_url is required")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("oauth: client_id is required")
	}
	if deviceCode == "" {
		return nil, fmt.Errorf("oauth: device_code is required")
	}
	if interval <= 0 {
		// Server didn't provide an interval (or caller passed
		// zero/negative). Default to the RFC 8628 §3.2 baseline.
		interval = 5 * time.Second
	}
	if doer == nil {
		doer = DefaultDoer()
	}

	for {
		// Wait the polling interval first. RFC 8628 §3.4 doesn't
		// mandate an initial delay, but waiting at least once gives
		// the user time to start the device-side flow before pollen
		// fires its first poll.
		// NewTimer + Stop so a cancel mid-wait doesn't leave the timer alive
		// until it fires (time.After can't be stopped); mirrors the intruder
		// runner's delay loop.
		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("oauth: device poll: %w", ctx.Err())
		}

		form := url.Values{}
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		form.Set("device_code", deviceCode)
		// Per RFC 8628 §3.4: include client_id when the client is a
		// public client (no secret). Confidential clients
		// authenticate via Basic auth and may omit client_id.
		if cfg.ClientSecret == "" {
			form.Set("client_id", cfg.ClientID)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", cfg.TokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, fmt.Errorf("oauth: build token request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		if cfg.ClientSecret != "" {
			creds := cfg.ClientID + ":" + cfg.ClientSecret
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(creds)))
		}

		status, raw, err := doer(req)
		if err != nil {
			return nil, fmt.Errorf("oauth: %w", err)
		}

		if status >= 200 && status < 300 {
			var parsed struct {
				AccessToken  string `json:"access_token"`
				TokenType    string `json:"token_type"`
				RefreshToken string `json:"refresh_token"`
				ExpiresIn    int    `json:"expires_in"`
				Scope        string `json:"scope"`
			}
			if err := json.Unmarshal(raw, &parsed); err != nil {
				return nil, fmt.Errorf("oauth: parse token response: %w", err)
			}
			if parsed.AccessToken == "" {
				return nil, fmt.Errorf("oauth: token response missing access_token")
			}
			t := &Token{
				AccessToken:  parsed.AccessToken,
				TokenType:    parsed.TokenType,
				RefreshToken: parsed.RefreshToken,
				Scope:        parsed.Scope,
			}
			if parsed.ExpiresIn > 0 {
				t.ExpiresAt = time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second)
			}
			return t, nil
		}

		// Non-2xx — parse the OAuth error response and act on it
		// per RFC 8628 §3.5. authorization_pending and slow_down
		// continue polling; access_denied / expired_token / other
		// errors are terminal.
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = json.Unmarshal(raw, &errResp)
		switch errResp.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		case "access_denied":
			return nil, fmt.Errorf("oauth: user denied authorization")
		case "expired_token":
			return nil, fmt.Errorf("oauth: device_code expired")
		default:
			msg := errResp.Error
			if errResp.ErrorDescription != "" {
				if msg != "" {
					msg += ": "
				}
				msg += errResp.ErrorDescription
			}
			if msg == "" {
				return nil, fmt.Errorf("oauth: token endpoint returned %d", status)
			}
			return nil, fmt.Errorf("oauth: token endpoint returned %d: %s", status, msg)
		}
	}
}
