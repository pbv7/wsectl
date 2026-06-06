package testutil

import (
	"context"
	"fmt"

	"github.com/pbv7/wsectl/internal/auth"
)

type FakeSecretStore struct {
	Items map[string]auth.SecretBundle
}

func (s *FakeSecretStore) Get(_ context.Context, ref auth.SecretRef) (auth.SecretBundle, error) {
	if s.Items == nil {
		s.Items = map[string]auth.SecretBundle{}
	}
	item, ok := s.Items[ref.Name]
	if !ok {
		return auth.SecretBundle{}, fmt.Errorf("secret %q not found", ref.Name)
	}
	return item, nil
}

func (s *FakeSecretStore) Set(_ context.Context, ref auth.SecretRef, value auth.SecretBundle) error {
	if s.Items == nil {
		s.Items = map[string]auth.SecretBundle{}
	}
	s.Items[ref.Name] = value
	return nil
}

func (s *FakeSecretStore) Delete(_ context.Context, ref auth.SecretRef) error {
	delete(s.Items, ref.Name)
	return nil
}
