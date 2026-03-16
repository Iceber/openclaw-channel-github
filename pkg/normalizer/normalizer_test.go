package normalizer

import (
	"testing"
	"time"

	"github.com/Iceber/openclaw-channel-github/pkg/events"
)

func TestNormalizeIssueComment(t *testing.T) {
	ts := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	e := &events.IssueCommentEvent{
		Action: events.ActionCreated,
		Issue: events.Issue{
			Number:  42,
			Title:   "Test Issue",
			HTMLURL: "https://github.com/owner/repo/issues/42",
		},
		Comment: events.Comment{
			ID:        999,
			Body:      "@openclaw-bot summarize",
			User:      events.User{ID: 100, Login: "alice", Type: "User"},
			CreatedAt: ts,
		},
		Repository: events.Repository{
			FullName: "owner/repo",
		},
		Sender: events.User{ID: 100, Login: "alice", Type: "User"},
	}

	normalized := NormalizeIssueComment(e)

	if normalized.Provider != "github" {
		t.Errorf("expected provider 'github', got %q", normalized.Provider)
	}
	if normalized.Repository != "owner/repo" {
		t.Errorf("expected repo 'owner/repo', got %q", normalized.Repository)
	}
	if normalized.Thread.Type != ThreadTypeIssue {
		t.Errorf("expected thread type 'issue', got %q", normalized.Thread.Type)
	}
	if normalized.Thread.Number != 42 {
		t.Errorf("expected thread number 42, got %d", normalized.Thread.Number)
	}
	if normalized.Message.Type != MessageTypeComment {
		t.Errorf("expected message type 'comment', got %q", normalized.Message.Type)
	}
	if normalized.Message.Text != "@openclaw-bot summarize" {
		t.Errorf("unexpected message text: %q", normalized.Message.Text)
	}
	if normalized.Sender.Login != "alice" {
		t.Errorf("expected sender 'alice', got %q", normalized.Sender.Login)
	}
	if normalized.Sender.IsBot {
		t.Error("expected sender to not be a bot")
	}
}

func TestNormalizeIssueCommentOnPR(t *testing.T) {
	e := &events.IssueCommentEvent{
		Issue: events.Issue{
			Number:  10,
			Title:   "feat: something",
			HTMLURL: "https://github.com/owner/repo/pull/10",
		},
		Comment: events.Comment{
			ID:   123,
			Body: "test",
		},
		Repository: events.Repository{FullName: "owner/repo"},
		Sender:     events.User{ID: 1, Login: "user", Type: "User"},
	}

	normalized := NormalizeIssueComment(e)

	if normalized.Thread.Type != ThreadTypePullRequest {
		t.Errorf("expected thread type 'pull_request' for PR URL, got %q", normalized.Thread.Type)
	}
}

func TestNormalizeIssueOpened(t *testing.T) {
	e := &events.IssueEvent{
		Action: events.ActionOpened,
		Issue: events.Issue{
			Number:  1,
			Title:   "Bug report",
			Body:    "Something is broken",
			HTMLURL: "https://github.com/owner/repo/issues/1",
		},
		Repository: events.Repository{FullName: "owner/repo"},
		Sender:     events.User{ID: 200, Login: "bob", Type: "User"},
	}

	normalized := NormalizeIssueOpened(e)

	if normalized.Message.Type != MessageTypeIssueBody {
		t.Errorf("expected message type 'issue_body', got %q", normalized.Message.Type)
	}
	if normalized.Message.Text != "Something is broken" {
		t.Errorf("unexpected message text: %q", normalized.Message.Text)
	}
}

func TestNormalizePullRequestOpened(t *testing.T) {
	e := &events.PullRequestEvent{
		Action: events.ActionOpened,
		PullRequest: events.PullRequest{
			Number:  5,
			Title:   "Add feature",
			Body:    "This adds a feature",
			HTMLURL: "https://github.com/owner/repo/pull/5",
		},
		Repository: events.Repository{FullName: "owner/repo"},
		Sender:     events.User{ID: 300, Login: "charlie", Type: "User"},
	}

	normalized := NormalizePullRequestOpened(e)

	if normalized.Thread.Type != ThreadTypePullRequest {
		t.Errorf("expected thread type 'pull_request', got %q", normalized.Thread.Type)
	}
	if normalized.Message.Type != MessageTypePRBody {
		t.Errorf("expected message type 'pr_body', got %q", normalized.Message.Type)
	}
}

func TestNormalizePullRequestReview(t *testing.T) {
	e := &events.PullRequestReviewEvent{
		Action: events.ActionSubmitted,
		Review: events.Review{
			ID:    777,
			Body:  "LGTM",
			State: "approved",
			User:  events.User{ID: 400, Login: "reviewer", Type: "User"},
		},
		PullRequest: events.PullRequest{
			Number:  5,
			Title:   "Add feature",
			HTMLURL: "https://github.com/owner/repo/pull/5",
		},
		Repository: events.Repository{FullName: "owner/repo"},
		Sender:     events.User{ID: 400, Login: "reviewer", Type: "User"},
	}

	normalized := NormalizePullRequestReview(e)

	if normalized.Message.Type != MessageTypeReview {
		t.Errorf("expected message type 'review', got %q", normalized.Message.Type)
	}
	if normalized.Message.ID != "review-777" {
		t.Errorf("expected message ID 'review-777', got %q", normalized.Message.ID)
	}
}

func TestSenderBotDetection(t *testing.T) {
	botEvent := &events.IssueCommentEvent{
		Issue:      events.Issue{Number: 1, HTMLURL: "https://github.com/o/r/issues/1"},
		Comment:    events.Comment{ID: 1, Body: "test"},
		Repository: events.Repository{FullName: "o/r"},
		Sender:     events.User{ID: 1, Login: "bot[bot]", Type: "Bot"},
	}

	normalized := NormalizeIssueComment(botEvent)
	if !normalized.Sender.IsBot {
		t.Error("expected sender to be identified as bot")
	}
}
