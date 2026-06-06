package auth

import (
	"context"
	"fmt"
	"os"
)

type EnvStore struct{}

// Get reads credentials from WSECTL_* environment variables.
func (EnvStore) Get(context.Context, SecretRef) (SecretBundle, error) {
	b := SecretBundle{
		ClientID:     os.Getenv("WSECTL_CLIENT_ID"),
		ClientSecret: os.Getenv("WSECTL_CLIENT_SECRET"),
		AccessToken:  os.Getenv("WSECTL_ACCESS_TOKEN"),
		RefreshToken: os.Getenv("WSECTL_REFRESH_TOKEN"),
		AdminToken:   os.Getenv("WSECTL_ADMIN_TOKEN"),
		AccountURL:   os.Getenv("WSECTL_ACCOUNT_URL"),
	}
	if b.AccessToken == "" && b.AdminToken == "" && b.ClientID == "" {
		return b, fmt.Errorf("no WSECTL credentials found in environment")
	}
	return b, nil
}

// Set always fails because environment credentials are read-only.
func (EnvStore) Set(context.Context, SecretRef, SecretBundle) error {
	return fmt.Errorf("env secret store is read-only")
}

// Delete always fails because environment credentials are read-only.
func (EnvStore) Delete(context.Context, SecretRef) error {
	return fmt.Errorf("env secret store is read-only")
}

func (EnvStore) CheckWritable(context.Context, SecretRef) error {
	return fmt.Errorf("env secret store is read-only; use keyring, encrypted-file, or plaintext for auth login")
}
