package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Validate checks the typed configuration after all sources have been merged.
func Validate(cfg Config) error {
	if cfg.Defaults.Output == "" {
		cfg.Defaults.Output = "auto"
	}
	if !ValidOutput(cfg.Defaults.Output) {
		return validationFailure(validationKindOutput, "invalid output %q", cfg.Defaults.Output)
	}
	if d, err := time.ParseDuration(cfg.Defaults.Timeout); cfg.Defaults.Timeout != "" && err != nil {
		return validationFailure(validationKindTimeout, "invalid timeout %q", cfg.Defaults.Timeout)
	} else if cfg.Defaults.Timeout != "" && d <= 0 {
		return validationFailure(validationKindTimeout, "timeout must be positive")
	}
	if err := validateRateLimit(cfg.Defaults.RateLimit); err != nil {
		return err
	}
	switch cfg.History.IncludeParams {
	case "", "none", "safe", "all":
	default:
		return validationFailure(validationKindHistory, "invalid history include_params %q", cfg.History.IncludeParams)
	}
	for name, p := range cfg.Profiles {
		switch p.AuthType {
		case "", "oauth2", "admin_token":
		default:
			return validationFailure(validationKindProfile, "profile %q has invalid auth_type %q", name, p.AuthType)
		}
		if p.AccountURL == "" {
			return validationFailure(validationKindProfile, "profile %q missing account_url", name)
		}
		if err := ValidateAccountURL(p.AccountURL); err != nil {
			return validationFailure(validationKindProfile, "profile %q has %w", name, err)
		}
		if err := ValidateSecretRef(p.SecretRef); err != nil {
			return validationFailure(validationKindProfile, "profile %q has %w", name, err)
		}
	}
	return nil
}

func validateRateLimit(spec string) error {
	if spec == "" {
		return nil
	}
	parts := strings.Split(spec, "/")
	if len(parts) != 2 || parts[1] != "s" {
		return validationFailure(validationKindRateLimit, "unsupported rate limit %q, expected N/s", spec)
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil || n <= 0 {
		return validationFailure(validationKindRateLimit, "unsupported rate limit %q, expected N/s", spec)
	}
	return nil
}

type validationKind string

const (
	validationKindOutput    validationKind = "output"
	validationKindTimeout   validationKind = "timeout"
	validationKindRateLimit validationKind = "rate_limit"
	validationKindHistory   validationKind = "history"
	validationKindProfile   validationKind = "profile"
)

type validationError struct {
	kind validationKind
	err  error
}

func (e validationError) Error() string { return e.err.Error() }
func (e validationError) Unwrap() error { return e.err }

func validationFailure(kind validationKind, format string, args ...any) error {
	return validationError{kind: kind, err: fmt.Errorf(format, args...)}
}

func validationKindOf(err error) (validationKind, bool) {
	var validationErr validationError
	if errors.As(err, &validationErr) {
		return validationErr.kind, true
	}
	return "", false
}

// ValidOutput reports whether value is a supported default output format.
func ValidOutput(value string) bool {
	switch value {
	case "auto", "json", "yaml", "table", "ndjson", "raw":
		return true
	default:
		return false
	}
}

// IsValidationError reports whether err is a semantic validation failure of
// the merged configuration (config file, env overrides, or flags) — the
// user's input to fix, mapped to the usage/validation exit code by the
// command layer. Read or parse failures are not validation errors.
func IsValidationError(err error) bool {
	_, ok := validationKindOf(err)
	return ok
}
