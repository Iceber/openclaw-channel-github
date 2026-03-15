// Package e2e provides end-to-end tests for the OpenClaw GitHub Channel.
// These tests spin up a full webhook server and a mock GitHub API server
// to validate the complete processing pipeline.
package e2e

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Iceber/openclaw-channel-github/pkg/auth"
	"github.com/Iceber/openclaw-channel-github/pkg/config"
	"github.com/Iceber/openclaw-channel-github/pkg/normalizer"
	"github.com/Iceber/openclaw-channel-github/pkg/outbound"
	"github.com/Iceber/openclaw-channel-github/pkg/server"
	"github.com/Iceber/openclaw-channel-github/pkg/state"
)

const testSecret = "e2e-test-secret"

// mockGitHubAPI records all API calls made by the outbound sender.
type mockGitHubAPI struct {
	mu       sync.Mutex
	calls    []apiCall
	server   *httptest.Server
}

type apiCall struct {
	Method string
	Path   string
	Body   string
}

func newMockGitHubAPI() *mockGitHubAPI {
	m := &mockGitHubAPI{}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		m.mu.Lock()
		m.calls = append(m.calls, apiCall{
			Method: r.Method,
			Path:   r.URL.Path,
			Body:   string(body),
		})
		m.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintln(w, `{"id": 1}`)
	}))
	return m
}

func (m *mockGitHubAPI) getCalls() []apiCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]apiCall, len(m.calls))
	copy(result, m.calls)
	return result
}

func (m *mockGitHubAPI) close() {
	m.server.Close()
}

func signPayload(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(testSecret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// setupTestServer creates a full test server with a mock GitHub API backend.
func setupTestServer(t *testing.T, cfgOverride func(*config.Config)) (*httptest.Server, *mockGitHubAPI, *[]capturedEvent) {
	t.Helper()

	mockAPI := newMockGitHubAPI()

	cfg := &config.Config{
		Server: config.ServerConfig{Addr: ":0"},
		Channel: config.ChannelConfig{
			Enabled:       true,
			Mode:          "app",
			AppID:         12345,
			InstallationID: 67890,
			PrivateKeyPath: "/dev/null",
			WebhookSecret: testSecret,
			Repositories:  []string{"owner/repo"},
			IgnoreBots:    true,
			Trigger: config.TriggerConfig{
				RequireMention: true,
				BotUsername:    "openclaw-bot",
				Commands:       []string{"/openclaw"},
				Labels:         []string{"ai-review"},
			},
		},
	}
	if cfgOverride != nil {
		cfgOverride(cfg)
	}

	ghAuth := auth.NewGitHubAuth(12345, nil)
	ghAuth.SetInstallationToken("test-token", time.Now().Add(time.Hour))
	store := state.NewStore(time.Hour)
	sender := outbound.NewSender(nil, mockAPI.server.URL)

	handler := server.NewHandler(cfg, ghAuth, store, sender, nil)

	var captured []capturedEvent
	var mu sync.Mutex
	handler.MessageHandler = func(sessionKey string, event *normalizer.NormalizedEvent) (string, error) {
		mu.Lock()
		captured = append(captured, capturedEvent{
			SessionKey: sessionKey,
			Event:      event,
		})
		mu.Unlock()
		// Return a reply to test outbound
		return "Echo: " + event.Message.Text, nil
	}

	mux := server.NewMux(handler)
	ts := httptest.NewServer(mux)

	t.Cleanup(func() {
		ts.Close()
		mockAPI.close()
	})

	return ts, mockAPI, &captured
}

type capturedEvent struct {
	SessionKey string
	Event      *normalizer.NormalizedEvent
}

func sendWebhook(t *testing.T, serverURL string, eventType string, payload []byte, deliveryID string) *http.Response {
	t.Helper()
	sig := signPayload(payload)
	req, err := http.NewRequest(http.MethodPost, serverURL+"/webhook", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", eventType)
	req.Header.Set("X-GitHub-Delivery", deliveryID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sending webhook: %v", err)
	}
	return resp
}

// --- E2E Test Cases ---

// TestE2E_IssueCommentMention tests the full pipeline:
// 1. User creates issue comment with @mention
// 2. Webhook received, verified, parsed, normalized
// 3. Trigger matched (mention)
// 4. Message handler called
// 5. Reply sent back to mock GitHub API
func TestE2E_IssueCommentMention(t *testing.T) {
	ts, mockAPI, captured := setupTestServer(t, nil)

	payload := buildIssueCommentPayload("owner/repo", "alice", "User", "@openclaw-bot summarize this issue", 42)
	resp := sendWebhook(t, ts.URL, "issue_comment", payload, "delivery-1")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "processed" {
		t.Fatalf("expected status 'processed', got %q", result["status"])
	}
	if result["session_key"] != "github:owner/repo:issue:42" {
		t.Fatalf("expected session key 'github:owner/repo:issue:42', got %q", result["session_key"])
	}

	// Verify message handler was called
	if len(*captured) != 1 {
		t.Fatalf("expected 1 captured event, got %d", len(*captured))
	}
	evt := (*captured)[0]
	if evt.Event.Sender.Login != "alice" {
		t.Errorf("expected sender 'alice', got %q", evt.Event.Sender.Login)
	}
	if evt.Event.Trigger.Kind != normalizer.TriggerKindMention {
		t.Errorf("expected trigger kind 'mention', got %q", evt.Event.Trigger.Kind)
	}

	// Verify outbound API call was made
	calls := mockAPI.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 API call, got %d", len(calls))
	}
	if calls[0].Path != "/repos/owner/repo/issues/42/comments" {
		t.Errorf("expected comment path, got %q", calls[0].Path)
	}
	if !strings.Contains(calls[0].Body, "Echo:") {
		t.Errorf("expected reply body, got %q", calls[0].Body)
	}
}

// TestE2E_IssueCommentCommand tests /openclaw command trigger.
func TestE2E_IssueCommentCommand(t *testing.T) {
	ts, _, captured := setupTestServer(t, nil)

	payload := buildIssueCommentPayload("owner/repo", "bob", "User", "/openclaw review this", 10)
	resp := sendWebhook(t, ts.URL, "issue_comment", payload, "delivery-cmd-1")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if len(*captured) != 1 {
		t.Fatalf("expected 1 captured event, got %d", len(*captured))
	}
	if (*captured)[0].Event.Trigger.Kind != normalizer.TriggerKindCommand {
		t.Errorf("expected trigger kind 'command', got %q", (*captured)[0].Event.Trigger.Kind)
	}
}

// TestE2E_PROpenedAutoTrigger tests auto-trigger on PR opened.
func TestE2E_PROpenedAutoTrigger(t *testing.T) {
	ts, _, captured := setupTestServer(t, func(cfg *config.Config) {
		cfg.Channel.AutoTrigger.OnPROpened = true
	})

	payload := buildPullRequestPayload("owner/repo", "charlie", "opened", 5)
	resp := sendWebhook(t, ts.URL, "pull_request", payload, "delivery-pr-1")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if len(*captured) != 1 {
		t.Fatalf("expected 1 captured event, got %d", len(*captured))
	}
	evt := (*captured)[0]
	if evt.Event.Thread.Type != normalizer.ThreadTypePullRequest {
		t.Errorf("expected thread type 'pull_request', got %q", evt.Event.Thread.Type)
	}
	if evt.Event.Trigger.Kind != normalizer.TriggerKindAuto {
		t.Errorf("expected trigger kind 'auto', got %q", evt.Event.Trigger.Kind)
	}
}

// TestE2E_RepoNotAllowed tests that non-allowlisted repos are rejected.
func TestE2E_RepoNotAllowed(t *testing.T) {
	ts, _, captured := setupTestServer(t, nil)

	payload := buildIssueCommentPayload("other/repo", "alice", "User", "@openclaw-bot help", 1)
	resp := sendWebhook(t, ts.URL, "issue_comment", payload, "delivery-reject-1")
	defer resp.Body.Close()

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "repo_not_allowed" {
		t.Errorf("expected status 'repo_not_allowed', got %q", result["status"])
	}
	if len(*captured) != 0 {
		t.Errorf("expected 0 captured events, got %d", len(*captured))
	}
}

// TestE2E_BotLoopPrevention tests that bot messages are ignored.
func TestE2E_BotLoopPrevention(t *testing.T) {
	ts, _, captured := setupTestServer(t, nil)

	payload := buildIssueCommentPayload("owner/repo", "openclaw-bot", "Bot", "Bot generated output", 42)
	resp := sendWebhook(t, ts.URL, "issue_comment", payload, "delivery-bot-1")
	defer resp.Body.Close()

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "bot_ignored" {
		t.Errorf("expected status 'bot_ignored', got %q", result["status"])
	}
	if len(*captured) != 0 {
		t.Errorf("expected 0 captured events, got %d", len(*captured))
	}
}

// TestE2E_DuplicateDelivery tests that duplicate deliveries are deduplicated.
func TestE2E_DuplicateDelivery(t *testing.T) {
	ts, mockAPI, captured := setupTestServer(t, nil)

	payload := buildIssueCommentPayload("owner/repo", "alice", "User", "@openclaw-bot help", 42)

	// First delivery
	resp1 := sendWebhook(t, ts.URL, "issue_comment", payload, "delivery-dup")
	resp1.Body.Close()

	// Second delivery (same ID)
	resp2 := sendWebhook(t, ts.URL, "issue_comment", payload, "delivery-dup")
	defer resp2.Body.Close()

	var result map[string]string
	json.NewDecoder(resp2.Body).Decode(&result)
	if result["status"] != "duplicate" {
		t.Errorf("expected status 'duplicate', got %q", result["status"])
	}

	// Only one event should be captured
	if len(*captured) != 1 {
		t.Errorf("expected 1 captured event, got %d", len(*captured))
	}
	// Only one API call
	if len(mockAPI.getCalls()) != 1 {
		t.Errorf("expected 1 API call, got %d", len(mockAPI.getCalls()))
	}
}

// TestE2E_NoTriggerMatch tests that events without trigger are skipped.
func TestE2E_NoTriggerMatch(t *testing.T) {
	ts, _, captured := setupTestServer(t, nil)

	payload := buildIssueCommentPayload("owner/repo", "alice", "User", "Just a regular comment", 42)
	resp := sendWebhook(t, ts.URL, "issue_comment", payload, "delivery-notrig")
	defer resp.Body.Close()

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "no_trigger" {
		t.Errorf("expected status 'no_trigger', got %q", result["status"])
	}
	if len(*captured) != 0 {
		t.Errorf("expected 0 captured events, got %d", len(*captured))
	}
}

// TestE2E_IssueOpened tests the full pipeline for issue opened events.
func TestE2E_IssueOpened(t *testing.T) {
	ts, _, captured := setupTestServer(t, func(cfg *config.Config) {
		cfg.Channel.AutoTrigger.OnIssueOpened = true
	})

	payload := buildIssuePayload("owner/repo", "alice", "opened", 99, "Bug: something is broken")
	resp := sendWebhook(t, ts.URL, "issues", payload, "delivery-issue-1")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if len(*captured) != 1 {
		t.Fatalf("expected 1 captured event, got %d", len(*captured))
	}
	evt := (*captured)[0]
	if evt.Event.Message.Type != normalizer.MessageTypeIssueBody {
		t.Errorf("expected message type 'issue_body', got %q", evt.Event.Message.Type)
	}
	if evt.SessionKey != "github:owner/repo:issue:99" {
		t.Errorf("expected session key 'github:owner/repo:issue:99', got %q", evt.SessionKey)
	}
}

// TestE2E_PRReviewSubmitted tests PR review events.
func TestE2E_PRReviewSubmitted(t *testing.T) {
	ts, _, captured := setupTestServer(t, func(cfg *config.Config) {
		cfg.Channel.Trigger.RequireMention = false
		cfg.Channel.Trigger.Commands = nil
		cfg.Channel.Trigger.Labels = nil
	})

	payload := buildPRReviewPayload("owner/repo", "reviewer", "submitted", 10, "LGTM!")
	resp := sendWebhook(t, ts.URL, "pull_request_review", payload, "delivery-review-1")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if len(*captured) != 1 {
		t.Fatalf("expected 1 captured event, got %d", len(*captured))
	}
	if (*captured)[0].Event.Message.Type != normalizer.MessageTypeReview {
		t.Errorf("expected message type 'review', got %q", (*captured)[0].Event.Message.Type)
	}
}

// TestE2E_LabelTrigger tests label-based triggering.
func TestE2E_LabelTrigger(t *testing.T) {
	ts, _, captured := setupTestServer(t, func(cfg *config.Config) {
		cfg.Channel.Trigger.RequireMention = false
	})

	payload := buildIssueLabeledPayload("owner/repo", "alice", 42, "ai-review")
	resp := sendWebhook(t, ts.URL, "issues", payload, "delivery-label-1")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if len(*captured) != 1 {
		t.Fatalf("expected 1 captured event, got %d", len(*captured))
	}
	if (*captured)[0].Event.Trigger.Kind != normalizer.TriggerKindLabel {
		t.Errorf("expected trigger kind 'label', got %q", (*captured)[0].Event.Trigger.Kind)
	}
}

// TestE2E_DiscussionComment tests discussion comment events.
func TestE2E_DiscussionComment(t *testing.T) {
	ts, _, captured := setupTestServer(t, func(cfg *config.Config) {
		cfg.Channel.Trigger.RequireMention = true
	})

	payload := buildDiscussionCommentPayload("owner/repo", "alice", 7, "@openclaw-bot explain")
	resp := sendWebhook(t, ts.URL, "discussion_comment", payload, "delivery-disc-1")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if len(*captured) != 1 {
		t.Fatalf("expected 1 captured event, got %d", len(*captured))
	}
	if (*captured)[0].Event.Thread.Type != normalizer.ThreadTypeDiscussion {
		t.Errorf("expected thread type 'discussion', got %q", (*captured)[0].Event.Thread.Type)
	}
}

// TestE2E_HealthEndpoint tests the health check endpoint.
func TestE2E_HealthEndpoint(t *testing.T) {
	ts, _, _ := setupTestServer(t, nil)

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestE2E_InvalidSignature tests that invalid signatures are rejected.
func TestE2E_InvalidSignature(t *testing.T) {
	ts, _, _ := setupTestServer(t, nil)

	payload := buildIssueCommentPayload("owner/repo", "alice", "User", "@openclaw-bot help", 42)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/webhook", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-GitHub-Delivery", "delivery-sig")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// TestE2E_PRCommentOnPR tests that issue_comment events on PRs are correctly typed.
func TestE2E_PRCommentOnPR(t *testing.T) {
	ts, _, captured := setupTestServer(t, nil)

	payload := buildPRCommentPayload("owner/repo", "alice", "User", "@openclaw-bot review", 15)
	resp := sendWebhook(t, ts.URL, "issue_comment", payload, "delivery-prcomment-1")
	defer resp.Body.Close()

	if len(*captured) != 1 {
		t.Fatalf("expected 1 captured event, got %d", len(*captured))
	}
	if (*captured)[0].Event.Thread.Type != normalizer.ThreadTypePullRequest {
		t.Errorf("expected thread type 'pull_request', got %q", (*captured)[0].Event.Thread.Type)
	}
}

// TestE2E_MultipleEventsSessionConsistency tests that the same issue always maps to the same session.
func TestE2E_MultipleEventsSessionConsistency(t *testing.T) {
	ts, _, captured := setupTestServer(t, nil)

	payload1 := buildIssueCommentPayload("owner/repo", "alice", "User", "@openclaw-bot help", 42)
	resp1 := sendWebhook(t, ts.URL, "issue_comment", payload1, "delivery-sess-1")
	resp1.Body.Close()

	payload2 := buildIssueCommentPayload("owner/repo", "bob", "User", "@openclaw-bot also help", 42)
	resp2 := sendWebhook(t, ts.URL, "issue_comment", payload2, "delivery-sess-2")
	resp2.Body.Close()

	if len(*captured) != 2 {
		t.Fatalf("expected 2 captured events, got %d", len(*captured))
	}
	if (*captured)[0].SessionKey != (*captured)[1].SessionKey {
		t.Errorf("expected same session key, got %q and %q", (*captured)[0].SessionKey, (*captured)[1].SessionKey)
	}
}

// --- Payload Builders ---

func buildIssueCommentPayload(repo, sender, senderType, body string, issueNumber int) []byte {
	payload := map[string]any{
		"action": "created",
		"issue": map[string]any{
			"number": issueNumber,
			"title": "Test Issue",
			"body": "Issue body",
			"state": "open",
			"user": map[string]any{"id": 100, "login": sender, "type": senderType},
			"labels": []any{},
			"assignees": []any{},
			"html_url": fmt.Sprintf("https://github.com/%s/issues/%d", repo, issueNumber),
		},
		"comment": map[string]any{
			"id": 999,
			"body": body,
			"user": map[string]any{"id": 100, "login": sender, "type": senderType},
			"html_url": fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-999", repo, issueNumber),
		},
		"repository": map[string]any{
			"id": 1, "full_name": repo, "name": "repo",
			"owner": map[string]any{"id": 10, "login": "owner", "type": "Organization"},
		},
		"sender": map[string]any{"id": 100, "login": sender, "type": senderType},
		"installation": map[string]any{"id": 5555},
	}
	data, _ := json.Marshal(payload)
	return data
}

func buildPRCommentPayload(repo, sender, senderType, body string, prNumber int) []byte {
	payload := map[string]any{
		"action": "created",
		"issue": map[string]any{
			"number": prNumber,
			"title": "Test PR",
			"body": "PR body",
			"state": "open",
			"user": map[string]any{"id": 100, "login": sender, "type": senderType},
			"labels": []any{},
			"assignees": []any{},
			"html_url": fmt.Sprintf("https://github.com/%s/pull/%d", repo, prNumber),
		},
		"comment": map[string]any{
			"id": 888,
			"body": body,
			"user": map[string]any{"id": 100, "login": sender, "type": senderType},
			"html_url": fmt.Sprintf("https://github.com/%s/pull/%d#issuecomment-888", repo, prNumber),
		},
		"repository": map[string]any{
			"id": 1, "full_name": repo, "name": "repo",
			"owner": map[string]any{"id": 10, "login": "owner", "type": "Organization"},
		},
		"sender": map[string]any{"id": 100, "login": sender, "type": senderType},
		"installation": map[string]any{"id": 5555},
	}
	data, _ := json.Marshal(payload)
	return data
}

func buildPullRequestPayload(repo, sender, action string, prNumber int) []byte {
	payload := map[string]any{
		"action": action,
		"pull_request": map[string]any{
			"number": prNumber,
			"title": "feat: new feature",
			"body": "This adds a cool feature",
			"state": "open",
			"merged": false,
			"user": map[string]any{"id": 300, "login": sender, "type": "User"},
			"labels": []any{},
			"assignees": []any{},
			"html_url": fmt.Sprintf("https://github.com/%s/pull/%d", repo, prNumber),
			"diff_url": fmt.Sprintf("https://github.com/%s/pull/%d.diff", repo, prNumber),
			"head": map[string]any{"ref": "feature-branch", "sha": "abc12345"},
			"base": map[string]any{"ref": "main", "sha": "def67890"},
		},
		"repository": map[string]any{
			"id": 1, "full_name": repo, "name": "repo",
			"owner": map[string]any{"id": 10, "login": "owner", "type": "Organization"},
		},
		"sender": map[string]any{"id": 300, "login": sender, "type": "User"},
		"installation": map[string]any{"id": 5555},
	}
	data, _ := json.Marshal(payload)
	return data
}

func buildIssuePayload(repo, sender, action string, number int, title string) []byte {
	payload := map[string]any{
		"action": action,
		"issue": map[string]any{
			"number": number,
			"title": title,
			"body": "Something is broken",
			"state": "open",
			"user": map[string]any{"id": 200, "login": sender, "type": "User"},
			"labels": []any{},
			"assignees": []any{},
			"html_url": fmt.Sprintf("https://github.com/%s/issues/%d", repo, number),
		},
		"repository": map[string]any{
			"id": 1, "full_name": repo, "name": "repo",
			"owner": map[string]any{"id": 10, "login": "owner", "type": "Organization"},
		},
		"sender": map[string]any{"id": 200, "login": sender, "type": "User"},
		"installation": map[string]any{"id": 5555},
	}
	data, _ := json.Marshal(payload)
	return data
}

func buildPRReviewPayload(repo, sender, action string, prNumber int, body string) []byte {
	payload := map[string]any{
		"action": action,
		"review": map[string]any{
			"id": 777,
			"body": body,
			"state": "approved",
			"user": map[string]any{"id": 400, "login": sender, "type": "User"},
			"html_url": fmt.Sprintf("https://github.com/%s/pull/%d#pullrequestreview-777", repo, prNumber),
		},
		"pull_request": map[string]any{
			"number": prNumber,
			"title": "feat: feature",
			"body": "PR body",
			"state": "open",
			"merged": false,
			"user": map[string]any{"id": 300, "login": "author", "type": "User"},
			"labels": []any{},
			"assignees": []any{},
			"html_url": fmt.Sprintf("https://github.com/%s/pull/%d", repo, prNumber),
			"head": map[string]any{"ref": "feature", "sha": "abc12345"},
			"base": map[string]any{"ref": "main", "sha": "def67890"},
		},
		"repository": map[string]any{
			"id": 1, "full_name": repo, "name": "repo",
			"owner": map[string]any{"id": 10, "login": "owner", "type": "Organization"},
		},
		"sender": map[string]any{"id": 400, "login": sender, "type": "User"},
		"installation": map[string]any{"id": 5555},
	}
	data, _ := json.Marshal(payload)
	return data
}

func buildIssueLabeledPayload(repo, sender string, issueNumber int, labelName string) []byte {
	payload := map[string]any{
		"action": "labeled",
		"issue": map[string]any{
			"number": issueNumber,
			"title": "Test Issue",
			"body": "Issue body",
			"state": "open",
			"user": map[string]any{"id": 100, "login": sender, "type": "User"},
			"labels": []any{map[string]any{"name": labelName}},
			"assignees": []any{},
			"html_url": fmt.Sprintf("https://github.com/%s/issues/%d", repo, issueNumber),
		},
		"label": map[string]any{"name": labelName},
		"repository": map[string]any{
			"id": 1, "full_name": repo, "name": "repo",
			"owner": map[string]any{"id": 10, "login": "owner", "type": "Organization"},
		},
		"sender": map[string]any{"id": 100, "login": sender, "type": "User"},
		"installation": map[string]any{"id": 5555},
	}
	data, _ := json.Marshal(payload)
	return data
}

func buildDiscussionCommentPayload(repo, sender string, discNumber int, body string) []byte {
	payload := map[string]any{
		"action": "created",
		"discussion": map[string]any{
			"number": discNumber,
			"title": "Test Discussion",
			"body": "Discussion body",
			"state": "open",
			"category": map[string]any{"name": "General", "slug": "general"},
			"user": map[string]any{"id": 100, "login": sender, "type": "User"},
			"html_url": fmt.Sprintf("https://github.com/%s/discussions/%d", repo, discNumber),
		},
		"comment": map[string]any{
			"id": 555,
			"body": body,
			"user": map[string]any{"id": 100, "login": sender, "type": "User"},
			"html_url": fmt.Sprintf("https://github.com/%s/discussions/%d#discussioncomment-555", repo, discNumber),
		},
		"repository": map[string]any{
			"id": 1, "full_name": repo, "name": "repo",
			"owner": map[string]any{"id": 10, "login": "owner", "type": "Organization"},
		},
		"sender": map[string]any{"id": 100, "login": sender, "type": "User"},
		"installation": map[string]any{"id": 5555},
	}
	data, _ := json.Marshal(payload)
	return data
}
