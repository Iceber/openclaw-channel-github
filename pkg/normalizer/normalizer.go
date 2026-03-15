// Package normalizer converts GitHub webhook events into normalized OpenClaw messages.
package normalizer

import (
	"time"

	"github.com/Iceber/openclaw-channel-github/pkg/events"
)

// Provider is the constant provider identifier for GitHub.
const Provider = "github"

// ThreadType represents the type of conversation thread.
type ThreadType string

const (
	ThreadTypeIssue       ThreadType = "issue"
	ThreadTypePullRequest ThreadType = "pull_request"
)

// MessageType represents the type of normalized message.
type MessageType string

const (
	MessageTypeComment     MessageType = "comment"
	MessageTypeIssueBody   MessageType = "issue_body"
	MessageTypePRBody      MessageType = "pr_body"
	MessageTypeReview      MessageType = "review"
)

// TriggerKind describes how the event was triggered.
type TriggerKind string

const (
	TriggerKindMention TriggerKind = "mention"
	TriggerKindCommand TriggerKind = "command"
	TriggerKindLabel   TriggerKind = "label"
	TriggerKindAuto    TriggerKind = "auto"
	TriggerKindNone    TriggerKind = ""
)

// Thread represents the conversation thread context.
type Thread struct {
	Type   ThreadType `json:"type"`
	Number int        `json:"number"`
	Title  string     `json:"title"`
	URL    string     `json:"url"`
}

// Message represents a normalized message within a thread.
type Message struct {
	Type      MessageType `json:"type"`
	ID        string      `json:"id"`
	Text      string      `json:"text"`
	CreatedAt time.Time   `json:"createdAt"`
}

// Sender represents the message sender.
type Sender struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	Association string `json:"association,omitempty"`
	IsBot       bool   `json:"isBot"`
}

// Trigger describes how this event was triggered.
type Trigger struct {
	Kind    TriggerKind `json:"kind"`
	Command string      `json:"command,omitempty"`
}

// NormalizedEvent is the unified event format consumed by the OpenClaw Gateway.
type NormalizedEvent struct {
	Provider   string  `json:"provider"`
	AccountID  string  `json:"accountId"`
	Repository string  `json:"repository"`
	Thread     Thread  `json:"thread"`
	Message    Message `json:"message"`
	Sender     Sender  `json:"sender"`
	Trigger    Trigger `json:"trigger"`
}

// NormalizeIssueOpened normalizes an issue opened event.
func NormalizeIssueOpened(e *events.IssueEvent) *NormalizedEvent {
	return &NormalizedEvent{
		Provider:   Provider,
		Repository: e.Repository.FullName,
		Thread: Thread{
			Type:   ThreadTypeIssue,
			Number: e.Issue.Number,
			Title:  e.Issue.Title,
			URL:    e.Issue.HTMLURL,
		},
		Message: Message{
			Type:      MessageTypeIssueBody,
			ID:        formatIssueID(e.Repository.FullName, e.Issue.Number),
			Text:      e.Issue.Body,
			CreatedAt: e.Issue.CreatedAt,
		},
		Sender: senderFromUser(e.Sender),
	}
}

// NormalizeIssueComment normalizes an issue comment event.
func NormalizeIssueComment(e *events.IssueCommentEvent) *NormalizedEvent {
	return &NormalizedEvent{
		Provider:   Provider,
		Repository: e.Repository.FullName,
		Thread: Thread{
			Type:   threadTypeFromIssue(e.Issue),
			Number: e.Issue.Number,
			Title:  e.Issue.Title,
			URL:    e.Issue.HTMLURL,
		},
		Message: Message{
			Type:      MessageTypeComment,
			ID:        formatCommentID(e.Comment.ID),
			Text:      e.Comment.Body,
			CreatedAt: e.Comment.CreatedAt,
		},
		Sender: senderFromUser(e.Sender),
	}
}

// NormalizePullRequestOpened normalizes a pull request opened event.
func NormalizePullRequestOpened(e *events.PullRequestEvent) *NormalizedEvent {
	return &NormalizedEvent{
		Provider:   Provider,
		Repository: e.Repository.FullName,
		Thread: Thread{
			Type:   ThreadTypePullRequest,
			Number: e.PullRequest.Number,
			Title:  e.PullRequest.Title,
			URL:    e.PullRequest.HTMLURL,
		},
		Message: Message{
			Type:      MessageTypePRBody,
			ID:        formatPRID(e.Repository.FullName, e.PullRequest.Number),
			Text:      e.PullRequest.Body,
			CreatedAt: e.PullRequest.CreatedAt,
		},
		Sender: senderFromUser(e.Sender),
	}
}

// NormalizePullRequestReview normalizes a pull request review event.
func NormalizePullRequestReview(e *events.PullRequestReviewEvent) *NormalizedEvent {
	return &NormalizedEvent{
		Provider:   Provider,
		Repository: e.Repository.FullName,
		Thread: Thread{
			Type:   ThreadTypePullRequest,
			Number: e.PullRequest.Number,
			Title:  e.PullRequest.Title,
			URL:    e.PullRequest.HTMLURL,
		},
		Message: Message{
			Type:      MessageTypeReview,
			ID:        formatReviewID(e.Review.ID),
			Text:      e.Review.Body,
			CreatedAt: e.Review.SubmittedAt,
		},
		Sender: senderFromUser(e.Sender),
	}
}

// NormalizePullRequestReviewComment normalizes a PR review comment event.
func NormalizePullRequestReviewComment(e *events.PullRequestReviewCommentEvent) *NormalizedEvent {
	return &NormalizedEvent{
		Provider:   Provider,
		Repository: e.Repository.FullName,
		Thread: Thread{
			Type:   ThreadTypePullRequest,
			Number: e.PullRequest.Number,
			Title:  e.PullRequest.Title,
			URL:    e.PullRequest.HTMLURL,
		},
		Message: Message{
			Type:      MessageTypeComment,
			ID:        formatCommentID(e.Comment.ID),
			Text:      e.Comment.Body,
			CreatedAt: e.Comment.CreatedAt,
		},
		Sender: senderFromUser(e.Sender),
	}
}

func senderFromUser(u events.User) Sender {
	return Sender{
		ID:    formatSenderID(u.ID),
		Login: u.Login,
		IsBot: u.Type == "Bot",
	}
}

// threadTypeFromIssue determines if the issue is actually a PR.
// GitHub sends issue_comment events for PR comments too;
// PRs have a non-empty "pull_request" key, but our Issue type
// checks the HTMLURL for "/pull/" as a heuristic.
func threadTypeFromIssue(issue events.Issue) ThreadType {
	if containsPullPath(issue.HTMLURL) {
		return ThreadTypePullRequest
	}
	return ThreadTypeIssue
}

func containsPullPath(url string) bool {
	// Simple check: GitHub PR URLs contain "/pull/"
	for i := 0; i < len(url)-5; i++ {
		if url[i:i+6] == "/pull/" {
			return true
		}
	}
	return false
}

func formatSenderID(id int64) string {
	return "github:" + itoa(id)
}

func formatCommentID(id int64) string {
	return "comment-" + itoa(id)
}

func formatReviewID(id int64) string {
	return "review-" + itoa(id)
}

func formatIssueID(repo string, number int) string {
	return repo + ":issue:" + itoa(int64(number))
}

func formatPRID(repo string, number int) string {
	return repo + ":pr:" + itoa(int64(number))
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
