package auth

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pbv7/wsectl/internal/atomicfile"
)

type PlaintextStore struct{}

func (PlaintextStore) Get(_ context.Context, ref SecretRef) (SecretBundle, error) {
	raw, err := os.ReadFile(ref.Name)
	if err != nil {
		return SecretBundle{}, err
	}
	var b SecretBundle
	return b, json.Unmarshal(raw, &b)
}

func (PlaintextStore) Set(_ context.Context, ref SecretRef, value SecretBundle) error {
	if err := os.MkdirAll(filepath.Dir(ref.Name), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(ref.Name, raw, 0o600)
}

func (PlaintextStore) Delete(_ context.Context, ref SecretRef) error {
	return os.Remove(ref.Name)
}

func (PlaintextStore) CheckWritable(_ context.Context, ref SecretRef) error {
	if err := os.MkdirAll(filepath.Dir(ref.Name), 0o700); err != nil {
		return err
	}
	probe := filepath.Join(filepath.Dir(ref.Name), ".wsectl-write-check")
	if err := atomicfile.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return err
	}
	return os.Remove(probe)
}
