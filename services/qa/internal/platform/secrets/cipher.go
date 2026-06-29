package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Cipher struct {
	aead cipher.AEAD
}

func New(hexKey string) (*Cipher, error) {
	hexKey = strings.TrimSpace(hexKey)
	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 {
		return nil, errors.New("QA_CONFIG_ENCRYPTION_KEY must be exactly 32 bytes encoded as 64 hex characters")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM cipher: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) Encrypt(plaintext string) ([]byte, error) {
	if c == nil || c.aead == nil {
		return nil, errors.New("secret cipher is not configured")
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate encryption nonce: %w", err)
	}
	return c.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func (c *Cipher) Decrypt(ciphertext []byte) (string, error) {
	if c == nil || c.aead == nil {
		return "", errors.New("secret cipher is not configured")
	}
	if len(ciphertext) < c.aead.NonceSize() {
		return "", errors.New("encrypted secret is invalid")
	}
	nonce, payload := ciphertext[:c.aead.NonceSize()], ciphertext[c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", errors.New("encrypted secret cannot be decrypted")
	}
	return string(plaintext), nil
}

func Last4(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= 4 {
		return string(runes)
	}
	return string(runes[len(runes)-4:])
}
