package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const RefreshURL = "https://worksection.com/oauth2/refresh"

const defaultOAuthTimeout = 30 * time.Second

// HTTPClientWithTimeout returns the timeout-bearing client used for OAuth
// token exchange and refresh when tests do not inject a custom client.
func HTTPClientWithTimeout(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = defaultOAuthTimeout
	}
	return &http.Client{Timeout: timeout}
}

// NeedsRefresh reports whether an OAuth token should be refreshed before use.
func NeedsRefresh(expires time.Time, now time.Time) bool {
	if expires.IsZero() {
		return false
	}
	return now.Add(5 * time.Minute).After(expires)
}

// Refresh exchanges a stored refresh token for new Worksection OAuth tokens.
func Refresh(ctx context.Context, client *http.Client, b SecretBundle) (SecretBundle, error) {
	if client == nil {
		client = HTTPClientWithTimeout(defaultOAuthTimeout)
	}
	if b.ClientID == "" || b.ClientSecret == "" || b.RefreshToken == "" {
		return b, fmt.Errorf("client_id, client_secret, and refresh_token are required")
	}
	form := url.Values{}
	form.Set("client_id", b.ClientID)
	form.Set("client_secret", b.ClientSecret)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", b.RefreshToken)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, RefreshURL, strings.NewReader(form.Encode()))
		if err != nil {
			return b, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < 2 {
				if sleepErr := sleepContext(ctx, time.Duration(attempt+1)*time.Second); sleepErr != nil {
					return b, sleepErr
				}
				continue
			}
			return b, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = oauthHTTPError("oauth refresh failed", resp)
			_ = resp.Body.Close()
			if !retryOAuthStatus(resp.StatusCode) || attempt == 2 {
				return b, lastErr
			}
			delay := time.Duration(attempt+1) * time.Second
			if resp.StatusCode == http.StatusTooManyRequests {
				if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
					if parsedDelay, err := parseRetryAfter(retryAfter, time.Now()); err == nil {
						delay = parsedDelay
					}
				}
			}
			if err := sleepContext(ctx, delay); err != nil {
				return b, err
			}
			continue
		}
		defer func() { _ = resp.Body.Close() }()
		var body struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
			AccountURL   string `json:"account_url"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return b, err
		}
		if body.AccessToken == "" || body.RefreshToken == "" {
			return b, fmt.Errorf("oauth refresh response did not include tokens")
		}
		b.AccessToken = body.AccessToken
		b.RefreshToken = body.RefreshToken
		if body.AccountURL != "" {
			b.AccountURL = body.AccountURL
		}
		if body.ExpiresIn > 0 {
			b.AccessExpires = time.Now().Add(time.Duration(body.ExpiresIn) * time.Second)
		}
		return b, nil
	}
	return b, lastErr
}

// ExchangeCode exchanges a Worksection OAuth authorization code for access and
// refresh tokens.
func ExchangeCode(ctx context.Context, client *http.Client, clientID, clientSecret, code, redirectURI string) (SecretBundle, error) {
	if client == nil {
		client = HTTPClientWithTimeout(defaultOAuthTimeout)
	}
	if clientID == "" || clientSecret == "" || code == "" || redirectURI == "" {
		return SecretBundle{}, fmt.Errorf("client_id, client_secret, code, and redirect_uri are required")
	}
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return SecretBundle{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return SecretBundle{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SecretBundle{}, oauthHTTPError("oauth token exchange failed", resp)
	}
	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		AccountURL   string `json:"account_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return SecretBundle{}, err
	}
	if body.AccessToken == "" || body.RefreshToken == "" {
		return SecretBundle{}, fmt.Errorf("oauth token response did not include tokens")
	}
	b := SecretBundle{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AccessToken:  body.AccessToken,
		RefreshToken: body.RefreshToken,
		AccountURL:   body.AccountURL,
	}
	if body.ExpiresIn > 0 {
		b.AccessExpires = time.Now().Add(time.Duration(body.ExpiresIn) * time.Second)
	}
	return b, nil
}

func oauthHTTPError(prefix string, resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	var body struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		Message          string `json:"message"`
	}
	if err := json.Unmarshal(raw, &body); err == nil {
		message := firstOAuthMessage(body.ErrorDescription, body.Message, body.Error)
		if message != "" {
			return fmt.Errorf("%s with HTTP %d: %s", prefix, resp.StatusCode, redactOAuthMessage(message))
		}
	}
	return fmt.Errorf("%s with HTTP %d", prefix, resp.StatusCode)
}

func retryOAuthStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func parseRetryAfter(value string, now time.Time) (time.Duration, error) {
	if seconds, err := time.ParseDuration(value + "s"); err == nil {
		return seconds, nil
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0, err
	}
	d := when.Sub(now)
	if d < 0 {
		return 0, nil
	}
	return d, nil
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func firstOAuthMessage(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func redactOAuthMessage(message string) string {
	for _, marker := range []string{"access_token", "refresh_token", "client_secret"} {
		if strings.Contains(strings.ToLower(message), marker) {
			return "[REDACTED]"
		}
	}
	return message
}
