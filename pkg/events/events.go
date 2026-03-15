// Package events provides types for GitHub webhook events and payload parsing.
package events

import (
	"encoding/json"
	"fmt"
	"time"
)

// EventType represents a GitHub webhook event type.
type EventType string

const (
	EventIssues                    EventType = "issues"
	EventIssueComment              EventType = "issue_comment"
	EventPullRequest               EventType = "pull_request"
	EventPullRequestReview         EventType = "pull_request_review"
	EventPullRequestReviewComment  EventType = "pull_request_review_comment"
)

// Action represents the action within a GitHub webhook event.
type Action string

const (
	ActionOpened    Action = "opened"
	ActionEdited    Action = "edited"
	ActionClosed    Action = "closed"
	ActionCreated   Action = "created"
	ActionSubmitted Action = "submitted"
	ActionLabeled   Action = "labeled"
	ActionUnlabeled Action = "unlabeled"
)

// WebhookEvent is the raw envelope for a GitHub webhook delivery.
type WebhookEvent struct {
	DeliveryID string    `json:"-"`
	EventType  EventType `json:"-"`
	Action     Action    `json:"action"`
	Payload    json.RawMessage
}

// User represents a GitHub user.
type User struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Type  string `json:"type"` // "User", "Bot", "Organization"
}

// Repository represents a GitHub repository.
type Repository struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"` // "owner/repo"
	Name     string `json:"name"`
	Owner    User   `json:"owner"`
}

// Label represents a GitHub label.
type Label struct {
	Name string `json:"name"`
}

// Issue represents a GitHub issue.
type Issue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	User      User      `json:"user"`
	Labels    []Label   `json:"labels"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
}

// PullRequest represents a GitHub pull request.
type PullRequest struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	User      User      `json:"user"`
	Labels    []Label   `json:"labels"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
}

// Comment represents a GitHub comment (on issue or PR).
type Comment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	User      User      `json:"user"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
}

// Review represents a GitHub pull request review.
type Review struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	State     string    `json:"state"` // "approved", "changes_requested", "commented"
	User      User      `json:"user"`
	HTMLURL   string    `json:"html_url"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// Installation represents a GitHub App installation.
type Installation struct {
	ID int64 `json:"id"`
}

// IssueEvent is the payload for issue webhook events.
type IssueEvent struct {
	Action       Action       `json:"action"`
	Issue        Issue        `json:"issue"`
	Repository   Repository   `json:"repository"`
	Sender       User         `json:"sender"`
	Installation Installation `json:"installation"`
}

// IssueCommentEvent is the payload for issue_comment webhook events.
type IssueCommentEvent struct {
	Action       Action       `json:"action"`
	Issue        Issue        `json:"issue"`
	Comment      Comment      `json:"comment"`
	Repository   Repository   `json:"repository"`
	Sender       User         `json:"sender"`
	Installation Installation `json:"installation"`
}

// PullRequestEvent is the payload for pull_request webhook events.
type PullRequestEvent struct {
	Action       Action       `json:"action"`
	PullRequest  PullRequest  `json:"pull_request"`
	Repository   Repository   `json:"repository"`
	Sender       User         `json:"sender"`
	Installation Installation `json:"installation"`
}

// PullRequestReviewEvent is the payload for pull_request_review webhook events.
type PullRequestReviewEvent struct {
	Action       Action       `json:"action"`
	Review       Review       `json:"review"`
	PullRequest  PullRequest  `json:"pull_request"`
	Repository   Repository   `json:"repository"`
	Sender       User         `json:"sender"`
	Installation Installation `json:"installation"`
}

// PullRequestReviewCommentEvent is the payload for pull_request_review_comment webhook events.
type PullRequestReviewCommentEvent struct {
	Action       Action       `json:"action"`
	Comment      Comment      `json:"comment"`
	PullRequest  PullRequest  `json:"pull_request"`
	Repository   Repository   `json:"repository"`
	Sender       User         `json:"sender"`
	Installation Installation `json:"installation"`
}

// ParseEvent parses a webhook payload based on the event type.
func ParseEvent(eventType EventType, payload []byte) (any, error) {
	switch eventType {
	case EventIssues:
		var e IssueEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, fmt.Errorf("parsing issue event: %w", err)
		}
		return &e, nil

	case EventIssueComment:
		var e IssueCommentEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, fmt.Errorf("parsing issue_comment event: %w", err)
		}
		return &e, nil

	case EventPullRequest:
		var e PullRequestEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, fmt.Errorf("parsing pull_request event: %w", err)
		}
		return &e, nil

	case EventPullRequestReview:
		var e PullRequestReviewEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, fmt.Errorf("parsing pull_request_review event: %w", err)
		}
		return &e, nil

	case EventPullRequestReviewComment:
		var e PullRequestReviewCommentEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, fmt.Errorf("parsing pull_request_review_comment event: %w", err)
		}
		return &e, nil

	default:
		return nil, fmt.Errorf("unsupported event type: %s", eventType)
	}
}
