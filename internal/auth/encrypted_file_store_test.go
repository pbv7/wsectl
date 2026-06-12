package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEncryptedSecret(t *testing.T, ref SecretRef, pass string) {
	t.Helper()
	t.Setenv("WSECTL_SECRET_PASSPHRASE", pass)
	value := SecretBundle{ClientID: "client_123", AccessToken: "access_123", RefreshToken: "refresh_123"}
	if err := (EncryptedFileStore{}).Set(context.Background(), ref, value); err != nil {
		t.Fatal(err)
	}
}

func readEncryptedPayload(t *testing.T, path string) encryptedPayload {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var payload encryptedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func writeEncryptedPayload(t *testing.T, path string, payload encryptedPayload) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func flipFirstByte(t *testing.T, encoded string) string {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] ^= 0xff
	return base64.StdEncoding.EncodeToString(raw)
}

func requireGetFails(t *testing.T, ref SecretRef) {
	t.Helper()
	got, err := (EncryptedFileStore{}).Get(context.Background(), ref)
	if err == nil {
		t.Fatalf("Get succeeded, want authentication failure; got %#v", got)
	}
	if got != (SecretBundle{}) {
		t.Fatalf("failed Get must not return secret material, got %#v", got)
	}
}

func TestEncryptedFileStoreSetCreatesMissingParents(t *testing.T) {
	ref := SecretRef{Scheme: "encrypted-file", Name: filepath.Join(t.TempDir(), "a", "b", "secret.json")}
	writeEncryptedSecret(t, ref, "passphrase")
	got, err := (EncryptedFileStore{}).Get(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "access_123" {
		t.Fatalf("round-trip through nested path failed, got %#v", got)
	}
}

func TestPlaintextStoreSetCreatesMissingParents(t *testing.T) {
	ref := SecretRef{Scheme: "plaintext", Name: filepath.Join(t.TempDir(), "a", "b", "secret.json")}
	if err := (PlaintextStore{}).Set(context.Background(), ref, SecretBundle{AccessToken: "access_123"}); err != nil {
		t.Fatal(err)
	}
	got, err := (PlaintextStore{}).Get(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "access_123" {
		t.Fatalf("round-trip through nested path failed, got %#v", got)
	}
}

func TestEncryptedFileStoreRejectsWrongPassphrase(t *testing.T) {
	ref := SecretRef{Scheme: "encrypted-file", Name: filepath.Join(t.TempDir(), "secret.json")}
	writeEncryptedSecret(t, ref, "correct-passphrase")
	t.Setenv("WSECTL_SECRET_PASSPHRASE", "wrong-passphrase")
	requireGetFails(t, ref)
}

func TestEncryptedFileStoreRejectsTamperedCiphertext(t *testing.T) {
	ref := SecretRef{Scheme: "encrypted-file", Name: filepath.Join(t.TempDir(), "secret.json")}
	writeEncryptedSecret(t, ref, "passphrase")
	payload := readEncryptedPayload(t, ref.Name)
	payload.Data = flipFirstByte(t, payload.Data)
	writeEncryptedPayload(t, ref.Name, payload)
	requireGetFails(t, ref)
}

func TestEncryptedFileStoreRejectsTamperedNonce(t *testing.T) {
	ref := SecretRef{Scheme: "encrypted-file", Name: filepath.Join(t.TempDir(), "secret.json")}
	writeEncryptedSecret(t, ref, "passphrase")
	payload := readEncryptedPayload(t, ref.Name)
	payload.Nonce = flipFirstByte(t, payload.Nonce)
	writeEncryptedPayload(t, ref.Name, payload)
	requireGetFails(t, ref)
}

func TestEncryptedFileStoreRejectsTamperedSalt(t *testing.T) {
	ref := SecretRef{Scheme: "encrypted-file", Name: filepath.Join(t.TempDir(), "secret.json")}
	writeEncryptedSecret(t, ref, "passphrase")
	payload := readEncryptedPayload(t, ref.Name)
	payload.Salt = flipFirstByte(t, payload.Salt)
	writeEncryptedPayload(t, ref.Name, payload)
	requireGetFails(t, ref)
}

func TestEncryptedFileStoreRejectsUnsupportedKDF(t *testing.T) {
	ref := SecretRef{Scheme: "encrypted-file", Name: filepath.Join(t.TempDir(), "secret.json")}
	writeEncryptedSecret(t, ref, "passphrase")
	payload := readEncryptedPayload(t, ref.Name)
	payload.KDF = "scrypt"
	writeEncryptedPayload(t, ref.Name, payload)
	got, err := (EncryptedFileStore{}).Get(context.Background(), ref)
	if err == nil || !strings.Contains(err.Error(), "unsupported encrypted-file KDF") {
		t.Fatalf("err = %v (bundle %#v), want unsupported-KDF error", err, got)
	}
}

func TestEncryptedFileStoreLegacyPayloadRejectsWrongPassphrase(t *testing.T) {
	ref := SecretRef{Scheme: "encrypted-file", Name: filepath.Join(t.TempDir(), "legacy.json")}
	aead, err := legacyAEAD("correct-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	nonce := []byte("123456789012")
	plain, err := json.Marshal(SecretBundle{AccessToken: "legacy-access"})
	if err != nil {
		t.Fatal(err)
	}
	writeEncryptedPayload(t, ref.Name, encryptedPayload{
		Nonce: base64.StdEncoding.EncodeToString(nonce),
		Data:  base64.StdEncoding.EncodeToString(aead.Seal(nil, nonce, plain, nil)),
	})
	t.Setenv("WSECTL_SECRET_PASSPHRASE", "wrong-passphrase")
	requireGetFails(t, ref)
}
