// Package server provides the HTTP webhook receiver for the OpenClaw GitHub Channel.
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/Iceber/openclaw-channel-github/pkg/auth"
	"github.com/Iceber/openclaw-channel-github/pkg/config"
	"github.com/Iceber/openclaw-channel-github/pkg/events"
	"github.com/Iceber/openclaw-channel-github/pkg/normalizer"
	"github.com/Iceber/openclaw-channel-github/pkg/outbound"
	"github.com/Iceber/openclaw-channel-github/pkg/routing"
	"github.com/Iceber/openclaw-channel-github/pkg/state"
)

// maxPayloadSize is the maximum allowed webhook payload size (10 MB).
const maxPayloadSize = 10 * 1024 * 1024

// Handler processes GitHub webhook events.
type Handler struct {
	cfg      *config.Config
	ghAuth   *auth.GitHubAuth
	store    *state.Store
	sender   *outbound.Sender
	logger   *slog.Logger

	// MessageHandler is the callback invoked when a valid, triggered event is received.
	// If nil, the handler will log the event but not process it further.
	MessageHandler func(sessionKey string, event *normalizer.NormalizedEvent) (string, error)
}

// NewHandler creates a new webhook Handler.
func NewHandler(cfg *config.Config, ghAuth *auth.GitHubAuth, store *state.Store, sender *outbound.Sender, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		cfg:    cfg,
		ghAuth: ghAuth,
		store:  store,
		sender: sender,
		logger: logger,
	}
}

// ServeHTTP handles incoming webhook requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the payload
	body, err := io.ReadAll(io.LimitReader(r.Body, maxPayloadSize))
	if err != nil {
		h.logger.Error("failed to read request body", "error", err)
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// Verify webhook signature
	signature := auth.ExtractSignature(r)
	if !auth.VerifyWebhookSignature(body, h.cfg.Channel.WebhookSecret, signature) {
		h.logger.Warn("invalid webhook signature")
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Extract delivery ID and event type
	deliveryID := auth.ExtractDeliveryID(r)
	eventType := events.EventType(auth.ExtractEventType(r))

	h.logger.Info("received webhook",
		"delivery_id", deliveryID,
		"event_type", eventType,
	)

	// Check for duplicate delivery
	if h.store.IsDuplicate(deliveryID) {
		h.logger.Info("duplicate delivery, skipping", "delivery_id", deliveryID)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"duplicate"}`)
		return
	}

	// Parse the event
	parsed, err := events.ParseEvent(eventType, body)
	if err != nil {
		h.logger.Warn("unsupported or unparseable event",
			"event_type", eventType,
			"error", err,
		)
		// Return 200 for unsupported events to avoid GitHub retries
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"unsupported_event"}`)
		return
	}

	// Normalize the event
	normalized := h.normalizeEvent(eventType, parsed)
	if normalized == nil {
		h.logger.Info("event not normalizable, skipping", "event_type", eventType)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"skipped"}`)
		return
	}

	// Check repository allowlist
	if !h.cfg.IsRepoAllowed(normalized.Repository) {
		h.logger.Warn("repository not in allowlist", "repo", normalized.Repository)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"repo_not_allowed"}`)
		return
	}

	// Bot loop prevention
	if routing.IsBotSender(normalized, h.cfg.Channel.Trigger.BotUsername, h.cfg.Channel.IgnoreBots) {
		h.logger.Info("ignoring bot sender", "sender", normalized.Sender.Login)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"bot_ignored"}`)
		return
	}

	// Outbound marker detection (bot loop prevention via marker)
	if routing.HasOutboundMarker(normalized, h.cfg.Channel.Outbound.OutboundMarker) {
		h.logger.Info("ignoring outbound-marked message", "sender", normalized.Sender.Login)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"outbound_marker_ignored"}`)
		return
	}

	// Evaluate trigger
	triggerResult := routing.EvaluateTrigger(&h.cfg.Channel.Trigger, normalized)

	// Check auto-trigger if regular trigger didn't match
	if !triggerResult.Triggered {
		triggerResult = routing.EvaluateAutoTrigger(&h.cfg.Channel.AutoTrigger, normalized)
	}
	if !triggerResult.Triggered {
		h.logger.Info("trigger not matched, skipping",
			"repo", normalized.Repository,
			"thread", normalized.Thread.Number,
		)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"no_trigger"}`)
		return
	}

	// Set trigger info on the normalized event
	normalized.Trigger = normalizer.Trigger{
		Kind:    triggerResult.Kind,
		Command: triggerResult.Command,
	}

	// Generate session key
	sessionKey := routing.SessionKey(normalized)

	h.logger.Info("processing event",
		"session_key", sessionKey,
		"trigger_kind", triggerResult.Kind,
		"repo", normalized.Repository,
		"thread_number", normalized.Thread.Number,
	)

	// Mark delivery as processed
	h.store.MarkProcessed(deliveryID)

	// Invoke message handler if set
	if h.MessageHandler != nil {
		reply, err := h.MessageHandler(sessionKey, normalized)
		if err != nil {
			h.logger.Error("message handler failed",
				"session_key", sessionKey,
				"error", err,
			)
			http.Error(w, "Internal processing error", http.StatusInternalServerError)
			return
		}

		// Send reply back to GitHub if non-empty
		if reply != "" {
			token := h.ghAuth.GetInstallationToken()
			if token == "" {
				h.logger.Error("no valid installation token available")
				http.Error(w, "Authentication error", http.StatusInternalServerError)
				return
			}

			if err := h.sender.SendComment(token, normalized, reply); err != nil {
				h.logger.Error("failed to send reply",
					"session_key", sessionKey,
					"error", err,
				)
				http.Error(w, "Failed to send reply", http.StatusInternalServerError)
				return
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	resp := map[string]string{
		"status":      "processed",
		"session_key": sessionKey,
	}
	json.NewEncoder(w).Encode(resp)
}

// normalizeEvent converts a parsed GitHub event to a normalized event.
func (h *Handler) normalizeEvent(eventType events.EventType, parsed any) *normalizer.NormalizedEvent {
	switch eventType {
	case events.EventIssues:
		e := parsed.(*events.IssueEvent)
		switch e.Action {
		case events.ActionOpened:
			return normalizer.NormalizeIssueOpened(e)
		case events.ActionEdited:
			return normalizer.NormalizeIssueEdited(e)
		case events.ActionClosed:
			return normalizer.NormalizeIssueClosed(e)
		case events.ActionReopened:
			return normalizer.NormalizeIssueReopened(e)
		case events.ActionLabeled:
			return normalizer.NormalizeIssueLabeled(e)
		}

	case events.EventIssueComment:
		e := parsed.(*events.IssueCommentEvent)
		switch e.Action {
		case events.ActionCreated:
			return normalizer.NormalizeIssueComment(e)
		case events.ActionEdited:
			return normalizer.NormalizeIssueCommentEdited(e)
		}

	case events.EventPullRequest:
		e := parsed.(*events.PullRequestEvent)
		switch e.Action {
		case events.ActionOpened:
			return normalizer.NormalizePullRequestOpened(e)
		case events.ActionEdited:
			return normalizer.NormalizePullRequestEdited(e)
		case events.ActionClosed:
			return normalizer.NormalizePullRequestClosed(e)
		case events.ActionSynchronize:
			return normalizer.NormalizePullRequestSynchronize(e)
		case events.ActionLabeled:
			return normalizer.NormalizePullRequestLabeled(e)
		}

	case events.EventPullRequestReview:
		e := parsed.(*events.PullRequestReviewEvent)
		if e.Action == events.ActionSubmitted {
			return normalizer.NormalizePullRequestReview(e)
		}

	case events.EventPullRequestReviewComment:
		e := parsed.(*events.PullRequestReviewCommentEvent)
		if e.Action == events.ActionCreated {
			return normalizer.NormalizePullRequestReviewComment(e)
		}

	case events.EventDiscussion:
		e := parsed.(*events.DiscussionEvent)
		if e.Action == events.ActionCreated {
			return normalizer.NormalizeDiscussionCreated(e)
		}

	case events.EventDiscussionComment:
		e := parsed.(*events.DiscussionCommentEvent)
		if e.Action == events.ActionCreated {
			return normalizer.NormalizeDiscussionComment(e)
		}

	case events.EventCheckRun:
		e := parsed.(*events.CheckRunEvent)
		if e.Action == events.ActionCompleted {
			return normalizer.NormalizeCheckRun(e)
		}

	case events.EventWorkflowRun:
		e := parsed.(*events.WorkflowRunEvent)
		if e.Action == events.ActionCompleted {
			return normalizer.NormalizeWorkflowRun(e)
		}
	}
	return nil
}

// NewMux creates an HTTP mux with the webhook handler and health endpoints.
func NewMux(handler *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/webhook", handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})
	return mux
}
