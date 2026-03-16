// Package normalizer converts GitHub webhook events into normalized OpenClaw messages.
package normalizer

import (
	"fmt"
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
	ThreadTypeDiscussion  ThreadType = "discussion"
)

// MessageType represents the type of normalized message.
type MessageType string

const (
	MessageTypeComment       MessageType = "comment"
	MessageTypeIssueBody     MessageType = "issue_body"
	MessageTypePRBody        MessageType = "pr_body"
	MessageTypeReview        MessageType = "review"
	MessageTypeReviewComment MessageType = "review_comment"
	MessageTypeDiscussionBody MessageType = "discussion_body"
	MessageTypeContextUpdate MessageType = "context_update"
	MessageTypeCIStatus      MessageType = "ci_status"
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
	Provider   string   `json:"provider"`
	AccountID  string   `json:"accountId"`
	Repository string   `json:"repository"`
	Thread     Thread   `json:"thread"`
	Message    Message  `json:"message"`
	Sender     Sender   `json:"sender"`
	Trigger    Trigger  `json:"trigger"`
	Context    *Context `json:"context,omitempty"`
}

// Context holds additional metadata about the event for agent decision-making.
type Context struct {
	// Labels on the issue/PR at event time.
	Labels []string `json:"labels,omitempty"`
	// Assignees of the issue/PR.
	Assignees []string `json:"assignees,omitempty"`
	// State of the issue/PR (open, closed, merged).
	State string `json:"state,omitempty"`
	// EventAction is the raw GitHub action (opened, edited, closed, labeled, etc.).
	EventAction string `json:"eventAction,omitempty"`
	// ReviewState for review events (approved, changes_requested, commented).
	ReviewState string `json:"reviewState,omitempty"`
	// FilePath for inline review comments.
	FilePath string `json:"filePath,omitempty"`
	// Line number for inline review comments.
	Line int `json:"line,omitempty"`
	// CIStatus for check_run/workflow_run events.
	CIStatus string `json:"ciStatus,omitempty"`
	// CIConclusion for check_run/workflow_run events.
	CIConclusion string `json:"ciConclusion,omitempty"`
	// Merged indicates if a PR was merged.
	Merged bool `json:"merged,omitempty"`
	// HeadRef is the head branch of a PR.
	HeadRef string `json:"headRef,omitempty"`
	// BaseRef is the base branch of a PR.
	BaseRef string `json:"baseRef,omitempty"`
	// ReviewThreadID for thread-level session keys.
	ReviewThreadID int64 `json:"reviewThreadId,omitempty"`
	// DiscussionCategory for discussion events.
	DiscussionCategory string `json:"discussionCategory,omitempty"`
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
			Type:      MessageTypeReviewComment,
			ID:        formatCommentID(e.Comment.ID),
			Text:      e.Comment.Body,
			CreatedAt: e.Comment.CreatedAt,
		},
		Sender: senderFromUser(e.Sender),
		Context: &Context{
			FilePath:       e.Comment.Path,
			Line:           e.Comment.Line,
			ReviewThreadID: e.Comment.PullRequestReviewID,
		},
	}
}

// NormalizeIssueEdited normalizes an issue edited event.
func NormalizeIssueEdited(e *events.IssueEvent) *NormalizedEvent {
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
			Type:      MessageTypeContextUpdate,
			ID:        formatIssueID(e.Repository.FullName, e.Issue.Number) + ":edited",
			Text:      e.Issue.Body,
			CreatedAt: e.Issue.UpdatedAt,
		},
		Sender:  senderFromUser(e.Sender),
		Context: issueContext(e),
	}
}

// NormalizeIssueClosed normalizes an issue closed event.
func NormalizeIssueClosed(e *events.IssueEvent) *NormalizedEvent {
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
			Type:      MessageTypeContextUpdate,
			ID:        formatIssueID(e.Repository.FullName, e.Issue.Number) + ":closed",
			Text:      "Issue closed",
			CreatedAt: e.Issue.UpdatedAt,
		},
		Sender:  senderFromUser(e.Sender),
		Context: issueContext(e),
	}
}

// NormalizeIssueReopened normalizes an issue reopened event.
func NormalizeIssueReopened(e *events.IssueEvent) *NormalizedEvent {
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
			Type:      MessageTypeContextUpdate,
			ID:        formatIssueID(e.Repository.FullName, e.Issue.Number) + ":reopened",
			Text:      "Issue reopened",
			CreatedAt: e.Issue.UpdatedAt,
		},
		Sender:  senderFromUser(e.Sender),
		Context: issueContext(e),
	}
}

// NormalizeIssueLabeled normalizes an issue labeled event.
func NormalizeIssueLabeled(e *events.IssueEvent) *NormalizedEvent {
	labelName := ""
	if e.Label != nil {
		labelName = e.Label.Name
	}
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
			Type:      MessageTypeContextUpdate,
			ID:        formatIssueID(e.Repository.FullName, e.Issue.Number) + ":labeled:" + labelName,
			Text:      "Label added: " + labelName,
			CreatedAt: e.Issue.UpdatedAt,
		},
		Sender:  senderFromUser(e.Sender),
		Context: issueContext(e),
	}
}

// NormalizeIssueCommentEdited normalizes an issue comment edited event.
func NormalizeIssueCommentEdited(e *events.IssueCommentEvent) *NormalizedEvent {
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
			Type:      MessageTypeContextUpdate,
			ID:        formatCommentID(e.Comment.ID) + ":edited",
			Text:      e.Comment.Body,
			CreatedAt: e.Comment.UpdatedAt,
		},
		Sender: senderFromUser(e.Sender),
	}
}

// NormalizePullRequestEdited normalizes a pull request edited event.
func NormalizePullRequestEdited(e *events.PullRequestEvent) *NormalizedEvent {
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
			Type:      MessageTypeContextUpdate,
			ID:        formatPRID(e.Repository.FullName, e.PullRequest.Number) + ":edited",
			Text:      e.PullRequest.Body,
			CreatedAt: e.PullRequest.UpdatedAt,
		},
		Sender:  senderFromUser(e.Sender),
		Context: prContext(e),
	}
}

// NormalizePullRequestClosed normalizes a pull request closed event.
func NormalizePullRequestClosed(e *events.PullRequestEvent) *NormalizedEvent {
	text := "Pull request closed"
	if e.PullRequest.Merged {
		text = "Pull request merged"
	}
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
			Type:      MessageTypeContextUpdate,
			ID:        formatPRID(e.Repository.FullName, e.PullRequest.Number) + ":closed",
			Text:      text,
			CreatedAt: e.PullRequest.UpdatedAt,
		},
		Sender:  senderFromUser(e.Sender),
		Context: prContext(e),
	}
}

// NormalizePullRequestSynchronize normalizes a PR synchronize (new commits pushed) event.
func NormalizePullRequestSynchronize(e *events.PullRequestEvent) *NormalizedEvent {
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
			Type:      MessageTypeContextUpdate,
			ID:        formatPRID(e.Repository.FullName, e.PullRequest.Number) + ":synchronize:" + e.PullRequest.Head.SHA[:8],
			Text:      "New commits pushed to " + e.PullRequest.Head.Ref,
			CreatedAt: e.PullRequest.UpdatedAt,
		},
		Sender:  senderFromUser(e.Sender),
		Context: prContext(e),
	}
}

// NormalizePullRequestLabeled normalizes a PR labeled event.
func NormalizePullRequestLabeled(e *events.PullRequestEvent) *NormalizedEvent {
	labelName := ""
	if e.Label != nil {
		labelName = e.Label.Name
	}
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
			Type:      MessageTypeContextUpdate,
			ID:        formatPRID(e.Repository.FullName, e.PullRequest.Number) + ":labeled:" + labelName,
			Text:      "Label added: " + labelName,
			CreatedAt: e.PullRequest.UpdatedAt,
		},
		Sender:  senderFromUser(e.Sender),
		Context: prContext(e),
	}
}

// NormalizeDiscussionCreated normalizes a discussion created event.
func NormalizeDiscussionCreated(e *events.DiscussionEvent) *NormalizedEvent {
	return &NormalizedEvent{
		Provider:   Provider,
		Repository: e.Repository.FullName,
		Thread: Thread{
			Type:   ThreadTypeDiscussion,
			Number: e.Discussion.Number,
			Title:  e.Discussion.Title,
			URL:    e.Discussion.HTMLURL,
		},
		Message: Message{
			Type:      MessageTypeDiscussionBody,
			ID:        formatDiscussionID(e.Repository.FullName, e.Discussion.Number),
			Text:      e.Discussion.Body,
			CreatedAt: e.Discussion.CreatedAt,
		},
		Sender: senderFromUser(e.Sender),
		Context: &Context{
			DiscussionCategory: e.Discussion.Category.Name,
		},
	}
}

// NormalizeDiscussionComment normalizes a discussion comment event.
func NormalizeDiscussionComment(e *events.DiscussionCommentEvent) *NormalizedEvent {
	return &NormalizedEvent{
		Provider:   Provider,
		Repository: e.Repository.FullName,
		Thread: Thread{
			Type:   ThreadTypeDiscussion,
			Number: e.Discussion.Number,
			Title:  e.Discussion.Title,
			URL:    e.Discussion.HTMLURL,
		},
		Message: Message{
			Type:      MessageTypeComment,
			ID:        formatDiscussionCommentID(e.Comment.ID),
			Text:      e.Comment.Body,
			CreatedAt: e.Comment.CreatedAt,
		},
		Sender: senderFromUser(e.Sender),
		Context: &Context{
			DiscussionCategory: e.Discussion.Category.Name,
		},
	}
}

// NormalizeCheckRun normalizes a check_run event.
func NormalizeCheckRun(e *events.CheckRunEvent) *NormalizedEvent {
	return &NormalizedEvent{
		Provider:   Provider,
		Repository: e.Repository.FullName,
		Thread: Thread{
			Type: ThreadTypePullRequest,
		},
		Message: Message{
			Type:      MessageTypeCIStatus,
			ID:        "check-run-" + itoa(e.CheckRun.ID),
			Text:      fmt.Sprintf("Check run '%s': %s (%s)", e.CheckRun.Name, e.CheckRun.Status, e.CheckRun.Conclusion),
			CreatedAt: e.CheckRun.StartedAt,
		},
		Sender: senderFromUser(e.Sender),
		Context: &Context{
			CIStatus:     e.CheckRun.Status,
			CIConclusion: e.CheckRun.Conclusion,
			EventAction:  string(e.Action),
		},
	}
}

// NormalizeWorkflowRun normalizes a workflow_run event.
func NormalizeWorkflowRun(e *events.WorkflowRunEvent) *NormalizedEvent {
	return &NormalizedEvent{
		Provider:   Provider,
		Repository: e.Repository.FullName,
		Thread: Thread{
			Type: ThreadTypePullRequest,
		},
		Message: Message{
			Type:      MessageTypeCIStatus,
			ID:        "workflow-run-" + itoa(e.WorkflowRun.ID),
			Text:      fmt.Sprintf("Workflow '%s': %s (%s)", e.WorkflowRun.Name, e.WorkflowRun.Status, e.WorkflowRun.Conclusion),
			CreatedAt: e.WorkflowRun.CreatedAt,
		},
		Sender: senderFromUser(e.Sender),
		Context: &Context{
			CIStatus:     e.WorkflowRun.Status,
			CIConclusion: e.WorkflowRun.Conclusion,
			EventAction:  string(e.Action),
		},
	}
}

func issueContext(e *events.IssueEvent) *Context {
	ctx := &Context{
		State:       e.Issue.State,
		EventAction: string(e.Action),
	}
	for _, l := range e.Issue.Labels {
		ctx.Labels = append(ctx.Labels, l.Name)
	}
	for _, a := range e.Issue.Assignees {
		ctx.Assignees = append(ctx.Assignees, a.Login)
	}
	return ctx
}

func prContext(e *events.PullRequestEvent) *Context {
	ctx := &Context{
		State:       e.PullRequest.State,
		Merged:      e.PullRequest.Merged,
		HeadRef:     e.PullRequest.Head.Ref,
		BaseRef:     e.PullRequest.Base.Ref,
		EventAction: string(e.Action),
	}
	for _, l := range e.PullRequest.Labels {
		ctx.Labels = append(ctx.Labels, l.Name)
	}
	for _, a := range e.PullRequest.Assignees {
		ctx.Assignees = append(ctx.Assignees, a.Login)
	}
	return ctx
}

func formatDiscussionID(repo string, number int) string {
	return repo + ":discussion:" + itoa(int64(number))
}

func formatDiscussionCommentID(id int64) string {
	return "discussion-comment-" + itoa(id)
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
