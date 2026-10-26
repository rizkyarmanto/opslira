package credentials

import (
	"encoding/base64"
	"testing"
)

// TestEncryptDecryptRoundTrip proves the three-column layout round-trips.
func TestEncryptDecryptRoundTrip(t *testing.T) {
	svc, err := New("", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	payload := map[string]any{
		"username": "svc-opslira",
		"password": "p@ss word",
		"nested":   map[string]any{"community": "public"},
	}

	enc, err := svc.Encrypt(payload)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	for name, value := range map[string]string{
		"encrypted_payload": enc.Payload,
		"encryption_iv":     enc.IV,
		"encryption_tag":    enc.Tag,
	} {
		if value == "" {
			t.Errorf("%s is empty", name)
		}
		if _, err := base64.StdEncoding.DecodeString(value); err != nil {
			t.Errorf("%s is not base64: %v", name, err)
		}
	}

	dec, err := svc.Decrypt(enc.Payload, enc.IV, enc.Tag)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if dec["username"] != "svc-opslira" || dec["password"] != "p@ss word" {
		t.Errorf("round trip mismatch: %#v", dec)
	}
	nested, ok := dec["nested"].(map[string]any)
	if !ok || nested["community"] != "public" {
		t.Errorf("nested map mismatch: %#v", dec["nested"])
	}
}

// TestKeyDerivationMatchesPHP pins the PHP compat rules: APP_ENCRYPTION_KEY
// wins, APP_KEY strips the base64: prefix and decodes, empty falls back to
// SHA-256("").
func TestKeyDerivationMatchesPHP(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("php-style-key"))

	fromAppKey, err := New("", "base64:"+encoded)
	if err != nil {
		t.Fatalf("New with base64 APP_KEY: %v", err)
	}
	plain, err := New("", "base64:"+encoded)
	if err != nil {
		t.Fatal(err)
	}

	// A ciphertext sealed with the base64-decoded key must open with the
	// same derived key, and a dedicated encryption key must win over APP_KEY.
	enc, err := plain.Encrypt(map[string]any{"k": "v"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fromAppKey.Decrypt(enc.Payload, enc.IV, enc.Tag); err != nil {
		t.Errorf("base64 APP_KEY derivation mismatch: %v", err)
	}

	dedicated, err := New("other-key", "base64:"+encoded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dedicated.Decrypt(enc.Payload, enc.IV, enc.Tag); err == nil {
		t.Error("dedicated APP_ENCRYPTION_KEY must not open an APP_KEY ciphertext")
	}
}

// TestDecryptTamperFails: GCM auth must reject a flipped ciphertext byte.
func TestDecryptTamperFails(t *testing.T) {
	svc, err := New("k", "")
	if err != nil {
		t.Fatal(err)
	}
	enc, err := svc.Encrypt(map[string]any{"password": "secret"})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := base64.StdEncoding.DecodeString(enc.Payload)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] ^= 0xFF
	tampered := base64.StdEncoding.EncodeToString(raw)

	if _, err := svc.Decrypt(tampered, enc.IV, enc.Tag); err == nil {
		t.Error("tampered ciphertext decrypted without error")
	}
}
