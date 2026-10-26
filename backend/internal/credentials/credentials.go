// Package credentials encrypts target credential payloads at rest with
// AES-256-GCM, storing ciphertext, IV and auth tag as three separate
// base64 columns — byte-compatible with the Laravel
// CredentialEncryptionService.
package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// Encrypted mirrors target_credentials: three base64 strings stored in
// separate columns.
type Encrypted struct {
	Payload string
	IV      string
	Tag     string
}

// Service encrypts and decrypts credential payloads with the derived key.
type Service struct {
	aead cipher.AEAD
}

// New derives the AES-256-GCM key exactly like the PHP service:
//
//	rawKey = APP_ENCRYPTION_KEY when set, otherwise APP_KEY with an optional
//	"base64:" prefix stripped and the remainder base64-decoded;
//	key    = SHA-256(rawKey).
//
// With both variables empty the key is SHA-256(""), which keeps local dev
// working the same way Laravel did; production must set APP_ENCRYPTION_KEY.
func New(appEncryptionKey, appKey string) (*Service, error) {
	raw := strings.TrimSpace(appEncryptionKey)
	if raw == "" {
		raw = strings.TrimSpace(appKey)
		if strings.HasPrefix(raw, "base64:") {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(raw, "base64:"))
			if err != nil {
				return nil, fmt.Errorf("APP_KEY base64 decode: %w", err)
			}
			raw = string(decoded)
		}
	}

	sum := sha256.Sum256([]byte(raw))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &Service{aead: aead}, nil
}

// Encrypt seals the payload JSON and returns base64 ciphertext, IV and tag.
func (s *Service) Encrypt(payload map[string]any) (Encrypted, error) {
	plain, err := json.Marshal(payload)
	if err != nil {
		return Encrypted{}, fmt.Errorf("credential json: %w", err)
	}

	iv := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return Encrypted{}, err
	}

	sealed := s.aead.Seal(nil, iv, plain, nil)
	tag := sealed[len(sealed)-s.aead.Overhead():]
	ciphertext := sealed[:len(sealed)-s.aead.Overhead()]

	return Encrypted{
		Payload: base64.StdEncoding.EncodeToString(ciphertext),
		IV:      base64.StdEncoding.EncodeToString(iv),
		Tag:     base64.StdEncoding.EncodeToString(tag),
	}, nil
}

// Decrypt opens the three stored base64 values back into the payload map.
func (s *Service) Decrypt(payload, iv, tag string) (map[string]any, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("credential payload base64: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(iv)
	if err != nil {
		return nil, fmt.Errorf("credential iv base64: %w", err)
	}
	authTag, err := base64.StdEncoding.DecodeString(tag)
	if err != nil {
		return nil, fmt.Errorf("credential tag base64: %w", err)
	}

	sealed := append(append([]byte{}, ciphertext...), authTag...)
	plain, err := s.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("credential decryption failed: %w", err)
	}

	var out map[string]any
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, fmt.Errorf("credential json: %w", err)
	}

	return out, nil
}
