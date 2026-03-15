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
func SessionKey(event *normalizer.NormalizedEvent) string {
	return fmt.Sprintf("github:%s:%s:%d",
		event.Repository,
		event.Thread.Type,
		event.Thread.Number,
	)
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

	// If mention is not required and no commands are configured, treat as auto-trigger
	if !cfg.RequireMention && len(cfg.Commands) == 0 && len(cfg.Labels) == 0 {
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
	if event.Sender.IsBot {
		return true
	}
	if ignoreBots && event.Sender.IsBot {
		return true
	}
	if botUsername != "" && strings.EqualFold(event.Sender.Login, botUsername) {
		return true
	}
	return false
}
