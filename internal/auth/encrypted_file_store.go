package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pbv7/wsectl/internal/atomicfile"
	"golang.org/x/crypto/argon2"
)

type EncryptedFileStore struct{}

type encryptedPayload struct {
	Version int    `json:"version,omitempty"`
	KDF     string `json:"kdf,omitempty"`
	Salt    string `json:"salt,omitempty"`
	Nonce   string `json:"nonce"`
	Data    string `json:"data"`
}

func (EncryptedFileStore) Get(_ context.Context, ref SecretRef) (SecretBundle, error) {
	pass := os.Getenv("WSECTL_SECRET_PASSPHRASE")
	if pass == "" {
		return SecretBundle{}, fmt.Errorf("WSECTL_SECRET_PASSPHRASE is required for encrypted-file secrets")
	}
	raw, err := os.ReadFile(ref.Name)
	if err != nil {
		return SecretBundle{}, err
	}
	var payload encryptedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return SecretBundle{}, err
	}
	nonce, err := base64.StdEncoding.DecodeString(payload.Nonce)
	if err != nil {
		return SecretBundle{}, err
	}
	data, err := base64.StdEncoding.DecodeString(payload.Data)
	if err != nil {
		return SecretBundle{}, err
	}
	aead, err := payloadAEAD(pass, payload)
	if err != nil {
		return SecretBundle{}, err
	}
	plain, err := aead.Open(nil, nonce, data, nil)
	if err != nil {
		return SecretBundle{}, err
	}
	var b SecretBundle
	return b, json.Unmarshal(plain, &b)
}

func (EncryptedFileStore) Set(_ context.Context, ref SecretRef, value SecretBundle) error {
	pass := os.Getenv("WSECTL_SECRET_PASSPHRASE")
	if pass == "" {
		return fmt.Errorf("WSECTL_SECRET_PASSPHRASE is required for encrypted-file secrets")
	}
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}
	aead, err := argon2AEAD(pass, salt)
	if err != nil {
		return err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	plain, err := json.Marshal(value)
	if err != nil {
		return err
	}
	payload := encryptedPayload{
		Version: 2,
		KDF:     "argon2id",
		Salt:    base64.StdEncoding.EncodeToString(salt),
		Nonce:   base64.StdEncoding.EncodeToString(nonce),
		Data:    base64.StdEncoding.EncodeToString(aead.Seal(nil, nonce, plain, nil)),
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(ref.Name, raw, 0o600)
}

func (EncryptedFileStore) Delete(_ context.Context, ref SecretRef) error {
	return os.Remove(ref.Name)
}

func (EncryptedFileStore) CheckWritable(_ context.Context, ref SecretRef) error {
	if os.Getenv("WSECTL_SECRET_PASSPHRASE") == "" {
		return fmt.Errorf("WSECTL_SECRET_PASSPHRASE is required for encrypted-file secrets")
	}
	probe := filepath.Join(filepath.Dir(ref.Name), ".wsectl-write-check")
	if err := atomicfile.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return err
	}
	return os.Remove(probe)
}

func payloadAEAD(pass string, payload encryptedPayload) (cipher.AEAD, error) {
	if payload.Version == 2 {
		if payload.KDF != "argon2id" {
			return nil, fmt.Errorf("unsupported encrypted-file KDF %q", payload.KDF)
		}
		salt, err := base64.StdEncoding.DecodeString(payload.Salt)
		if err != nil {
			return nil, err
		}
		return argon2AEAD(pass, salt)
	}
	return legacyAEAD(pass)
}

func argon2AEAD(pass string, salt []byte) (cipher.AEAD, error) {
	key := argon2.IDKey([]byte(pass), salt, 3, 64*1024, 4, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func legacyAEAD(pass string) (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(pass))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
