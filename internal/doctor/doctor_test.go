package doctor

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/pbv7/wsectl/internal/auth"
	"github.com/pbv7/wsectl/internal/config"
	"github.com/pbv7/wsectl/internal/worksection"
)

func TestMissingConfigWithEnvironmentCredentialsIsHealthy(t *testing.T) {
	t.Setenv("WSECTL_ACCOUNT_URL", "https://company.worksection.com")
	report, err := Run(context.Background(), Options{}, Dependencies{
		LoadConfig: func(context.Context, config.Overrides) (config.Config, error) {
			cfg := config.Builtin()
			cfg.Path = "/missing/config.toml"
			return cfg, nil
		},
		Stat: func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		LoadSecret: func(context.Context, auth.SecretRef) (auth.SecretBundle, error) {
			return auth.SecretBundle{AccessToken: "secret"}, nil
		},
		Now:  fixedNow,
		GOOS: "linux",
	})
	if err != nil || !report.Healthy {
		t.Fatalf("environment-only setup should be healthy: report=%#v err=%v", report, err)
	}
	assertCheck(t, report, "config_file", StatusWarn)
	assertCheck(t, report, "credentials", StatusOK)
}

func TestInvalidProfileIsUsageFailure(t *testing.T) {
	t.Setenv("WSECTL_ACCOUNT_URL", "")
	cfg := config.Builtin()
	cfg.Path = "/missing/config.toml"
	report, err := Run(context.Background(), Options{}, dependenciesFor(cfg, auth.SecretBundle{}))
	assertExitCode(t, err, 2)
	assertCheck(t, report, "profile", StatusFail)
}

func TestUnavailableKeyringIsAuthenticationFailure(t *testing.T) {
	cfg := testConfig("keyring:wsectl/default")
	deps := dependenciesFor(cfg, auth.SecretBundle{})
	deps.LoadSecret = func(context.Context, auth.SecretRef) (auth.SecretBundle, error) {
		return auth.SecretBundle{}, errors.New("keyring service unavailable")
	}
	report, err := Run(context.Background(), Options{}, deps)
	assertExitCode(t, err, 3)
	assertCheck(t, report, "secret_backend", StatusOK)
	assertCheck(t, report, "credentials", StatusFail)
}

func TestExpiredOAuthWithRefreshTokenWarnsButPasses(t *testing.T) {
	cfg := testConfig("env:")
	secret := auth.SecretBundle{
		AccessToken:   "secret",
		RefreshToken:  "refresh",
		AccessExpires: fixedNow().Add(-time.Minute),
	}
	report, err := Run(context.Background(), Options{}, dependenciesFor(cfg, secret))
	if err != nil || !report.Healthy {
		t.Fatalf("refreshable OAuth should remain healthy: report=%#v err=%v", report, err)
	}
	assertCheck(t, report, "token_expiry", StatusWarn)
}

func TestExpiredOAuthWithoutRefreshTokenFails(t *testing.T) {
	cfg := testConfig("env:")
	secret := auth.SecretBundle{
		AccessToken:   "secret",
		AccessExpires: fixedNow().Add(-time.Minute),
	}
	report, err := Run(context.Background(), Options{}, dependenciesFor(cfg, secret))
	assertExitCode(t, err, 3)
	assertCheck(t, report, "token_expiry", StatusFail)
}

func TestPlaintextStoreProducesWarning(t *testing.T) {
	cfg := testConfig("plaintext:/tmp/wsectl-secrets.json")
	secret := auth.SecretBundle{AccessToken: "secret", AccessExpires: fixedNow().Add(time.Hour)}
	report, err := Run(context.Background(), Options{}, dependenciesFor(cfg, secret))
	if err != nil {
		t.Fatal(err)
	}
	assertCheck(t, report, "plaintext_store", StatusWarn)
}

func TestAdminTokenProfileSkipsOAuthExpiryChecks(t *testing.T) {
	cfg := testConfig("env:")
	profile := cfg.Profiles["default"]
	profile.AuthType = "admin_token"
	cfg.Profiles["default"] = profile
	secret := auth.SecretBundle{
		AdminToken:    "admin-secret",
		AccessToken:   "",
		AccessExpires: fixedNow().Add(-time.Hour),
	}
	report, err := Run(context.Background(), Options{}, dependenciesFor(cfg, secret))
	if err != nil || !report.Healthy {
		t.Fatalf("admin token profile should be healthy without OAuth expiry checks: report=%#v err=%v", report, err)
	}
	assertCheck(t, report, "auth_type", StatusOK)
	assertCheck(t, report, "credentials", StatusOK)
	if hasCheck(report, "token_expiry") {
		t.Fatalf("admin token profile should not emit token_expiry check: %#v", report.Checks)
	}
}

func TestInvalidAccountURLIsUsageFailure(t *testing.T) {
	cfg := testConfig("env:")
	profile := cfg.Profiles["default"]
	profile.AccountURL = "http://company.worksection.com"
	cfg.Profiles["default"] = profile
	report, err := Run(context.Background(), Options{}, dependenciesFor(cfg, validSecret()))
	assertExitCode(t, err, 2)
	assertCheck(t, report, "account_url", StatusFail)
}

func TestExistingUnusualProfileNameWarnsButDoesNotFail(t *testing.T) {
	cfg := testConfig("env:")
	cfg.Profiles["client.acme"] = cfg.Profiles["default"]
	secret := validSecret()
	report, err := Run(context.Background(), Options{}, dependenciesFor(cfg, secret))
	if err != nil || !report.Healthy {
		t.Fatalf("unusual existing profile name should only warn: report=%#v err=%v", report, err)
	}
	assertCheck(t, report, "profile_names", StatusWarn)
}

func TestEnabledHistoryPathMustBeWritable(t *testing.T) {
	cfg := testConfig("env:")
	dir := t.TempDir()
	blocker := dir + "/not-a-directory"
	if err := os.WriteFile(blocker, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.History.Enabled = true
	cfg.History.Path = blocker + "/history.jsonl"
	report, err := Run(context.Background(), Options{}, dependenciesFor(cfg, validSecret()))
	assertExitCode(t, err, 2)
	assertCheck(t, report, "history", StatusFail)
}

func TestKeyringCredentialFailureUsesBackendRemediation(t *testing.T) {
	cfg := testConfig("keyring:wsectl/default")
	deps := dependenciesFor(cfg, auth.SecretBundle{})
	deps.LoadSecret = func(context.Context, auth.SecretRef) (auth.SecretBundle, error) {
		return auth.SecretBundle{}, errors.New("no available keyring backend")
	}
	report, err := Run(context.Background(), Options{}, deps)
	assertExitCode(t, err, 3)
	for _, remediation := range report.Remediation {
		if remediation == disabledKeyringBackendRemediation {
			return
		}
	}
	t.Fatalf("missing disabled keyring remediation: %#v", report.Remediation)
}

func TestAPICheckSuccess(t *testing.T) {
	cfg := testConfig("env:")
	deps := dependenciesFor(cfg, validSecret())
	deps.APICheck = func(context.Context) error { return nil }
	report, err := Run(context.Background(), Options{CheckAPI: true}, deps)
	if err != nil || !report.Healthy || !report.APIChecked {
		t.Fatalf("API check should pass: report=%#v err=%v", report, err)
	}
	assertCheck(t, report, "api", StatusOK)
}

func TestAPICheckClassifications(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
	}{
		{
			name: "unauthorized",
			err:  &worksection.Error{Code: worksection.CodeAuth, Message: "unauthorized"},
			code: 3,
		},
		{
			name: "timeout",
			err:  &worksection.Error{Code: worksection.CodeNetwork, Message: "request timed out"},
			code: 5,
		},
		{
			name: "network",
			err:  errors.New("connection refused"),
			code: 5,
		},
		{
			name: "api",
			err:  &worksection.Error{Code: worksection.CodeAPI, Message: "API error"},
			code: 6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig("env:")
			deps := dependenciesFor(cfg, validSecret())
			deps.APICheck = func(context.Context) error { return tt.err }
			report, err := Run(context.Background(), Options{CheckAPI: true}, deps)
			assertExitCode(t, err, tt.code)
			if !report.APIChecked {
				t.Fatal("API check was not recorded")
			}
			assertCheck(t, report, "api", StatusFail)
		})
	}
}

func TestInvalidConfigAndEffectiveValues(t *testing.T) {
	report, err := Run(context.Background(), Options{}, Dependencies{
		LoadConfig: func(context.Context, config.Overrides) (config.Config, error) {
			return config.Config{}, errors.New("invalid timeout \"tomorrow\"")
		},
	})
	assertExitCode(t, err, 2)
	assertCheck(t, report, "config", StatusFail)

	cfg := testConfig("env:")
	cfg.Defaults.RateLimit = "invalid"
	report, err = Run(context.Background(), Options{}, dependenciesFor(cfg, validSecret()))
	assertExitCode(t, err, 2)
	assertCheck(t, report, "rate_limit", StatusFail)
}

func testConfig(secretRef string) config.Config {
	cfg := config.Builtin()
	cfg.Path = "/missing/config.toml"
	cfg.CurrentProfile = "default"
	cfg.Profiles["default"] = config.Profile{
		AccountURL: "https://company.worksection.com",
		AuthType:   "oauth2",
		SecretRef:  secretRef,
	}
	return cfg
}

func dependenciesFor(cfg config.Config, secret auth.SecretBundle) Dependencies {
	return Dependencies{
		LoadConfig: func(context.Context, config.Overrides) (config.Config, error) {
			return cfg, nil
		},
		Stat: func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		LoadSecret: func(context.Context, auth.SecretRef) (auth.SecretBundle, error) {
			return secret, nil
		},
		Now:  fixedNow,
		GOOS: "linux",
	}
}

func validSecret() auth.SecretBundle {
	return auth.SecretBundle{AccessToken: "secret", RefreshToken: "refresh", AccessExpires: fixedNow().Add(time.Hour)}
}

func fixedNow() time.Time {
	return time.Date(2026, time.June, 6, 12, 0, 0, 0, time.UTC)
}

func assertCheck(t *testing.T, report Report, name string, status Status) {
	t.Helper()
	for _, check := range report.Checks {
		if check.Name == name {
			if check.Status != status {
				t.Fatalf("%s status = %s, want %s", name, check.Status, status)
			}
			return
		}
	}
	t.Fatalf("missing check %q in %#v", name, report.Checks)
}

func hasCheck(report Report, name string) bool {
	for _, check := range report.Checks {
		if check.Name == name {
			return true
		}
	}
	return false
}

func assertExitCode(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected exit code %d, got nil", want)
	}
	exitErr, ok := err.(interface{ ExitCode() int })
	if !ok {
		t.Fatalf("error does not expose an exit code: %T %v", err, err)
	}
	if got := exitErr.ExitCode(); got != want {
		t.Fatalf("exit code = %d, want %d (%v)", got, want, err)
	}
}
