package github

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestNewApp(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	c, err := NewApp(1, 2, pemKey, "acme")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if !c.org || c.Owner() != "acme" {
		t.Fatalf("app clients must create repos under the org owner: %+v", c)
	}

	if _, err := NewApp(1, 2, []byte("not a key"), "acme"); err == nil {
		t.Fatal("expected error for invalid private key")
	}
}
