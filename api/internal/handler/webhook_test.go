package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestValidGitHubSignature(t *testing.T) {
	secret, body := []byte("s3cret"), []byte(`{"a":1}`)
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	good := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !validGitHubSignature(secret, body, good) {
		t.Fatal("valid signature rejected")
	}
	for _, bad := range []string{"", "sha256=", "sha1=" + good[7:], "sha256=zz", good[:len(good)-2] + "00"} {
		if validGitHubSignature(secret, body, bad) {
			t.Errorf("accepted bad signature %q", bad)
		}
	}
	if validGitHubSignature([]byte("other"), body, good) {
		t.Error("accepted signature made with another secret")
	}
}
