package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const encryptedPrefix = "enc:v1:"

// SecretBox encrypts the values that must not sit in the database in clear: client secrets, storage keys and webhook secrets.
type SecretBox struct {
	aead cipher.AEAD
}

// NewSecretBox builds an AES-256-GCM box from a base64 master key, which must decode to exactly 32 bytes.
func NewSecretBox(base64Key string) (*SecretBox, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("decode config master key: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("config master key must decode to exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SecretBox{aead: aead}, nil
}

// Encrypt seals a value and prefixes it with the format marker, so a plaintext value written by an older version is still recognisable. An empty value stays empty.
func (b *SecretBox) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := b.aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, ciphertext...)
	return encryptedPrefix + base64.RawStdEncoding.EncodeToString(payload), nil
}

// Decrypt opens a value produced by Encrypt, rejecting anything without the expected prefix or with a truncated payload.
func (b *SecretBox) Decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if len(value) <= len(encryptedPrefix) || value[:len(encryptedPrefix)] != encryptedPrefix {
		return "", errors.New("secret value has an unsupported format")
	}
	payload, err := base64.RawStdEncoding.DecodeString(value[len(encryptedPrefix):])
	if err != nil {
		return "", err
	}
	nonceSize := b.aead.NonceSize()
	if len(payload) <= nonceSize {
		return "", errors.New("encrypted secret is truncated")
	}
	plaintext, err := b.aead.Open(nil, payload[:nonceSize], payload[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// MaskSecret replaces a non-empty secret with asterisks for display.
func MaskSecret(value string) string {
	if value == "" {
		return ""
	}
	return "********"
}
