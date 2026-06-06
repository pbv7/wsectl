package doctor

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/pbv7/wsectl/internal/auth"
	"github.com/pbv7/wsectl/internal/config"
	"github.com/pbv7/wsectl/internal/worksection"
)

// Status is the severity of one diagnostic check.
type Status string

const (
	// StatusOK means a diagnostic check passed.
	StatusOK Status = "ok"
	// StatusWarn means setup is usable but deserves attention.
	StatusWarn Status = "warn"
	// StatusFail means setup is not usable for the checked workflow.
	StatusFail Status = "fail"
)

// Check is one actionable doctor diagnostic.
type Check struct {
	Name        string `json:"name"`
	Status      Status `json:"status"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

// Report is the stable machine-readable result of a doctor run.
type Report struct {
	Healthy     bool     `json:"healthy"`
	APIChecked  bool     `json:"api_checked"`
	ConfigPath  string   `json:"config_path,omitempty"`
	Profile     string   `json:"profile,omitempty"`
	AccountURL  string   `json:"account_url,omitempty"`
	Checks      []Check  `json:"checks"`
	Remediation []string `json:"remediation"`
}

// Options controls optional doctor behavior.
type Options struct {
	Overrides config.Overrides
	CheckAPI  bool
}

// Dependencies makes doctor checks deterministic and testable.
type Dependencies struct {
	LoadConfig func(context.Context, config.Overrides) (config.Config, error)
	Stat       func(string) (os.FileInfo, error)
	LoadSecret func(context.Context, auth.SecretRef) (auth.SecretBundle, error)
	APICheck   func(context.Context) error
	Now        func() time.Time
	GOOS       string
}

// DefaultDependencies returns the production doctor integrations.
func DefaultDependencies() Dependencies {
	return Dependencies{
		LoadConfig: config.Load,
		Stat:       os.Stat,
		LoadSecret: func(ctx context.Context, ref auth.SecretRef) (auth.SecretBundle, error) {
			store, err := auth.StoreFor(ref)
			if err != nil {
				return auth.SecretBundle{}, err
			}
			return store.Get(ctx, ref)
		},
		Now:  time.Now,
		GOOS: runtime.GOOS,
	}
}

// Run diagnoses configuration and credentials, and optionally API access.
func Run(ctx context.Context, opts Options, deps Dependencies) (Report, error) {
	deps = withDefaults(deps)
	report := Report{Checks: []Check{}, Remediation: []string{}}
	cfg, err := deps.LoadConfig(ctx, opts.Overrides)
	if err != nil {
		report.ConfigPath = effectiveConfigPath(opts.Overrides.ConfigPath)
		add(&report, StatusFail, "config", err.Error(), "Fix the config file or select another one with --config PATH.")
		return finalize(report, worksection.CodeUsage)
	}
	report.ConfigPath = cfg.Path
	checkConfigFile(&report, cfg.Path, deps)
	if err := config.Validate(cfg); err != nil {
		add(&report, StatusFail, "config_validation", err.Error(), "Correct the invalid configuration value.")
	} else {
		add(&report, StatusOK, "config_validation", "configuration values are valid", "")
	}
	if _, err := worksection.NewLimiter(cfg.RateLimit()); err != nil {
		add(&report, StatusFail, "rate_limit", err.Error(), "Set rate_limit to a value such as 1/s.")
	} else {
		add(&report, StatusOK, "rate_limit", "effective rate limit is "+cfg.RateLimit(), "")
	}
	if cfg.Timeout() <= 0 {
		add(&report, StatusFail, "timeout", "effective timeout must be positive", "Set timeout to a positive duration such as 30s.")
	} else {
		add(&report, StatusOK, "timeout", "effective timeout is "+cfg.Timeout().String(), "")
	}

	profileName, profile, err := cfg.ActiveProfile()
	report.Profile = profileName
	if err != nil {
		add(&report, StatusFail, "profile", err.Error(), "Run `wsectl profiles add NAME --account-url URL --auth-type oauth2` or set WSECTL_ACCOUNT_URL.")
		return finalize(report, worksection.CodeUsage)
	}
	add(&report, StatusOK, "profile", "active profile is "+profileName, "")
	report.AccountURL = profile.AccountURL
	if err := validateAccountURL(profile.AccountURL); err != nil {
		add(&report, StatusFail, "account_url", err.Error(), "Set a valid HTTPS Worksection account URL.")
	} else {
		add(&report, StatusOK, "account_url", profile.AccountURL, "")
	}

	authType := profile.AuthType
	if authType == "" {
		authType = "oauth2"
	}
	switch authType {
	case "oauth2", "admin_token":
		add(&report, StatusOK, "auth_type", authType, "")
	default:
		add(&report, StatusFail, "auth_type", "unsupported auth type "+authType, "Use oauth2 or admin_token.")
	}

	ref, err := auth.ParseRef(profile.SecretRef)
	if err != nil {
		add(&report, StatusFail, "secret_ref", err.Error(), "Fix the profile secret_ref.")
		return finalize(report, worksection.CodeUsage)
	}
	add(&report, StatusOK, "secret_ref", ref.Scheme+" secret store selected", "")
	if ref.Scheme == "plaintext" {
		add(&report, StatusWarn, "plaintext_store", "plaintext secret storage is enabled", "Use keyring or encrypted-file storage for persistent credentials.")
	}
	if _, err := auth.StoreFor(ref); err != nil {
		add(&report, StatusFail, "secret_backend", err.Error(), "Use a supported secret store: keyring, env, encrypted-file, or plaintext.")
		return finalize(report, worksection.CodeUsage)
	}
	add(&report, StatusOK, "secret_backend", ref.Scheme+" backend is supported", "")

	secret, err := deps.LoadSecret(ctx, ref)
	if err != nil {
		add(&report, StatusFail, "credentials", "credentials are unavailable from the "+ref.Scheme+" backend", "Run `wsectl auth login` or provide WSECTL_* credential variables.")
		return finalize(report, worksection.CodeAuth)
	}
	if err := validateCredentials(authType, secret); err != nil {
		add(&report, StatusFail, "credentials", err.Error(), "Run `wsectl auth login` or provide the required WSECTL_* credential variable.")
		return finalize(report, worksection.CodeAuth)
	}
	add(&report, StatusOK, "credentials", credentialSummary(authType, secret), "")
	if checkExpiry(&report, authType, secret, deps.Now()) {
		return finalize(report, worksection.CodeAuth)
	}

	if opts.CheckAPI && !hasFailure(report.Checks) {
		report.APIChecked = true
		if deps.APICheck == nil {
			add(&report, StatusFail, "api", "API diagnostic is unavailable", "Retry without --api or reinstall wsectl.")
			return finalize(report, worksection.CodeGeneral)
		}
		if err := deps.APICheck(ctx); err != nil {
			code := classifyAPIError(err)
			add(&report, StatusFail, "api", err.Error(), apiRemediation(code))
			return finalize(report, code)
		}
		add(&report, StatusOK, "api", "authenticated `me` request succeeded", "")
	}
	return finalize(report, worksection.CodeGeneral)
}

func withDefaults(deps Dependencies) Dependencies {
	defaults := DefaultDependencies()
	if deps.LoadConfig == nil {
		deps.LoadConfig = defaults.LoadConfig
	}
	if deps.Stat == nil {
		deps.Stat = defaults.Stat
	}
	if deps.LoadSecret == nil {
		deps.LoadSecret = defaults.LoadSecret
	}
	if deps.Now == nil {
		deps.Now = defaults.Now
	}
	if deps.GOOS == "" {
		deps.GOOS = defaults.GOOS
	}
	return deps
}

func checkConfigFile(report *Report, path string, deps Dependencies) {
	info, err := deps.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		add(report, StatusWarn, "config_file", "config file does not exist; environment-only configuration may still work", "Run `wsectl profiles add ...` to create a config file.")
		return
	}
	if err != nil {
		add(report, StatusFail, "config_file", err.Error(), "Check that the config path is readable.")
		return
	}
	add(report, StatusOK, "config_file", path+" is readable", "")
	if deps.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		add(report, StatusWarn, "config_permissions", fmt.Sprintf("config permissions are %04o", info.Mode().Perm()), "Run `chmod 600 "+path+"`.")
	} else {
		add(report, StatusOK, "config_permissions", "config permissions are restricted", "")
	}
}

func validateAccountURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("invalid account URL %q", raw)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("account URL must use https")
	}
	return nil
}

func validateCredentials(authType string, secret auth.SecretBundle) error {
	switch authType {
	case "admin_token":
		if secret.AdminToken == "" {
			return fmt.Errorf("admin token is missing")
		}
	default:
		if secret.AccessToken == "" {
			return fmt.Errorf("OAuth access token is missing")
		}
	}
	return nil
}

func credentialSummary(authType string, secret auth.SecretBundle) string {
	if authType == "admin_token" {
		return "admin token is present"
	}
	parts := []string{"OAuth access token is present"}
	if secret.RefreshToken != "" {
		parts = append(parts, "refresh token is present")
	}
	return strings.Join(parts, "; ")
}

func checkExpiry(report *Report, authType string, secret auth.SecretBundle, now time.Time) bool {
	if authType != "oauth2" {
		return false
	}
	if secret.AccessExpires.IsZero() {
		add(report, StatusWarn, "token_expiry", "OAuth access-token expiry is not recorded", "Use `wsectl auth refresh` if the API rejects the token.")
		return false
	}
	if !secret.AccessExpires.After(now) {
		if secret.RefreshToken != "" {
			add(report, StatusWarn, "token_expiry", "OAuth access token is expired but a refresh token is available", "Run `wsectl auth refresh` or an API command to refresh it.")
			return false
		}
		add(report, StatusFail, "token_expiry", "OAuth access token is expired and cannot be refreshed", "Run `wsectl auth login`.")
		return true
	}
	if auth.NeedsRefresh(secret.AccessExpires, now) {
		if secret.RefreshToken != "" {
			add(report, StatusWarn, "token_expiry", "OAuth access token expires soon and can be refreshed", "Run `wsectl auth refresh` or continue with an API command.")
			return false
		}
		add(report, StatusWarn, "token_expiry", "OAuth access token expires soon and no refresh token is stored", "Run `wsectl auth login` before it expires.")
		return false
	}
	add(report, StatusOK, "token_expiry", "OAuth access token is not near expiry", "")
	return false
}

func classifyAPIError(err error) worksection.ErrorCode {
	var wsErr *worksection.Error
	if errors.As(err, &wsErr) {
		switch wsErr.Code {
		case worksection.CodeAuth, worksection.CodeAuthorization, worksection.CodeNetwork,
			worksection.CodeRateLimited, worksection.CodeAPI:
			return wsErr.Code
		}
		return worksection.CodeAPI
	}
	return worksection.CodeNetwork
}

func apiRemediation(code worksection.ErrorCode) string {
	switch code {
	case worksection.CodeAuth:
		return "Run `wsectl auth login` and retry."
	case worksection.CodeAuthorization:
		return "Check the Worksection account permissions for this credential."
	case worksection.CodeNetwork:
		return "Check DNS, proxy, TLS, and account URL connectivity."
	case worksection.CodeRateLimited:
		return "Wait before retrying and keep requests at or below the configured rate limit."
	default:
		return "Inspect the Worksection API error and account permissions."
	}
}

func add(report *Report, status Status, name, message, remediation string) {
	report.Checks = append(report.Checks, Check{Name: name, Status: status, Message: message, Remediation: remediation})
	if remediation != "" && status != StatusOK {
		report.Remediation = append(report.Remediation, remediation)
	}
}

func hasFailure(checks []Check) bool {
	for _, check := range checks {
		if check.Status == StatusFail {
			return true
		}
	}
	return false
}

func finalize(report Report, code worksection.ErrorCode) (Report, error) {
	report.Healthy = !hasFailure(report.Checks)
	report.Remediation = uniqueSorted(report.Remediation)
	if report.Healthy {
		return report, nil
	}
	if code == worksection.CodeGeneral {
		code = worksection.CodeUsage
	}
	return report, &worksection.Error{
		Code:    code,
		Message: "doctor found one or more failing checks",
		Details: map[string]any{"failed_checks": failedCheckNames(report.Checks)},
	}
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func failedCheckNames(checks []Check) []string {
	var names []string
	for _, check := range checks {
		if check.Status == StatusFail {
			names = append(names, check.Name)
		}
	}
	return names
}

func effectiveConfigPath(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}
	if envPath := os.Getenv("WSECTL_CONFIG"); envPath != "" {
		return envPath
	}
	return config.DefaultConfigPath()
}
