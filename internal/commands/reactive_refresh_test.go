package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pbv7/wsectl/internal/auth"
	"github.com/pbv7/wsectl/internal/config"
	"github.com/spf13/cobra"
)

// failingStore is a writable-scheme store whose persistence is broken.
type failingStore struct{}

func (failingStore) Get(context.Context, auth.SecretRef) (auth.SecretBundle, error) {
	return auth.SecretBundle{}, nil
}
func (failingStore) Set(context.Context, auth.SecretRef, auth.SecretBundle) error {
	return errors.New("keychain unavailable")
}
func (failingStore) Delete(context.Context, auth.SecretRef) error        { return nil }
func (failingStore) CheckWritable(context.Context, auth.SecretRef) error { return nil }

func stubRefreshSecret(t *testing.T, refreshed auth.SecretBundle, err error) *int {
	t.Helper()
	calls := 0
	original := refreshSecretFunc
	refreshSecretFunc = func(context.Context, *http.Client, auth.SecretBundle) (auth.SecretBundle, error) {
		calls++
		return refreshed, err
	}
	t.Cleanup(func() { refreshSecretFunc = original })
	return &calls
}

func expiringOAuthSecret() auth.SecretBundle {
	return auth.SecretBundle{
		ClientID:      "client_123",
		ClientSecret:  "client_secret_123",
		AccessToken:   "stale-token",
		RefreshToken:  "refresh_123",
		AccessExpires: time.Now().Add(-time.Minute),
	}
}

// A4 (proactive path): when the refresh succeeds but the store cannot
// persist, the command must proceed with the refreshed token and warn —
// not fail with a valid token in hand.
func TestRefreshProfileSecretPersistFailureWarnsAndProceeds(t *testing.T) {
	refreshed := expiringOAuthSecret()
	refreshed.AccessToken = "fresh-token"
	calls := stubRefreshSecret(t, refreshed, nil)

	var warnings []string
	warn := func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	}
	secretInfo := profileSecret{
		secret: expiringOAuthSecret(),
		store:  failingStore{},
		ref:    auth.SecretRef{Scheme: "keyring", Name: "wsectl/default"},
	}
	got, err := refreshProfileSecret(context.Background(), config.Builtin(), config.Profile{AuthType: "oauth2"}, secretInfo, warn)
	if err != nil {
		t.Fatalf("persist failure must be non-fatal after a successful refresh, got %v", err)
	}
	if got.AccessToken != "fresh-token" {
		t.Fatalf("command must proceed with the refreshed token, got %#v", got)
	}
	if *calls != 1 {
		t.Fatalf("refresh calls = %d, want 1", *calls)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "could not be persisted") {
		t.Fatalf("want one persist warning, got %v", warnings)
	}
}

// ctxHonoringStore fails Set when the passed context is already cancelled,
// like any store backed by ctx-aware I/O would.
type ctxHonoringStore struct{ persisted *auth.SecretBundle }

func (ctxHonoringStore) Get(context.Context, auth.SecretRef) (auth.SecretBundle, error) {
	return auth.SecretBundle{}, nil
}
func (s ctxHonoringStore) Set(ctx context.Context, _ auth.SecretRef, value auth.SecretBundle) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	*s.persisted = value
	return nil
}
func (ctxHonoringStore) Delete(context.Context, auth.SecretRef) error        { return nil }
func (ctxHonoringStore) CheckWritable(context.Context, auth.SecretRef) error { return nil }

// Once a refresh succeeds, the server may have rotated the old refresh token
// away; losing the new bundle to a cancelled command context would lock the
// user out. Persistence must therefore not run on the command's context.
func TestPersistRefreshedSecretSurvivesCancelledContext(t *testing.T) {
	var persisted auth.SecretBundle
	secretInfo := profileSecret{
		store: ctxHonoringStore{persisted: &persisted},
		ref:   auth.SecretRef{Scheme: "keyring", Name: "wsectl/default"},
	}
	refreshed := expiringOAuthSecret()
	refreshed.AccessToken = "fresh-token"

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var warnings []string
	persistRefreshedSecret(ctx, secretInfo, refreshed, func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	})
	if persisted.AccessToken != "fresh-token" {
		t.Fatalf("refreshed bundle must persist despite a cancelled command context; warnings: %v", warnings)
	}
	if len(warnings) != 0 {
		t.Fatalf("persist should have succeeded silently, got %v", warnings)
	}
}

// A4 (env store): not persisting is the env store's designed behavior, so a
// refresh proceeds silently — no warning noise on every invocation.
func TestRefreshProfileSecretEnvStoreSkipsPersistSilently(t *testing.T) {
	refreshed := expiringOAuthSecret()
	refreshed.AccessToken = "fresh-token"
	stubRefreshSecret(t, refreshed, nil)

	var warnings []string
	warn := func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	}
	secretInfo := profileSecret{
		secret: expiringOAuthSecret(),
		store:  auth.EnvStore{},
		ref:    auth.SecretRef{Scheme: "env"},
	}
	got, err := refreshProfileSecret(context.Background(), config.Builtin(), config.Profile{AuthType: "oauth2"}, secretInfo, warn)
	if err != nil {
		t.Fatalf("env persist must not fail the command, got %v", err)
	}
	if got.AccessToken != "fresh-token" {
		t.Fatalf("command must proceed with the refreshed token, got %#v", got)
	}
	if len(warnings) != 0 {
		t.Fatalf("env store persist skip must be silent, got %v", warnings)
	}
}

// The warning sink writes to stderr only and is silenced by --quiet.
func TestWarnSinkWritesStderrAndRespectsQuiet(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	s := &state{}
	s.warnSink(cmd)("token for %s lost", "default")
	if out.Len() != 0 {
		t.Fatalf("warning leaked to stdout: %q", out.String())
	}
	if got := errOut.String(); got != "warning: token for default lost\n" {
		t.Fatalf("stderr = %q", got)
	}

	errOut.Reset()
	quiet := &state{quiet: true}
	quiet.warnSink(cmd)("suppressed")
	if errOut.Len() != 0 {
		t.Fatalf("--quiet must silence warnings, got %q", errOut.String())
	}
}

// writeReactiveProfile sets up an OAuth profile whose credential carries the
// stub server URL (the credential URL wins account-URL precedence, and config
// validation requires https for the configured value).
func writeReactiveProfile(t *testing.T, accountURL string) (secretPath string) {
	t.Helper()
	dir := t.TempDir()
	secretPath = filepath.Join(dir, "secret.json")
	raw, err := json.Marshal(auth.SecretBundle{
		ClientID:     "client_123",
		ClientSecret: "client_secret_123",
		AccessToken:  "stale-token",
		RefreshToken: "refresh_123",
		AccountURL:   accountURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secretPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.toml")
	cfg := fmt.Sprintf("current_profile = \"default\"\n\n[profiles.default]\naccount_url = \"https://placeholder.invalid\"\nauth_type = \"oauth2\"\nsecret_ref = %q\n", "plaintext:"+secretPath)
	if err := os.WriteFile(configPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WSECTL_CONFIG", configPath)
	return secretPath
}

// A3 end-to-end: a 401 from the API triggers one refresh, the request is
// replayed with the new token, and the refreshed bundle is persisted. The
// stored token has no recorded expiry, so the proactive path cannot help —
// this is exactly the case reactive refresh exists for.
func TestReactiveRefreshRecovers401EndToEnd(t *testing.T) {
	var gotAuth []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		if len(gotAuth) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"status":"error","status_code":0,"message":"Invalid token"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok","data":{"id":"1","name":"Ada"}}`))
	}))
	defer server.Close()

	secretPath := writeReactiveProfile(t, server.URL)
	refreshed := auth.SecretBundle{
		ClientID:     "client_123",
		ClientSecret: "client_secret_123",
		AccessToken:  "fresh-token",
		RefreshToken: "refresh_123",
	}
	calls := stubRefreshSecret(t, refreshed, nil)

	out, err := execute("me", "--json")
	if err != nil {
		t.Fatalf("401 with a refreshable credential must recover: %v\n%s", err, out)
	}
	if *calls != 1 {
		t.Fatalf("refresh calls = %d, want 1", *calls)
	}
	if len(gotAuth) != 2 || gotAuth[1] != "Bearer fresh-token" {
		t.Fatalf("replay must carry the refreshed bearer, got %v", gotAuth)
	}
	raw, readErr := os.ReadFile(secretPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(raw), "fresh-token") {
		t.Fatalf("refreshed token must be persisted to the store:\n%s", raw)
	}
}

// A3: when the refresh itself fails, the surfaced failure is the original
// API authentication error (exit 3), not the refresh implementation detail.
func TestReactiveRefreshFailureKeepsAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":"error","status_code":0,"message":"Invalid token"}`))
	}))
	defer server.Close()

	writeReactiveProfile(t, server.URL)
	calls := stubRefreshSecret(t, auth.SecretBundle{}, errors.New("refresh endpoint down"))

	out, err := execute("me")
	if err == nil {
		t.Fatalf("expected authentication failure, got success:\n%s", out)
	}
	if *calls != 1 {
		t.Fatalf("refresh calls = %d, want 1", *calls)
	}
	if commandExitCode(err) != 3 {
		t.Fatalf("exit = %d, want 3 (authentication)", commandExitCode(err))
	}
	if !strings.Contains(err.Error(), "Worksection HTTP 401") || strings.Contains(err.Error(), "refresh endpoint down") {
		t.Fatalf("error must be the original 401, not the refresh detail: %v", err)
	}
}

// executeSplit runs a command with separated streams so tests can assert
// warnings land on stderr while stdout stays data-only.
func executeSplit(args ...string) (stdout, stderr string, err error) {
	cmd := NewRoot("test", "commit", "date")
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

func reactive401Then200Server(t *testing.T) (server *httptest.Server, gotAuth *[]string) {
	t.Helper()
	var auths []string
	gotAuth = &auths
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auths = append(auths, r.Header.Get("Authorization"))
		if len(auths) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"status":"error","status_code":0,"message":"Invalid token"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok","data":{"id":"1","name":"Ada"}}`))
	}))
	t.Cleanup(server.Close)
	return server, gotAuth
}

// A3+A4 end-to-end for environment credentials: env-supplied OAuth tokens
// have no recorded expiry, so reactive refresh is their only recovery path.
// The read-only env store skips the write-back silently — no warning noise.
func TestReactiveRefreshEnvCredentialsEndToEnd(t *testing.T) {
	server, gotAuth := reactive401Then200Server(t)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	cfg := "current_profile = \"default\"\n\n[profiles.default]\naccount_url = \"https://placeholder.invalid\"\nauth_type = \"oauth2\"\nsecret_ref = \"env:\"\n"
	if err := os.WriteFile(configPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WSECTL_CONFIG", configPath)
	t.Setenv("WSECTL_ACCOUNT_URL", server.URL)
	t.Setenv("WSECTL_ACCESS_TOKEN", "stale-token")
	t.Setenv("WSECTL_REFRESH_TOKEN", "refresh_123")
	t.Setenv("WSECTL_CLIENT_ID", "client_123")
	t.Setenv("WSECTL_CLIENT_SECRET", "client_secret_123")

	refreshed := auth.SecretBundle{AccessToken: "fresh-token", RefreshToken: "refresh_123"}
	calls := stubRefreshSecret(t, refreshed, nil)

	stdout, stderr, err := executeSplit("me", "--json")
	if err != nil {
		t.Fatalf("env credentials must recover from 401 via reactive refresh: %v\nstderr: %s", err, stderr)
	}
	if *calls != 1 {
		t.Fatalf("refresh calls = %d, want 1", *calls)
	}
	if len(*gotAuth) != 2 || (*gotAuth)[1] != "Bearer fresh-token" {
		t.Fatalf("replay must carry the refreshed bearer, got %v", *gotAuth)
	}
	if strings.Contains(stderr, "warning") {
		t.Fatalf("read-only env store write-back skip must be silent:\n%s", stderr)
	}
	if !strings.Contains(stdout, `"status": "ok"`) {
		t.Fatalf("expected success envelope on stdout:\n%s", stdout)
	}
}

// A3+A4 end-to-end for a writable store that cannot persist: the command
// still succeeds with the refreshed token and the warning goes to stderr,
// never stdout.
func TestReactiveRefreshPersistFailureWarnsEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("read-only directories do not block file creation on Windows")
	}
	server, gotAuth := reactive401Then200Server(t)
	secretPath := writeReactiveProfile(t, server.URL)

	secretDir := filepath.Dir(secretPath)
	if err := os.Chmod(secretDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(secretDir, 0o700) })
	// Root (e.g. Docker CI) bypasses permission checks; probe and skip when
	// the directory is still writable so the test does not assert a warning
	// that cannot occur.
	probePath := filepath.Join(secretDir, "probe.txt")
	if err := os.WriteFile(probePath, []byte("probe"), 0o600); err == nil {
		_ = os.Remove(probePath)
		t.Skip("directory writable despite chmod 0500 (likely running as root)")
	}

	refreshed := auth.SecretBundle{
		ClientID:     "client_123",
		ClientSecret: "client_secret_123",
		AccessToken:  "fresh-token",
		RefreshToken: "refresh_123",
		AccountURL:   server.URL,
	}
	calls := stubRefreshSecret(t, refreshed, nil)

	stdout, stderr, err := executeSplit("me", "--json")
	if err != nil {
		t.Fatalf("persist failure after a successful refresh must be non-fatal: %v\nstderr: %s", err, stderr)
	}
	if *calls != 1 {
		t.Fatalf("refresh calls = %d, want 1", *calls)
	}
	if len(*gotAuth) != 2 || (*gotAuth)[1] != "Bearer fresh-token" {
		t.Fatalf("replay must carry the refreshed bearer, got %v", *gotAuth)
	}
	if !strings.Contains(stderr, "warning: refreshed OAuth token could not be persisted") {
		t.Fatalf("persist failure must warn on stderr:\n%s", stderr)
	}
	if strings.Contains(stdout, "could not be persisted") {
		t.Fatalf("warning leaked to stdout:\n%s", stdout)
	}
}

// Without refresh material (no refresh token), a 401 stays terminal and the
// refresher seam is never consulted.
func TestReactive401WithoutRefreshMaterialStaysTerminal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":"error","status_code":0,"message":"Invalid token"}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret.json")
	secretJSON := fmt.Sprintf(`{"access_token":"stale-token","account_url":%q}`, server.URL)
	if err := os.WriteFile(secretPath, []byte(secretJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.toml")
	cfg := fmt.Sprintf("current_profile = \"default\"\n\n[profiles.default]\naccount_url = \"https://placeholder.invalid\"\nauth_type = \"oauth2\"\nsecret_ref = %q\n", "plaintext:"+secretPath)
	if err := os.WriteFile(configPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WSECTL_CONFIG", configPath)
	calls := stubRefreshSecret(t, auth.SecretBundle{}, nil)

	_, err := execute("me")
	if err == nil || commandExitCode(err) != 3 {
		t.Fatalf("err = %v (exit %d), want authentication failure", err, commandExitCode(err))
	}
	if *calls != 0 {
		t.Fatalf("refresh calls = %d, want 0 without refresh material", *calls)
	}
}
