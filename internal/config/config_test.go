package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_OUTPUT", "json")
	t.Setenv("WSECTL_TIMEOUT", "5s")
	t.Setenv("WSECTL_RATE_LIMIT", "2/s")
	cfg, err := Load(context.Background(), Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Defaults.Output != "json" || cfg.Defaults.Timeout != "5s" || cfg.Defaults.RateLimit != "2/s" {
		t.Fatalf("unexpected defaults %#v", cfg.Defaults)
	}
}

func TestLoadFlagOverridesAndActiveProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`current_profile = "default"

[defaults]
output = "table"
rate_limit = "1/s"
timeout = "30s"

[profiles.default]
account_url = "https://default.worksection.com"
auth_type = "oauth2"
secret_ref = "keyring:wsectl/default"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(context.Background(), Overrides{
		ConfigPath: path,
		Profile:    "admin",
		AccountURL: "https://admin.worksection.com",
		Output:     "json",
		Timeout:    "5s",
		RateLimit:  "2/s",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Defaults.Output != "json" || cfg.Timeout() != 5*time.Second || cfg.RateLimit() != "2/s" {
		t.Fatalf("overrides not applied: %#v", cfg.Defaults)
	}
	name, profile, err := cfg.ActiveProfile()
	if err != nil {
		t.Fatal(err)
	}
	if name != "admin" || profile.AccountURL != "https://admin.worksection.com" || profile.AuthType != "oauth2" {
		t.Fatalf("unexpected active profile %s %#v", name, profile)
	}
}

func TestActiveProfileEnvironmentFallbackAndOverrides(t *testing.T) {
	cfg := Builtin()
	t.Setenv("WSECTL_PROFILE", "envprofile")
	t.Setenv("WSECTL_ACCOUNT_URL", "https://env.worksection.com")
	name, profile, err := cfg.ActiveProfile()
	if err != nil {
		t.Fatal(err)
	}
	if name != "envprofile" || profile.SecretRef != "env:" || profile.AccountURL != "https://env.worksection.com" {
		t.Fatalf("unexpected env profile %s %#v", name, profile)
	}

	cfg.Profiles["envprofile"] = Profile{AccountURL: "https://configured.worksection.com", AuthType: "admin_token", SecretRef: "keyring:wsectl/admin"}
	name, profile, err = cfg.ActiveProfile()
	if err != nil {
		t.Fatal(err)
	}
	if name != "envprofile" || profile.AccountURL != "https://env.worksection.com" || profile.AuthType != "admin_token" {
		t.Fatalf("environment account URL did not override configured profile: %s %#v", name, profile)
	}
}

func TestActiveProfileMissingWithoutEnvironmentFails(t *testing.T) {
	cfg := Builtin()
	_, _, err := cfg.ActiveProfile()
	if err == nil || !strings.Contains(err.Error(), "profile \"default\" not found") {
		t.Fatalf("expected missing profile error, got %v", err)
	}
}

func TestProfileSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Builtin()
	cfg.Path = path
	if err := AddProfile(&cfg, "default", Profile{AccountURL: "https://company.worksection.com", AuthType: "oauth2"}); err != nil {
		t.Fatal(err)
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(context.Background(), Overrides{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Profiles["default"].AccountURL != "https://company.worksection.com" {
		t.Fatalf("profile not loaded: %#v", loaded.Profiles)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestAddProfileRejectsUnsafeNewProfileName(t *testing.T) {
	cfg := Builtin()
	err := AddProfile(&cfg, "client.acme", Profile{AccountURL: "https://company.worksection.com", AuthType: "oauth2"})
	if err == nil || !strings.Contains(err.Error(), "letters, numbers, underscores, and hyphens") {
		t.Fatalf("expected profile name validation error, got %v", err)
	}
}

func TestValidateAllowsExistingUnusualProfileName(t *testing.T) {
	cfg := Builtin()
	cfg.Profiles["client.acme"] = Profile{AccountURL: "https://company.worksection.com", AuthType: "oauth2", SecretRef: "keyring:wsectl/client.acme"}
	if err := Validate(cfg); err != nil {
		t.Fatalf("existing unusual profile names should not fail config load: %v", err)
	}
}

func TestSaveSortsProfiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Builtin()
	cfg.Path = path
	if err := AddProfile(&cfg, "zeta", Profile{AccountURL: "https://zeta.worksection.com"}); err != nil {
		t.Fatal(err)
	}
	if err := AddProfile(&cfg, "alpha", Profile{AccountURL: "https://alpha.worksection.com"}); err != nil {
		t.Fatal(err)
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Index(text, "[profiles.alpha]") > strings.Index(text, "[profiles.zeta]") {
		t.Fatalf("profiles were not sorted:\n%s", text)
	}
}

func TestRemoveProfileUpdatesCurrentProfile(t *testing.T) {
	cfg := Builtin()
	if err := AddProfile(&cfg, "default", Profile{AccountURL: "https://company.worksection.com"}); err != nil {
		t.Fatal(err)
	}
	if err := AddProfile(&cfg, "other", Profile{AccountURL: "https://other.worksection.com"}); err != nil {
		t.Fatal(err)
	}
	cfg.CurrentProfile = "other"
	if err := RemoveProfile(&cfg, "other"); err != nil {
		t.Fatal(err)
	}
	if cfg.CurrentProfile != "default" {
		t.Fatalf("current profile = %q, want default", cfg.CurrentProfile)
	}
	if err := RemoveProfile(&cfg, "missing"); err == nil {
		t.Fatal("expected missing profile removal to fail")
	}
}

func TestValidateRejectsInvalidProfileValues(t *testing.T) {
	cfg := Builtin()
	cfg.Profiles["bad"] = Profile{AccountURL: "http://company.worksection.com", AuthType: "oauth2", SecretRef: "keyring:wsectl/bad"}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected account URL validation error, got %v", err)
	}
	cfg.Profiles["bad"] = Profile{AccountURL: "https://company.worksection.com", AuthType: "oauth2", SecretRef: "unknown:x"}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "unsupported secret store") {
		t.Fatalf("expected secret ref validation error, got %v", err)
	}
	cfg.Profiles["bad"] = Profile{AccountURL: "https://company.worksection.com", AuthType: "oauth2", SecretRef: "keyring:wsectl/bad"}
	cfg.Defaults.RateLimit = "fast"
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("expected rate-limit validation error, got %v", err)
	}
}
