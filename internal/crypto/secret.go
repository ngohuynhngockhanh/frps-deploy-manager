package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Sealer encrypts/decrypts small secrets (frp tokens, dashboard passwords)
// at rest using AES-256-GCM. Sealed blobs are nonce-prefixed: nonce||ciphertext.
type Sealer struct {
	aead cipher.AEAD
}

// LoadOrCreateSealer loads a 32-byte key from keyPath, creating it (mode 0600)
// with fresh random bytes if it does not exist.
func LoadOrCreateSealer(keyPath string) (*Sealer, error) {
	key, err := os.ReadFile(keyPath)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(keyPath, key, 0o600); err != nil {
			return nil, fmt.Errorf("write master key: %w", err)
		}
	} else if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead}, nil
}

// Seal returns nonce||ciphertext for plaintext.
func (s *Sealer) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return s.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Open decrypts a nonce||ciphertext blob produced by Seal.
func (s *Sealer) Open(blob []byte) ([]byte, error) {
	ns := s.aead.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("sealed blob too short")
	}
	nonce, ct := blob[:ns], blob[ns:]
	return s.aead.Open(nil, nonce, ct, nil)
}
