package security

import "testing"

func TestSecretBoxRoundTripAndRandomNonce(t *testing.T) {
	box, err := NewSecretBox("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if err != nil {
		t.Fatalf("NewSecretBox() error = %v", err)
	}
	first, err := box.Encrypt("client-secret")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	second, err := box.Encrypt("client-secret")
	if err != nil {
		t.Fatalf("Encrypt() second error = %v", err)
	}
	if first == second {
		t.Fatal("encryption must use a random nonce")
	}
	plaintext, err := box.Decrypt(first)
	if err != nil || plaintext != "client-secret" {
		t.Fatalf("Decrypt() = %q, %v", plaintext, err)
	}
}

func TestSecretBoxRejectsInvalidKey(t *testing.T) {
	if _, err := NewSecretBox("c2hvcnQ="); err == nil {
		t.Fatal("NewSecretBox() accepted a short key")
	}
}
