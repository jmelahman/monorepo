// Package secrets provides AES-256-GCM encryption for sensitive values
// persisted in the local database (e.g. per-board environment variables),
// plus load-or-create handling for the key file that lives next to the DB.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

// KeySize is the AES-256 key length in bytes.
const KeySize = 32

// v1Prefix versions the ciphertext format (base64(nonce||ciphertext)) so a
// future rotation or format change can coexist with existing rows.
const v1Prefix = "v1:"

// ErrNoCipher is returned by callers that require a configured Box.
var ErrNoCipher = errors.New("secrets: no cipher configured")

// Box encrypts and decrypts short string values with AES-256-GCM.
type Box struct {
	aead cipher.AEAD
}

// NewBox builds a Box from a KeySize-byte key.
func NewBox(key []byte) (*Box, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("secrets: key must be %d bytes, got %d", KeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secrets: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets: %w", err)
	}
	return &Box{aead: aead}, nil
}

// Encrypt seals plaintext and returns "v1:" + base64(nonce||ciphertext).
func (b *Box) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("secrets: nonce: %w", err)
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return v1Prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. It fails if the value was sealed with a different
// key or the ciphertext was tampered with.
func (b *Box) Decrypt(value string) (string, error) {
	raw, ok := strings.CutPrefix(value, v1Prefix)
	if !ok {
		return "", fmt.Errorf("secrets: unknown ciphertext format")
	}
	sealed, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", fmt.Errorf("secrets: decode: %w", err)
	}
	if len(sealed) < b.aead.NonceSize() {
		return "", fmt.Errorf("secrets: ciphertext too short")
	}
	nonce, ct := sealed[:b.aead.NonceSize()], sealed[b.aead.NonceSize():]
	plaintext, err := b.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("secrets: decrypt: %w", err)
	}
	return string(plaintext), nil
}

// NewRandomKey returns a fresh KeySize-byte key. Used for --in-memory mode,
// where persisting a key file would violate the "no on-disk state" promise.
func NewRandomKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("secrets: generate key: %w", err)
	}
	return key, nil
}

// LoadOrCreateKey reads a hex-encoded KeySize-byte key from path, generating
// and persisting one (mode 0600) on first use.
func LoadOrCreateKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		key, decErr := hex.DecodeString(strings.TrimSpace(string(data)))
		if decErr != nil || len(key) != KeySize {
			return nil, fmt.Errorf("secrets: %s is not a valid %d-byte hex key", path, KeySize)
		}
		return key, nil
	case os.IsNotExist(err):
		key, genErr := NewRandomKey()
		if genErr != nil {
			return nil, genErr
		}
		if writeErr := os.WriteFile(path, []byte(hex.EncodeToString(key)+"\n"), 0o600); writeErr != nil {
			return nil, fmt.Errorf("secrets: write key file: %w", writeErr)
		}
		return key, nil
	default:
		return nil, fmt.Errorf("secrets: read key file: %w", err)
	}
}
