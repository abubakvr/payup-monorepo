package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// FieldHash returns SHA-256 hex of lower(trim(plaintext)). Matches DB field_hash() for indexed lookups.
func FieldHash(plaintext string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(plaintext))))
	return hex.EncodeToString(h[:])
}

// Encrypt encrypts plaintext with AES-GCM using keyHex (64 hex chars = 32 bytes). Returns ciphertext (nonce+tag+body).
func Encrypt(plaintext []byte, keyHex string) ([]byte, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: must be 64 hex characters (0-9, a-f); generate with: openssl rand -hex 32")
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid encryption key: must be 64 hex chars (32 bytes), got %d chars (%d bytes); use: openssl rand -hex 32", len(keyHex), len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts ciphertext (nonce+tag+body) produced by Encrypt.
func Decrypt(ciphertext []byte, keyHex string) ([]byte, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: must be 64 hex characters (0-9, a-f); generate with: openssl rand -hex 32")
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid encryption key: must be 64 hex chars (32 bytes), got %d chars (%d bytes); use: openssl rand -hex 32", len(keyHex), len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, body := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, body, nil)
}
