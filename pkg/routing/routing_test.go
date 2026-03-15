package routing

import (
	"testing"

	"github.com/Iceber/openclaw-channel-github/pkg/config"
	"github.com/Iceber/openclaw-channel-github/pkg/normalizer"
)

func TestSessionKey(t *testing.T) {
	tests := []struct {
		name     string
		event    *normalizer.NormalizedEvent
		expected string
	}{
		{
			name: "issue session key",
			event: &normalizer.NormalizedEvent{
				Repository: "owner/repo",
				Thread: normalizer.Thread{
					Type:   normalizer.ThreadTypeIssue,
					Number: 42,
				},
			},
			expected: "github:owner/repo:issue:42",
		},
		{
			name: "pull request session key",
			event: &normalizer.NormalizedEvent{
				Repository: "org/project",
				Thread: normalizer.Thread{
					Type:   normalizer.ThreadTypePullRequest,
					Number: 100,
				},
			},
			expected: "github:org/project:pull_request:100",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SessionKey(tc.event)
			if got != tc.expected {
				t.Errorf("SessionKey() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestEvaluateTrigger_Mention(t *testing.T) {
	cfg := &config.TriggerConfig{
		RequireMention: true,
		BotUsername:    "openclaw-bot",
	}

	event := &normalizer.NormalizedEvent{
		Message: normalizer.Message{Text: "Hey @openclaw-bot can you help?"},
	}

	result := EvaluateTrigger(cfg, event)
	if !result.Triggered {
		t.Error("expected mention trigger to fire")
	}
	if result.Kind != normalizer.TriggerKindMention {
		t.Errorf("expected trigger kind 'mention', got %q", result.Kind)
	}
}

func TestEvaluateTrigger_MentionCaseInsensitive(t *testing.T) {
	cfg := &config.TriggerConfig{
		RequireMention: true,
		BotUsername:    "OpenClaw-Bot",
	}

	event := &normalizer.NormalizedEvent{
		Message: normalizer.Message{Text: "Hey @openclaw-bot please review"},
	}

	result := EvaluateTrigger(cfg, event)
	if !result.Triggered {
		t.Error("expected case-insensitive mention trigger to fire")
	}
}

func TestEvaluateTrigger_Command(t *testing.T) {
	cfg := &config.TriggerConfig{
		Commands: []string{"/openclaw"},
	}

	event := &normalizer.NormalizedEvent{
		Message: normalizer.Message{Text: "/openclaw summarize this PR"},
	}

	result := EvaluateTrigger(cfg, event)
	if !result.Triggered {
		t.Error("expected command trigger to fire")
	}
	if result.Kind != normalizer.TriggerKindCommand {
		t.Errorf("expected trigger kind 'command', got %q", result.Kind)
	}
	if result.Command != "/openclaw" {
		t.Errorf("expected command '/openclaw', got %q", result.Command)
	}
}

func TestEvaluateTrigger_NoMatch(t *testing.T) {
	cfg := &config.TriggerConfig{
		RequireMention: true,
		BotUsername:    "openclaw-bot",
		Commands:       []string{"/openclaw"},
	}

	event := &normalizer.NormalizedEvent{
		Message: normalizer.Message{Text: "Just a regular comment"},
	}

	result := EvaluateTrigger(cfg, event)
	if result.Triggered {
		t.Error("expected no trigger match")
	}
}

func TestEvaluateTrigger_AutoTrigger(t *testing.T) {
	cfg := &config.TriggerConfig{
		RequireMention: false,
	}

	event := &normalizer.NormalizedEvent{
		Message: normalizer.Message{Text: "Any message should trigger"},
	}

	result := EvaluateTrigger(cfg, event)
	if !result.Triggered {
		t.Error("expected auto trigger to fire")
	}
	if result.Kind != normalizer.TriggerKindAuto {
		t.Errorf("expected trigger kind 'auto', got %q", result.Kind)
	}
}

func TestIsBotSender(t *testing.T) {
	tests := []struct {
		name        string
		sender      normalizer.Sender
		botUsername string
		ignoreBots  bool
		expected    bool
	}{
		{
			name:     "bot type sender",
			sender:   normalizer.Sender{Login: "app[bot]", IsBot: true},
			expected: true,
		},
		{
			name:        "matching bot username",
			sender:      normalizer.Sender{Login: "openclaw-bot", IsBot: false},
			botUsername:  "openclaw-bot",
			expected:    true,
		},
		{
			name:        "case insensitive bot match",
			sender:      normalizer.Sender{Login: "OpenClaw-Bot", IsBot: false},
			botUsername:  "openclaw-bot",
			expected:    true,
		},
		{
			name:     "regular user",
			sender:   normalizer.Sender{Login: "alice", IsBot: false},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			event := &normalizer.NormalizedEvent{Sender: tc.sender}
			got := IsBotSender(event, tc.botUsername, tc.ignoreBots)
			if got != tc.expected {
				t.Errorf("IsBotSender() = %v, want %v", got, tc.expected)
			}
		})
	}
}
