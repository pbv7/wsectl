package config

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/pbv7/wsectl/internal/atomicfile"
	"github.com/spf13/viper"
)

type Defaults struct {
	Output    string `mapstructure:"output"`
	RateLimit string `mapstructure:"rate_limit"`
	Timeout   string `mapstructure:"timeout"`
}

type History struct {
	Enabled       bool   `mapstructure:"enabled"`
	Path          string `mapstructure:"path"`
	IncludeParams string `mapstructure:"include_params"`
}

// Profile identifies a Worksection account and the credential reference used
// to authenticate requests for that account.
type Profile struct {
	AccountURL string `mapstructure:"account_url"`
	AuthType   string `mapstructure:"auth_type"`
	SecretRef  string `mapstructure:"secret_ref"`
}

// Config is the validated in-memory representation of config.toml plus
// environment and flag overrides.
type Config struct {
	Path           string             `mapstructure:"-"`
	CurrentProfile string             `mapstructure:"current_profile"`
	Defaults       Defaults           `mapstructure:"defaults"`
	History        History            `mapstructure:"history"`
	Profiles       map[string]Profile `mapstructure:"profiles"`
}

// Overrides contains values supplied by global CLI flags.
type Overrides struct {
	ConfigPath string
	Profile    string
	AccountURL string
	Output     string
	Timeout    string
	RateLimit  string
	Debug      bool
}

// Builtin returns the fallback configuration used when no config file exists.
func Builtin() Config {
	return Config{
		CurrentProfile: "default",
		Defaults: Defaults{
			Output:    "auto",
			RateLimit: "1/s",
			Timeout:   "30s",
		},
		History: History{
			Enabled:       false,
			Path:          "",
			IncludeParams: "all",
		},
		Profiles: map[string]Profile{},
	}
}

// Load reads config.toml, applies environment variables and flags, and returns
// a typed configuration ready for command execution.
func Load(_ context.Context, overrides Overrides) (Config, error) {
	cfg := Builtin()
	path := firstNonEmpty(overrides.ConfigPath, os.Getenv("WSECTL_CONFIG"), DefaultConfigPath())
	// Keep "." inside profile keys such as client.acme instead of treating
	// them as nested Viper paths.
	v := viper.NewWithOptions(viper.KeyDelimiter("::"))
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && !os.IsNotExist(err) {
			return cfg, err
		}
	} else if err := v.Unmarshal(&cfg); err != nil {
		return cfg, err
	}
	cfg.Path = path
	envSources, err := applyEnv(&cfg)
	if err != nil {
		return cfg, err
	}
	applyOverrides(&cfg, overrides)
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	if err := Validate(cfg); err != nil {
		return cfg, envOverrideError(err, envSources, overrides)
	}
	return cfg, nil
}

// ActiveProfileName returns the selected profile after environment override
// handling.
func (c Config) ActiveProfileName() string {
	if env := os.Getenv("WSECTL_PROFILE"); env != "" {
		return env
	}
	if c.CurrentProfile == "" {
		return "default"
	}
	return c.CurrentProfile
}

// ActiveProfile returns the active profile, falling back to environment-only
// credentials when no configured profile exists.
func (c Config) ActiveProfile() (string, Profile, error) {
	name := c.ActiveProfileName()
	p, ok := c.Profiles[name]
	if !ok {
		p = Profile{
			AccountURL: os.Getenv("WSECTL_ACCOUNT_URL"),
			AuthType:   "oauth2",
			SecretRef:  "env:",
		}
		if p.AccountURL == "" {
			return name, p, fmt.Errorf("profile %q not found and WSECTL_ACCOUNT_URL is not set", name)
		}
	}
	if env := os.Getenv("WSECTL_ACCOUNT_URL"); env != "" {
		p.AccountURL = env
	}
	return name, p, nil
}

// Timeout parses the effective request timeout.
func (c Config) Timeout() time.Duration {
	d, err := time.ParseDuration(firstNonEmpty(os.Getenv("WSECTL_TIMEOUT"), c.Defaults.Timeout, "30s"))
	if err != nil {
		return 30 * time.Second
	}
	return d
}

// RateLimit returns the effective rate-limit spec.
func (c Config) RateLimit() string {
	return firstNonEmpty(os.Getenv("WSECTL_RATE_LIMIT"), c.Defaults.RateLimit, "1/s")
}

// DefaultConfigPath returns the platform-specific default config path.
func DefaultConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "wsectl", "config.toml")
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("AppData"); appData != "" {
			return filepath.Join(appData, "wsectl", "config.toml")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.toml"
	}
	return filepath.Join(home, ".config", "wsectl", "config.toml")
}

// DefaultStateDir returns the platform-specific default state directory.
func DefaultStateDir() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "wsectl")
	}
	if runtime.GOOS == "windows" {
		if localAppData := firstNonEmpty(os.Getenv("LocalAppData"), os.Getenv("LOCALAPPDATA")); localAppData != "" {
			return filepath.Join(localAppData, "wsectl")
		}
		if appData := firstNonEmpty(os.Getenv("AppData"), os.Getenv("APPDATA")); appData != "" {
			return filepath.Join(appData, "wsectl")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".local", "state", "wsectl")
}

// DefaultHistoryPath returns the default JSONL action history path.
func DefaultHistoryPath() string {
	return filepath.Join(DefaultStateDir(), "history.jsonl")
}

type envSources struct {
	output        string
	timeout       string
	rateLimit     string
	historyParams string
}

func applyEnv(cfg *Config) (envSources, error) {
	var sources envSources
	if v := os.Getenv("WSECTL_OUTPUT"); v != "" {
		cfg.Defaults.Output = v
		sources.output = v
	}
	if v := os.Getenv("WSECTL_TIMEOUT"); v != "" {
		cfg.Defaults.Timeout = v
		sources.timeout = v
	}
	if v := os.Getenv("WSECTL_RATE_LIMIT"); v != "" {
		cfg.Defaults.RateLimit = v
		sources.rateLimit = v
	}
	if v := os.Getenv("WSECTL_PROFILE"); v != "" {
		cfg.CurrentProfile = v
	}
	if v := os.Getenv("WSECTL_HISTORY"); v != "" {
		enabled, err := parseEnvBool(v)
		if err != nil {
			return sources, fmt.Errorf("WSECTL_HISTORY=%q: %w", v, err)
		}
		cfg.History.Enabled = enabled
	}
	if v := os.Getenv("WSECTL_HISTORY_FILE"); v != "" {
		cfg.History.Path = v
	}
	if v := os.Getenv("WSECTL_HISTORY_PARAMS"); v != "" {
		cfg.History.IncludeParams = v
		sources.historyParams = v
	}
	return sources, nil
}

func applyOverrides(cfg *Config, o Overrides) {
	if o.Profile != "" {
		cfg.CurrentProfile = o.Profile
	}
	if o.Output != "" {
		cfg.Defaults.Output = o.Output
	}
	if o.Timeout != "" {
		cfg.Defaults.Timeout = o.Timeout
	}
	if o.RateLimit != "" {
		cfg.Defaults.RateLimit = o.RateLimit
	}
	if o.AccountURL != "" {
		name := cfg.ActiveProfileName()
		p := cfg.Profiles[name]
		p.AccountURL = o.AccountURL
		if p.AuthType == "" {
			p.AuthType = "oauth2"
		}
		cfg.Profiles[name] = p
	}
}

// Save writes config.toml with owner-only permissions.
func Save(cfg Config) error {
	if cfg.Path == "" {
		cfg.Path = DefaultConfigPath()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "current_profile = %q\n\n", cfg.CurrentProfile)
	fmt.Fprintf(&b, "[defaults]\noutput = %q\nrate_limit = %q\ntimeout = %q\n\n",
		cfg.Defaults.Output, cfg.Defaults.RateLimit, cfg.Defaults.Timeout)
	fmt.Fprintf(&b, "[history]\nenabled = %t\npath = %q\ninclude_params = %q\n\n",
		cfg.History.Enabled, cfg.History.Path, cfg.History.IncludeParams)
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		p := cfg.Profiles[name]
		fmt.Fprintf(&b, "[profiles.%s]\naccount_url = %q\nauth_type = %q\nsecret_ref = %q\n\n",
			tomlProfileKey(name), p.AccountURL, p.AuthType, p.SecretRef)
	}
	return atomicfile.WriteFile(cfg.Path, []byte(b.String()), 0o600)
}

func envOverrideError(err error, sources envSources, overrides Overrides) error {
	kind, _ := validationKindOf(err)
	if overrides.Output == "" && sources.output != "" && kind == validationKindOutput {
		return fmt.Errorf("WSECTL_OUTPUT=%q: %w", sources.output, err)
	}
	if overrides.Timeout == "" && sources.timeout != "" && kind == validationKindTimeout {
		return fmt.Errorf("WSECTL_TIMEOUT=%q: %w", sources.timeout, err)
	}
	if overrides.RateLimit == "" && sources.rateLimit != "" && kind == validationKindRateLimit {
		return fmt.Errorf("WSECTL_RATE_LIMIT=%q: %w", sources.rateLimit, err)
	}
	if sources.historyParams != "" && kind == validationKindHistory {
		return fmt.Errorf("WSECTL_HISTORY_PARAMS=%q: %w", sources.historyParams, err)
	}
	return err
}

func parseEnvBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid history setting %q, expected 1/0/true/false", value)
	}
}

func tomlProfileKey(name string) string {
	if ValidateNewProfileName(name) == nil {
		return name
	}
	return tomlBasicString(name)
}

func tomlBasicString(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func ValidateAccountURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("invalid account URL %q", raw)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("account URL must use https")
	}
	return nil
}

func ValidateSecretRef(ref string) error {
	if ref == "" {
		return nil
	}
	scheme, name, ok := strings.Cut(ref, ":")
	if !ok || scheme == "" {
		return fmt.Errorf("invalid secret_ref %q", ref)
	}
	switch scheme {
	case "env":
		return nil
	case "keyring", "encrypted-file", "plaintext":
		if name == "" {
			return fmt.Errorf("secret_ref %q requires a target name or path", ref)
		}
		return nil
	default:
		return fmt.Errorf("unsupported secret store %q", scheme)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
