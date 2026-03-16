// Package outbound provides functionality for sending messages back to GitHub.
package outbound

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Iceber/openclaw-channel-github/pkg/normalizer"
)

const (
	// GitHubAPIBaseURL is the base URL for GitHub REST API.
	GitHubAPIBaseURL = "https://api.github.com"

	// MaxCommentLength is the maximum length for a GitHub comment.
	MaxCommentLength = 65536

	// OutboundMarkerPrefix is the default hidden marker for bot loop prevention.
	OutboundMarkerPrefix = "<!-- openclaw-outbound -->"
)

// Sender sends messages back to GitHub.
type Sender struct {
	client         *http.Client
	baseURL        string
	outboundMarker string
}

// NewSender creates a new outbound Sender.
func NewSender(client *http.Client, baseURL string) *Sender {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if baseURL == "" {
		baseURL = GitHubAPIBaseURL
	}
	return &Sender{client: client, baseURL: baseURL, outboundMarker: OutboundMarkerPrefix}
}

// SetOutboundMarker sets a custom outbound marker for bot loop prevention.
func (s *Sender) SetOutboundMarker(marker string) {
	s.outboundMarker = marker
}

// HasOutboundMarker checks if text contains the outbound marker.
func (s *Sender) HasOutboundMarker(text string) bool {
	return s.outboundMarker != "" && strings.Contains(text, s.outboundMarker)
}

// CommentRequest is the request body for creating a comment.
type CommentRequest struct {
	Body string `json:"body"`
}

// ReviewRequest is the request body for creating a PR review.
type ReviewRequest struct {
	Body     string          `json:"body"`
	Event    string          `json:"event"` // "APPROVE", "COMMENT", "REQUEST_CHANGES"
	Comments []ReviewComment `json:"comments,omitempty"`
}

// ReviewComment is an inline comment on a PR review.
type ReviewComment struct {
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`
	Side string `json:"side,omitempty"`
	Body string `json:"body"`
}

// wrapWithMarker adds the outbound marker to the body.
func (s *Sender) wrapWithMarker(body string) string {
	if s.outboundMarker == "" {
		return body
	}
	return body + "\n" + s.outboundMarker
}

// SendComment sends a comment to the appropriate issue or PR thread.
func (s *Sender) SendComment(token string, event *normalizer.NormalizedEvent, body string) error {
	body = s.wrapWithMarker(body)
	if len(body) > MaxCommentLength {
		body = body[:MaxCommentLength-3] + "..."
	}

	url := fmt.Sprintf("%s/repos/%s/issues/%d/comments",
		s.baseURL,
		event.Repository,
		event.Thread.Number,
	)

	return s.doPost(token, url, CommentRequest{Body: body})
}

// SendReview creates a PR review with the given event, body, and review action.
// action should be "APPROVE", "COMMENT", or "REQUEST_CHANGES".
func (s *Sender) SendReview(token string, repo string, prNumber int, body string, action string, comments []ReviewComment) error {
	body = s.wrapWithMarker(body)
	if len(body) > MaxCommentLength {
		body = body[:MaxCommentLength-3] + "..."
	}

	url := fmt.Sprintf("%s/repos/%s/pulls/%d/reviews",
		s.baseURL,
		repo,
		prNumber,
	)

	reqBody := ReviewRequest{
		Body:     body,
		Event:    action,
		Comments: comments,
	}

	return s.doPost(token, url, reqBody)
}

// AddReaction adds a reaction to a comment.
func (s *Sender) AddReaction(token string, repo string, commentID int64, reaction string) error {
	url := fmt.Sprintf("%s/repos/%s/issues/comments/%d/reactions",
		s.baseURL,
		repo,
		commentID,
	)

	return s.doPost(token, url, map[string]string{"content": reaction})
}

// doPost performs an authenticated POST request.
func (s *Sender) doPost(token string, url string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}
