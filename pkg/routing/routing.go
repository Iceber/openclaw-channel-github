// Package routing provides session key generation and trigger matching
// for the OpenClaw GitHub Channel.
package routing

import (
	"fmt"
	"strings"

	"github.com/Iceber/openclaw-channel-github/pkg/config"
	"github.com/Iceber/openclaw-channel-github/pkg/normalizer"
)

// SessionKey generates a stable session key for a normalized event.
// Format: github:<owner>/<repo>:<threadType>:<number>
// For review threads: github:<owner>/<repo>:pull_request:<number>:review-thread:<threadId>
func SessionKey(event *normalizer.NormalizedEvent) string {
	base := fmt.Sprintf("github:%s:%s:%d",
		event.Repository,
		event.Thread.Type,
		event.Thread.Number,
	)
	if event.Context != nil && event.Context.ReviewThreadID != 0 {
		return fmt.Sprintf("%s:review-thread:%d", base, event.Context.ReviewThreadID)
	}
	return base
}

// TriggerResult describes whether and how an event should trigger the agent.
type TriggerResult struct {
	// Triggered indicates whether the event should be processed.
	Triggered bool
	// Kind describes the type of trigger.
	Kind normalizer.TriggerKind
	// Command is the matched command (if trigger kind is "command").
	Command string
}

// EvaluateTrigger checks whether a normalized event matches the trigger configuration.
func EvaluateTrigger(cfg *config.TriggerConfig, event *normalizer.NormalizedEvent) TriggerResult {
	text := event.Message.Text

	// Check @mention
	if cfg.RequireMention && cfg.BotUsername != "" {
		mention := "@" + cfg.BotUsername
		if containsIgnoreCase(text, mention) {
			return TriggerResult{Triggered: true, Kind: normalizer.TriggerKindMention}
		}
	}

	// Check command prefixes
	for _, cmd := range cfg.Commands {
		if hasCommandPrefix(text, cmd) {
			return TriggerResult{Triggered: true, Kind: normalizer.TriggerKindCommand, Command: cmd}
		}
	}

	// Check label triggers from context
	if len(cfg.Labels) > 0 && event.Context != nil {
		for _, cfgLabel := range cfg.Labels {
			for _, evtLabel := range event.Context.Labels {
				if strings.EqualFold(cfgLabel, evtLabel) {
					return TriggerResult{Triggered: true, Kind: normalizer.TriggerKindLabel}
				}
			}
		}
	}

	// If mention is not required and no commands are configured, treat as auto-trigger
	if !cfg.RequireMention && len(cfg.Commands) == 0 && len(cfg.Labels) == 0 {
		return TriggerResult{Triggered: true, Kind: normalizer.TriggerKindAuto}
	}

	return TriggerResult{Triggered: false}
}

// EvaluateAutoTrigger checks if an event qualifies for auto-trigger based on config.
func EvaluateAutoTrigger(cfg *config.AutoTriggerConfig, event *normalizer.NormalizedEvent) TriggerResult {
	if cfg.OnPROpened && event.Thread.Type == normalizer.ThreadTypePullRequest && event.Message.Type == normalizer.MessageTypePRBody {
		return TriggerResult{Triggered: true, Kind: normalizer.TriggerKindAuto}
	}
	if cfg.OnIssueOpened && event.Thread.Type == normalizer.ThreadTypeIssue && event.Message.Type == normalizer.MessageTypeIssueBody {
		return TriggerResult{Triggered: true, Kind: normalizer.TriggerKindAuto}
	}
	return TriggerResult{Triggered: false}
}

// containsIgnoreCase checks if s contains substr, case-insensitively.
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// hasCommandPrefix checks if text starts with a command prefix,
// allowing for leading whitespace.
func hasCommandPrefix(text, cmd string) bool {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	cmdLower := strings.ToLower(cmd)

	if strings.HasPrefix(lower, cmdLower) {
		// Ensure the command is followed by space, newline, or end of string
		rest := trimmed[len(cmd):]
		return rest == "" || rest[0] == ' ' || rest[0] == '\n' || rest[0] == '\t'
	}
	return false
}

// IsBotSender checks whether the sender should be ignored (bot loop prevention).
func IsBotSender(event *normalizer.NormalizedEvent, botUsername string, ignoreBots bool) bool {
	if ignoreBots && event.Sender.IsBot {
		return true
	}
	if botUsername != "" && strings.EqualFold(event.Sender.Login, botUsername) {
		return true
	}
	return false
}

// HasOutboundMarker checks if the event message contains the outbound marker.
func HasOutboundMarker(event *normalizer.NormalizedEvent, marker string) bool {
	if marker == "" {
		return false
	}
	return strings.Contains(event.Message.Text, marker)
}
