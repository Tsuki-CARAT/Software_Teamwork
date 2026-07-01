package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

const StorageModeEncryptedColumn = "encrypted_column"

var credentialFingerprintContext = []byte("ai-gateway credential fingerprint v1")

type CredentialEncryptor struct {
	aead           cipher.AEAD
	fingerprintKey []byte
	keyVersion     string
}

func NewCredentialEncryptor(key []byte, keyVersion string) (*CredentialEncryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("credential encryption key must be 32 bytes")
	}
	if strings.TrimSpace(keyVersion) == "" {
		return nil, fmt.Errorf("credential encryption key version is required")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	fingerprintKey := hmacSHA256(key, credentialFingerprintContext)
	return &CredentialEncryptor{aead: aead, fingerprintKey: fingerprintKey, keyVersion: strings.TrimSpace(keyVersion)}, nil
}

func (e *CredentialEncryptor) Encrypt(apiKey string) (ProviderCredential, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return ProviderCredential{}, fmt.Errorf("api key is required")
	}
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return ProviderCredential{}, err
	}
	ciphertext := e.aead.Seal(nil, nonce, []byte(apiKey), nil)
	fingerprint := hmacSHA256(e.fingerprintKey, []byte(apiKey))
	return ProviderCredential{
		StorageMode:          StorageModeEncryptedColumn,
		Ciphertext:           ciphertext,
		Nonce:                nonce,
		EncryptionKeyVersion: e.keyVersion,
		FingerprintSHA256:    hex.EncodeToString(fingerprint),
		KeyLast4:             last4(apiKey),
		Status:               CredentialActive,
	}, nil
}

func (e *CredentialEncryptor) Decrypt(credential ProviderCredential) (string, error) {
	if credential.StorageMode != StorageModeEncryptedColumn {
		return "", fmt.Errorf("unsupported credential storage mode")
	}
	plain, err := e.aead.Open(nil, credential.Nonce, credential.Ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func hmacSHA256(key, message []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(message)
	return mac.Sum(nil)
}

func last4(value string) string {
	if len(value) <= 4 {
		return value
	}
	return value[len(value)-4:]
}
