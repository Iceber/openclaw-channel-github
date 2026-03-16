package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"testing"
	"time"
)

func computeTestSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyWebhookSignature(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		secret    string
		signature string
		valid     bool
	}{
		{
			name:      "valid signature",
			payload:   `{"action":"opened"}`,
			secret:    "test-secret",
			signature: "", // will be computed below
			valid:     true,
		},
		{
			name:      "invalid signature",
			payload:   `{"action":"opened"}`,
			secret:    "test-secret",
			signature: "sha256=0000000000000000000000000000000000000000000000000000000000000000",
			valid:     false,
		},
		{
			name:      "empty signature",
			payload:   `{"action":"opened"}`,
			secret:    "test-secret",
			signature: "",
			valid:     false,
		},
		{
			name:      "empty secret",
			payload:   `{"action":"opened"}`,
			secret:    "",
			signature: "sha256=abc",
			valid:     false,
		},
		{
			name:      "missing sha256 prefix",
			payload:   `{"action":"opened"}`,
			secret:    "test-secret",
			signature: "abc123",
			valid:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := []byte(tc.payload)

			sig := tc.signature
			if tc.name == "valid signature" {
				sig = computeTestSignature(payload, tc.secret)
			}

			got := VerifyWebhookSignature(payload, tc.secret, sig)
			if got != tc.valid {
				t.Errorf("VerifyWebhookSignature() = %v, want %v", got, tc.valid)
			}
		})
	}
}

func TestGenerateJWT(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	a := NewGitHubAuth(12345, keyPEM)
	token, err := a.GenerateJWT()
	if err != nil {
		t.Fatalf("GenerateJWT() error: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty JWT token")
	}
}

func TestGenerateJWT_InvalidKey(t *testing.T) {
	a := NewGitHubAuth(12345, []byte("not-a-pem-key"))
	_, err := a.GenerateJWT()
	if err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestInstallationTokenCaching(t *testing.T) {
	a := NewGitHubAuth(12345, nil)

	// Initially no token
	if got := a.GetInstallationToken(); got != "" {
		t.Errorf("expected empty token, got %q", got)
	}

	// Set a token with future expiry
	a.SetInstallationToken("test-token", time.Now().Add(time.Hour))
	if got := a.GetInstallationToken(); got != "test-token" {
		t.Errorf("expected 'test-token', got %q", got)
	}

	// Set a token with past expiry
	a.SetInstallationToken("expired", time.Now().Add(-time.Hour))
	if got := a.GetInstallationToken(); got != "" {
		t.Errorf("expected empty token for expired, got %q", got)
	}
}
