package auth

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEnvStore(t *testing.T) {
	t.Setenv("WSECTL_ACCESS_TOKEN", "access")
	t.Setenv("WSECTL_ACCOUNT_URL", "https://company.worksection.com")
	b, err := EnvStore{}.Get(context.Background(), SecretRef{Scheme: "env"})
	if err != nil {
		t.Fatal(err)
	}
	if b.AccessToken != "access" || b.AccountURL == "" {
		t.Fatalf("unexpected bundle %#v", b)
	}
}

func TestAuthorizationURLIncludesState(t *testing.T) {
	u := AuthorizationURL("client", "https://localhost:33443/callback", "state", nil)
	for _, want := range []string{"client_id=client", "state=state", "redirect_uri="} {
		if !strings.Contains(u, want) {
			t.Fatalf("%q missing %q", u, want)
		}
	}
}

func TestAuthorizationURLUsesSingleSpaceSeparatedScope(t *testing.T) {
	u := AuthorizationURL("client", "https://localhost:33443/callback", "state", []string{
		"projects_read",
		"tasks_read",
		"contacts_read",
	})
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	values := parsed.Query()
	scopes := values["scope"]
	if len(scopes) != 1 {
		t.Fatalf("scope parameters = %#v, want exactly one", scopes)
	}
	want := "projects_read tasks_read contacts_read"
	if scopes[0] != want {
		t.Fatalf("scope = %q, want %q", scopes[0], want)
	}
}

func TestNeedsRefresh(t *testing.T) {
	now := time.Now()
	if !NeedsRefresh(now.Add(time.Minute), now) {
		t.Fatal("expected refresh near expiry")
	}
	if NeedsRefresh(now.Add(time.Hour), now) {
		t.Fatal("did not expect refresh")
	}
}

func TestHTTPClientWithTimeout(t *testing.T) {
	if got := HTTPClientWithTimeout(0).Timeout; got != defaultOAuthTimeout {
		t.Fatalf("default timeout = %s, want %s", got, defaultOAuthTimeout)
	}
	if got := HTTPClientWithTimeout(5 * time.Second).Timeout; got != 5*time.Second {
		t.Fatalf("configured timeout = %s", got)
	}
}

func TestKeyringConfigRestrictsBackends(t *testing.T) {
	cfg := keyringConfig()
	got := map[string]bool{}
	for _, backend := range cfg.AllowedBackends {
		got[string(backend)] = true
	}
	for _, want := range []string{"keychain", "wincred", "secret-service", "kwallet", "pass"} {
		if !got[want] {
			t.Fatalf("allowed backends missing %s: %#v", want, cfg.AllowedBackends)
		}
	}
	for _, forbidden := range []string{"file", "keyctl"} {
		if got[forbidden] {
			t.Fatalf("forbidden backend %s is allowed: %#v", forbidden, cfg.AllowedBackends)
		}
	}
}

func TestRefresh(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("grant_type") != "refresh_token" {
			t.Fatalf("grant_type = %q", r.Form.Get("grant_type"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":86400,"account_url":"https://company.worksection.com"}`)),
		}, nil
	})}
	got, err := Refresh(context.Background(), client, SecretBundle{ClientID: "id", ClientSecret: "secret", RefreshToken: "old"})
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "new-access" || got.RefreshToken != "new-refresh" || got.AccountURL == "" {
		t.Fatalf("unexpected bundle %#v", got)
	}
}

func TestRefreshRetriesTransientFailures(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{"Retry-After": []string{"0"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":"rate_limited"}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":86400}`)),
		}, nil
	})}
	got, err := Refresh(context.Background(), client, SecretBundle{ClientID: "id", ClientSecret: "secret", RefreshToken: "old"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || got.AccessToken != "new-access" || got.RefreshToken != "new-refresh" {
		t.Fatalf("unexpected retry result calls=%d bundle=%#v", calls, got)
	}
}

func TestRefreshDoesNotRetryTerminalOAuthFailure(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":"invalid_grant"}`)),
		}, nil
	})}
	_, err := Refresh(context.Background(), client, SecretBundle{ClientID: "id", ClientSecret: "secret", RefreshToken: "old"})
	if err == nil {
		t.Fatal("expected refresh error")
	}
	if calls != 1 {
		t.Fatalf("terminal OAuth failure was retried %d times", calls)
	}
}

func TestEncryptedFileStoreWritesVersionedArgon2Payload(t *testing.T) {
	t.Setenv("WSECTL_SECRET_PASSPHRASE", "passphrase")
	ref := SecretRef{Scheme: "encrypted-file", Name: t.TempDir() + "/secret.json"}
	store := EncryptedFileStore{}
	value := SecretBundle{AccessToken: "access", RefreshToken: "refresh"}
	if err := store.Set(context.Background(), ref, value); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(ref.Name)
	if err != nil {
		t.Fatal(err)
	}
	var payload encryptedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Version != 2 || payload.KDF != "argon2id" || payload.Salt == "" {
		t.Fatalf("unexpected encrypted payload metadata %#v", payload)
	}
	got, err := store.Get(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != value.AccessToken || got.RefreshToken != value.RefreshToken {
		t.Fatalf("unexpected decrypted value %#v", got)
	}
}

func TestEncryptedFileStoreCheckWritableRequiresPassphrase(t *testing.T) {
	err := EncryptedFileStore{}.CheckWritable(context.Background(), SecretRef{Name: t.TempDir() + "/secret.json"})
	if err == nil || !strings.Contains(err.Error(), "WSECTL_SECRET_PASSPHRASE") {
		t.Fatalf("expected passphrase error, got %v", err)
	}
}

func TestPlaintextStoreCheckWritable(t *testing.T) {
	ref := SecretRef{Name: t.TempDir() + "/secret.json"}
	if err := (PlaintextStore{}).CheckWritable(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
}

func TestEnvStoreCheckWritableFails(t *testing.T) {
	err := EnvStore{}.CheckWritable(context.Background(), SecretRef{Scheme: "env"})
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %v", err)
	}
}

func TestExchangeCode(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "code" {
			t.Fatalf("unexpected form %#v", r.Form)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"access","refresh_token":"refresh","expires_in":86400,"account_url":"https://company.worksection.com"}`)),
		}, nil
	})}
	got, err := ExchangeCode(context.Background(), client, "id", "secret", "code", "https://localhost:33443/callback")
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "access" || got.RefreshToken != "refresh" || got.ClientID != "id" {
		t.Fatalf("unexpected bundle %#v", got)
	}
}

func TestOAuthCallbackRejectsUnsafeBinding(t *testing.T) {
	if _, err := StartOAuthCallback(CallbackOptions{Host: "0.0.0.0", Port: 33443}, "state"); err == nil {
		t.Fatal("expected non-loopback host to fail")
	}
	if _, err := StartOAuthCallback(CallbackOptions{Host: "localhost", Port: 0}, "state"); err == nil {
		t.Fatal("expected port 0 to fail")
	}
}

func TestOAuthCallbackKeepsListeningAfterInvalidRequests(t *testing.T) {
	port := freePort(t)
	callback, err := StartOAuthCallback(CallbackOptions{Host: "localhost", Port: port}, "good")
	if err != nil {
		t.Fatal(err)
	}
	defer callback.Close()

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	if resp, err := client.Get(callback.RedirectURI + "?state=bad&code=wrong"); err != nil {
		t.Fatal(err)
	} else {
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("invalid state status = %d", resp.StatusCode)
		}
	}
	if resp, err := client.Post(callback.RedirectURI+"?state=good&code=wrong", "text/plain", strings.NewReader("")); err != nil {
		t.Fatal(err)
	} else {
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("post status = %d", resp.StatusCode)
		}
	}
	if resp, err := client.Get(callback.RedirectURI + "?state=good&code=right"); err != nil {
		t.Fatal(err)
	} else {
		_ = resp.Body.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	code, err := callback.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if code != "right" {
		t.Fatalf("code = %q, want right", code)
	}
}

func TestOAuthCallbackReportsOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := net.LookupPort("tcp", port)
	if err != nil {
		t.Fatal(err)
	}
	_, err = StartOAuthCallback(CallbackOptions{Host: "127.0.0.1", Port: parsed}, "state")
	if err == nil || !strings.Contains(err.Error(), "choose a free port") {
		t.Fatalf("expected occupied port guidance, got %v", err)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := net.LookupPort("tcp", port)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
