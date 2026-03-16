package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Iceber/openclaw-channel-github/pkg/config"
	"github.com/Iceber/openclaw-channel-github/pkg/normalizer"
	"github.com/Iceber/openclaw-channel-github/pkg/outbound"
	"github.com/Iceber/openclaw-channel-github/pkg/state"
)

const testWebhookSecret = "test-webhook-secret"

func testConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{Addr: ":8080"},
		Channel: config.ChannelConfig{
			Enabled:       true,
			Mode:          "app",
			WebhookSecret: testWebhookSecret,
			Repositories:  []string{"owner/repo"},
			IgnoreBots:    true,
			Trigger: config.TriggerConfig{
				RequireMention: true,
				BotUsername:    "openclaw-bot",
				Commands:       []string{"/openclaw"},
			},
		},
	}
}

func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func newTestHandler() *Handler {
	cfg := testConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	store := state.NewStore(time.Hour)
	sender := outbound.NewSender(nil, "")
	return NewHandler(cfg, nil, store, sender, logger)
}

func TestWebhookMethodNotAllowed(t *testing.T) {
	handler := newTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestWebhookInvalidSignature(t *testing.T) {
	handler := newTestHandler()

	payload := []byte(`{"action":"created"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	req.Header.Set("X-GitHub-Event", "issue_comment")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestWebhookUnsupportedEvent(t *testing.T) {
	handler := newTestHandler()

	payload := []byte(`{"action":"completed"}`)
	sig := signPayload(payload, testWebhookSecret)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "check_run")
	req.Header.Set("X-GitHub-Delivery", "delivery-1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for unsupported event, got %d", w.Code)
	}
}

func TestWebhookDuplicateDelivery(t *testing.T) {
	handler := newTestHandler()

	payload := issueCommentPayload(t, "owner/repo", "alice", "User", "@openclaw-bot help")
	sig := signPayload(payload, testWebhookSecret)

	// First request
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-GitHub-Delivery", "dup-delivery")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", w.Code)
	}

	// Second request with same delivery ID
	req2 := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req2.Header.Set("X-Hub-Signature-256", sig)
	req2.Header.Set("X-GitHub-Event", "issue_comment")
	req2.Header.Set("X-GitHub-Delivery", "dup-delivery")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d", w2.Code)
	}

	var resp map[string]string
	json.Unmarshal(w2.Body.Bytes(), &resp)
	if resp["status"] != "duplicate" {
		t.Errorf("expected duplicate status, got %q", resp["status"])
	}
}

func TestWebhookRepoNotAllowed(t *testing.T) {
	handler := newTestHandler()

	payload := issueCommentPayload(t, "other/repo", "alice", "User", "@openclaw-bot help")
	sig := signPayload(payload, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-GitHub-Delivery", "delivery-repo-check")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "repo_not_allowed" {
		t.Errorf("expected repo_not_allowed, got %q", resp["status"])
	}
}

func TestWebhookBotIgnored(t *testing.T) {
	handler := newTestHandler()

	payload := issueCommentPayload(t, "owner/repo", "openclaw-bot", "Bot", "some output")
	sig := signPayload(payload, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-GitHub-Delivery", "delivery-bot")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "bot_ignored" {
		t.Errorf("expected bot_ignored, got %q", resp["status"])
	}
}

func TestWebhookNoTrigger(t *testing.T) {
	handler := newTestHandler()

	payload := issueCommentPayload(t, "owner/repo", "alice", "User", "just a comment without mention or command")
	sig := signPayload(payload, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-GitHub-Delivery", "delivery-no-trigger")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "no_trigger" {
		t.Errorf("expected no_trigger, got %q", resp["status"])
	}
}

func TestWebhookSuccessfulProcessing(t *testing.T) {
	handler := newTestHandler()

	var capturedSessionKey string
	var capturedEvent *normalizer.NormalizedEvent
	handler.MessageHandler = func(sessionKey string, event *normalizer.NormalizedEvent) (string, error) {
		capturedSessionKey = sessionKey
		capturedEvent = event
		return "", nil // no reply
	}

	payload := issueCommentPayload(t, "owner/repo", "alice", "User", "@openclaw-bot summarize")
	sig := signPayload(payload, testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-GitHub-Delivery", "delivery-success")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "processed" {
		t.Errorf("expected processed status, got %q", resp["status"])
	}

	if capturedSessionKey != "github:owner/repo:issue:42" {
		t.Errorf("expected session key 'github:owner/repo:issue:42', got %q", capturedSessionKey)
	}

	if capturedEvent == nil {
		t.Fatal("expected captured event to be non-nil")
	}
	if capturedEvent.Sender.Login != "alice" {
		t.Errorf("expected sender 'alice', got %q", capturedEvent.Sender.Login)
	}
	if capturedEvent.Trigger.Kind != normalizer.TriggerKindMention {
		t.Errorf("expected trigger kind 'mention', got %q", capturedEvent.Trigger.Kind)
	}
}

func TestHealthEndpoint(t *testing.T) {
	handler := newTestHandler()
	mux := NewMux(handler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// issueCommentPayload creates a test issue_comment webhook payload.
func issueCommentPayload(t *testing.T, repo, senderLogin, senderType, body string) []byte {
	t.Helper()
	payload := map[string]any{
		"action": "created",
		"issue": map[string]any{
			"number":   42,
			"title":    "Test Issue",
			"body":     "Issue body",
			"state":    "open",
			"user":     map[string]any{"id": 100, "login": senderLogin, "type": senderType},
			"html_url": "https://github.com/" + repo + "/issues/42",
		},
		"comment": map[string]any{
			"id":       999,
			"body":     body,
			"user":     map[string]any{"id": 100, "login": senderLogin, "type": senderType},
			"html_url": "https://github.com/" + repo + "/issues/42#issuecomment-999",
		},
		"repository": map[string]any{
			"id":        1,
			"full_name": repo,
			"name":      "repo",
			"owner":     map[string]any{"id": 10, "login": "owner", "type": "Organization"},
		},
		"sender":       map[string]any{"id": 100, "login": senderLogin, "type": senderType},
		"installation": map[string]any{"id": 5555},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal test payload: %v", err)
	}
	return data
}
