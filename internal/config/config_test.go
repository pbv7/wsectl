package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
