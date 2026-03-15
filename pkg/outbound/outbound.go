// Package outbound provides functionality for sending messages back to GitHub.
package outbound

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Iceber/openclaw-channel-github/pkg/normalizer"
)

const (
	// GitHubAPIBaseURL is the base URL for GitHub REST API.
	GitHubAPIBaseURL = "https://api.github.com"

	// MaxCommentLength is the maximum length for a GitHub comment.
	MaxCommentLength = 65536
)

// Sender sends messages back to GitHub.
type Sender struct {
	client  *http.Client
	baseURL string
}

// NewSender creates a new outbound Sender.
func NewSender(client *http.Client, baseURL string) *Sender {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if baseURL == "" {
		baseURL = GitHubAPIBaseURL
	}
	return &Sender{client: client, baseURL: baseURL}
}

// CommentRequest is the request body for creating a comment.
type CommentRequest struct {
	Body string `json:"body"`
}

// SendComment sends a comment to the appropriate issue or PR thread.
func (s *Sender) SendComment(token string, event *normalizer.NormalizedEvent, body string) error {
	if len(body) > MaxCommentLength {
		body = body[:MaxCommentLength-3] + "..."
	}

	url := fmt.Sprintf("%s/repos/%s/issues/%d/comments",
		s.baseURL,
		event.Repository,
		event.Thread.Number,
	)

	reqBody := CommentRequest{Body: body}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling comment request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating comment request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// AddReaction adds a reaction to a comment.
func (s *Sender) AddReaction(token string, repo string, commentID int64, reaction string) error {
	url := fmt.Sprintf("%s/repos/%s/issues/comments/%d/reactions",
		s.baseURL,
		repo,
		commentID,
	)

	reqBody := map[string]string{"content": reaction}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling reaction request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating reaction request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("adding reaction: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}
