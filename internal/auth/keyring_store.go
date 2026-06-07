package auth

import (
	"context"
	"encoding/json"

	"github.com/99designs/keyring"
)

type KeyringStore struct{}

// NewKeyringStore returns the OS keychain backed secret store.
func NewKeyringStore() KeyringStore { return KeyringStore{} }

func (KeyringStore) ring() (keyring.Keyring, error) {
	return keyring.Open(keyringConfig())
}

func keyringConfig() keyring.Config {
	return keyring.Config{
		ServiceName:              "wsectl",
		KeychainTrustApplication: true,
		AllowedBackends: []keyring.BackendType{
			keyring.KeychainBackend,
			keyring.WinCredBackend,
			keyring.SecretServiceBackend,
			keyring.KWalletBackend,
			keyring.PassBackend,
		},
	}
}

func (s KeyringStore) Get(_ context.Context, ref SecretRef) (SecretBundle, error) {
	r, err := s.ring()
	if err != nil {
		return SecretBundle{}, err
	}
	item, err := r.Get(ref.Name)
	if err != nil {
		return SecretBundle{}, err
	}
	var b SecretBundle
	return b, json.Unmarshal(item.Data, &b)
}

func (s KeyringStore) Set(_ context.Context, ref SecretRef, value SecretBundle) error {
	r, err := s.ring()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.Set(keyring.Item{Key: ref.Name, Data: raw})
}

func (s KeyringStore) Delete(_ context.Context, ref SecretRef) error {
	r, err := s.ring()
	if err != nil {
		return err
	}
	return r.Remove(ref.Name)
}

func (s KeyringStore) CheckWritable(context.Context, SecretRef) error {
	r, err := s.ring()
	if err != nil {
		return err
	}
	probe := "wsectl/.write-check"
	if err := r.Set(keyring.Item{Key: probe, Data: []byte("ok")}); err != nil {
		return err
	}
	return r.Remove(probe)
}
