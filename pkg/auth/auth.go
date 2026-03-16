// Package auth provides GitHub App authentication and webhook signature verification.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GitHubAuth manages authentication for GitHub App interactions.
type GitHubAuth struct {
	appID      int64
	privateKey []byte

	mu               sync.Mutex
	installationToken string
	tokenExpiry       time.Time
}

// NewGitHubAuth creates a new GitHubAuth with the given App ID and private key PEM data.
func NewGitHubAuth(appID int64, privateKeyPEM []byte) *GitHubAuth {
	return &GitHubAuth{
		appID:      appID,
		privateKey: privateKeyPEM,
	}
}

// GenerateJWT creates a short-lived JWT for GitHub App authentication.
// The token is valid for 10 minutes (GitHub's maximum).
func (a *GitHubAuth) GenerateJWT() (string, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(a.privateKey)
	if err != nil {
		return "", fmt.Errorf("parsing private key: %w", err)
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		Issuer:    fmt.Sprintf("%d", a.appID),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}

	return signed, nil
}

// SetInstallationToken caches an installation token with its expiry time.
func (a *GitHubAuth) SetInstallationToken(token string, expiry time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.installationToken = token
	a.tokenExpiry = expiry
}

// GetInstallationToken returns the cached installation token if still valid.
// Returns empty string if the token is expired or not set.
func (a *GitHubAuth) GetInstallationToken() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.installationToken == "" || time.Now().After(a.tokenExpiry.Add(-time.Minute)) {
		return ""
	}
	return a.installationToken
}

// VerifyWebhookSignature verifies the X-Hub-Signature-256 header against the payload.
func VerifyWebhookSignature(payload []byte, secret string, signature string) bool {
	if signature == "" || secret == "" {
		return false
	}

	sig := strings.TrimPrefix(signature, "sha256=")
	if sig == signature {
		// No "sha256=" prefix found
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sig), []byte(expected))
}

// ExtractSignature extracts the X-Hub-Signature-256 header from an HTTP request.
func ExtractSignature(r *http.Request) string {
	return r.Header.Get("X-Hub-Signature-256")
}

// ExtractDeliveryID extracts the X-GitHub-Delivery header from an HTTP request.
func ExtractDeliveryID(r *http.Request) string {
	return r.Header.Get("X-GitHub-Delivery")
}

// ExtractEventType extracts the X-GitHub-Event header from an HTTP request.
func ExtractEventType(r *http.Request) string {
	return r.Header.Get("X-GitHub-Event")
}
