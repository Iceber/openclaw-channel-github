package events

import (
	"testing"
)

func TestParseIssueCommentEvent(t *testing.T) {
	payload := []byte(`{
		"action": "created",
		"issue": {
			"number": 42,
			"title": "Test Issue",
			"body": "Issue body",
			"state": "open",
			"user": {"id": 100, "login": "alice", "type": "User"},
			"html_url": "https://github.com/owner/repo/issues/42"
		},
		"comment": {
			"id": 999,
			"body": "@openclaw-bot help me",
			"user": {"id": 100, "login": "alice", "type": "User"},
			"html_url": "https://github.com/owner/repo/issues/42#issuecomment-999"
		},
		"repository": {
			"id": 1,
			"full_name": "owner/repo",
			"name": "repo",
			"owner": {"id": 10, "login": "owner", "type": "Organization"}
		},
		"sender": {"id": 100, "login": "alice", "type": "User"},
		"installation": {"id": 5555}
	}`)

	parsed, err := ParseEvent(EventIssueComment, payload)
	if err != nil {
		t.Fatalf("ParseEvent() error: %v", err)
	}

	event, ok := parsed.(*IssueCommentEvent)
	if !ok {
		t.Fatalf("expected *IssueCommentEvent, got %T", parsed)
	}

	if event.Action != ActionCreated {
		t.Errorf("expected action 'created', got %q", event.Action)
	}
	if event.Issue.Number != 42 {
		t.Errorf("expected issue number 42, got %d", event.Issue.Number)
	}
	if event.Comment.ID != 999 {
		t.Errorf("expected comment ID 999, got %d", event.Comment.ID)
	}
	if event.Comment.Body != "@openclaw-bot help me" {
		t.Errorf("unexpected comment body: %q", event.Comment.Body)
	}
	if event.Repository.FullName != "owner/repo" {
		t.Errorf("expected repo 'owner/repo', got %q", event.Repository.FullName)
	}
	if event.Sender.Login != "alice" {
		t.Errorf("expected sender 'alice', got %q", event.Sender.Login)
	}
}

func TestParseIssueEvent(t *testing.T) {
	payload := []byte(`{
		"action": "opened",
		"issue": {
			"number": 1,
			"title": "New Issue",
			"body": "Please help",
			"state": "open",
			"user": {"id": 200, "login": "bob", "type": "User"},
			"html_url": "https://github.com/owner/repo/issues/1"
		},
		"repository": {
			"id": 1,
			"full_name": "owner/repo",
			"name": "repo",
			"owner": {"id": 10, "login": "owner", "type": "Organization"}
		},
		"sender": {"id": 200, "login": "bob", "type": "User"},
		"installation": {"id": 5555}
	}`)

	parsed, err := ParseEvent(EventIssues, payload)
	if err != nil {
		t.Fatalf("ParseEvent() error: %v", err)
	}

	event, ok := parsed.(*IssueEvent)
	if !ok {
		t.Fatalf("expected *IssueEvent, got %T", parsed)
	}

	if event.Action != ActionOpened {
		t.Errorf("expected action 'opened', got %q", event.Action)
	}
	if event.Issue.Title != "New Issue" {
		t.Errorf("expected title 'New Issue', got %q", event.Issue.Title)
	}
}

func TestParsePullRequestEvent(t *testing.T) {
	payload := []byte(`{
		"action": "opened",
		"pull_request": {
			"number": 10,
			"title": "feat: new feature",
			"body": "Adds something cool",
			"state": "open",
			"user": {"id": 300, "login": "charlie", "type": "User"},
			"html_url": "https://github.com/owner/repo/pull/10"
		},
		"repository": {
			"id": 1,
			"full_name": "owner/repo",
			"name": "repo",
			"owner": {"id": 10, "login": "owner", "type": "Organization"}
		},
		"sender": {"id": 300, "login": "charlie", "type": "User"},
		"installation": {"id": 5555}
	}`)

	parsed, err := ParseEvent(EventPullRequest, payload)
	if err != nil {
		t.Fatalf("ParseEvent() error: %v", err)
	}

	event, ok := parsed.(*PullRequestEvent)
	if !ok {
		t.Fatalf("expected *PullRequestEvent, got %T", parsed)
	}

	if event.PullRequest.Number != 10 {
		t.Errorf("expected PR number 10, got %d", event.PullRequest.Number)
	}
}

func TestParseUnsupportedEvent(t *testing.T) {
	_, err := ParseEvent(EventType("unknown"), []byte(`{}`))
	if err == nil {
		t.Error("expected error for unsupported event type")
	}
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := ParseEvent(EventIssues, []byte(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
