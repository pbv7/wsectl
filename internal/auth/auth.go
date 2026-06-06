package auth

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type SecretRef struct {
	Scheme string
	Name   string
}

// SecretBundle stores every credential type wsectl can use without exposing
// those secrets through command output.
type SecretBundle struct {
	ClientID      string    `json:"client_id,omitempty"`
	ClientSecret  string    `json:"client_secret,omitempty"`
	AccessToken   string    `json:"access_token,omitempty"`
	RefreshToken  string    `json:"refresh_token,omitempty"`
	AccessExpires time.Time `json:"access_expires,omitempty"`
	AccountURL    string    `json:"account_url,omitempty"`
	AdminToken    string    `json:"admin_token,omitempty"`
}

// SecretStore abstracts OS keychains, environment variables, and explicit file
// stores behind one credential API.
type SecretStore interface {
	Get(ctx context.Context, ref SecretRef) (SecretBundle, error)
	Set(ctx context.Context, ref SecretRef, value SecretBundle) error
	Delete(ctx context.Context, ref SecretRef) error
}

// WritableStore can validate that a secret backend is writable before a login
// flow opens the browser.
type WritableStore interface {
	CheckWritable(ctx context.Context, ref SecretRef) error
}

// CheckWritable reports whether a store can persist secrets.
func CheckWritable(ctx context.Context, store SecretStore, ref SecretRef) error {
	if checker, ok := store.(WritableStore); ok {
		return checker.CheckWritable(ctx, ref)
	}
	return nil
}

// ParseRef parses a secret reference such as keyring:wsectl/default.
func ParseRef(s string) (SecretRef, error) {
	if s == "" {
		return SecretRef{Scheme: "keyring", Name: "wsectl/default"}, nil
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return SecretRef{}, fmt.Errorf("invalid secret ref %q", s)
	}
	return SecretRef{Scheme: parts[0], Name: parts[1]}, nil
}

// StoreFor returns the concrete secret backend named by a SecretRef.
func StoreFor(ref SecretRef) (SecretStore, error) {
	switch ref.Scheme {
	case "keyring":
		return NewKeyringStore(), nil
	case "env":
		return EnvStore{}, nil
	case "encrypted-file":
		return EncryptedFileStore{}, nil
	case "plaintext":
		return PlaintextStore{}, nil
	default:
		return nil, fmt.Errorf("unsupported secret store %q", ref.Scheme)
	}
}
