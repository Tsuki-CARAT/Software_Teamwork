package secrets

import (
	"bytes"
	"testing"
)

const testKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestCipherRoundTripUsesUniqueNonce(t *testing.T) {
	cipher, err := New(testKey)
	if err != nil {
		t.Fatal(err)
	}
	first, err := cipher.Encrypt("secret-token")
	if err != nil {
		t.Fatal(err)
	}
	second, err := cipher.Encrypt("secret-token")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("AES-GCM ciphertext reused a nonce")
	}
	got, err := cipher.Decrypt(first)
	if err != nil {
		t.Fatal(err)
	}
	if got != "secret-token" || Last4(got) != "oken" {
		t.Fatalf("unexpected round trip: value=%q last4=%q", got, Last4(got))
	}
}

func TestNewRejectsInvalidKey(t *testing.T) {
	if _, err := New("short"); err == nil {
		t.Fatal("expected invalid key to fail")
	}
}
