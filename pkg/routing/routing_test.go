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
			name:       "bot type sender with ignoreBots",
			sender:     normalizer.Sender{Login: "app[bot]", IsBot: true},
			ignoreBots: true,
			expected:   true,
		},
		{
			name:     "bot type sender without ignoreBots",
			sender:   normalizer.Sender{Login: "app[bot]", IsBot: true},
			expected: false,
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

func TestEvaluateTrigger_LabelTrigger(t *testing.T) {
	cfg := &config.TriggerConfig{
		Labels: []string{"ai-review", "ai-help"},
	}

	event := &normalizer.NormalizedEvent{
		Message: normalizer.Message{Text: "some text"},
		Context: &normalizer.Context{
			Labels: []string{"bug", "ai-review"},
		},
	}

	result := EvaluateTrigger(cfg, event)
	if !result.Triggered {
		t.Error("expected label trigger to fire")
	}
	if result.Kind != normalizer.TriggerKindLabel {
		t.Errorf("expected trigger kind 'label', got %q", result.Kind)
	}
}

func TestEvaluateTrigger_LabelNoMatch(t *testing.T) {
	cfg := &config.TriggerConfig{
		Labels: []string{"ai-review"},
	}

	event := &normalizer.NormalizedEvent{
		Message: normalizer.Message{Text: "some text"},
		Context: &normalizer.Context{
			Labels: []string{"bug", "documentation"},
		},
	}

	result := EvaluateTrigger(cfg, event)
	if result.Triggered {
		t.Error("expected no trigger match for non-matching labels")
	}
}

func TestEvaluateAutoTrigger_PROpened(t *testing.T) {
	cfg := &config.AutoTriggerConfig{
		OnPROpened: true,
	}

	event := &normalizer.NormalizedEvent{
		Thread:  normalizer.Thread{Type: normalizer.ThreadTypePullRequest},
		Message: normalizer.Message{Type: normalizer.MessageTypePRBody},
	}

	result := EvaluateAutoTrigger(cfg, event)
	if !result.Triggered {
		t.Error("expected auto trigger for PR opened")
	}
	if result.Kind != normalizer.TriggerKindAuto {
		t.Errorf("expected trigger kind 'auto', got %q", result.Kind)
	}
}

func TestEvaluateAutoTrigger_IssueOpened(t *testing.T) {
	cfg := &config.AutoTriggerConfig{
		OnIssueOpened: true,
	}

	event := &normalizer.NormalizedEvent{
		Thread:  normalizer.Thread{Type: normalizer.ThreadTypeIssue},
		Message: normalizer.Message{Type: normalizer.MessageTypeIssueBody},
	}

	result := EvaluateAutoTrigger(cfg, event)
	if !result.Triggered {
		t.Error("expected auto trigger for issue opened")
	}
}

func TestEvaluateAutoTrigger_NotConfigured(t *testing.T) {
	cfg := &config.AutoTriggerConfig{}

	event := &normalizer.NormalizedEvent{
		Thread:  normalizer.Thread{Type: normalizer.ThreadTypePullRequest},
		Message: normalizer.Message{Type: normalizer.MessageTypePRBody},
	}

	result := EvaluateAutoTrigger(cfg, event)
	if result.Triggered {
		t.Error("expected no auto trigger when not configured")
	}
}

func TestSessionKey_ReviewThread(t *testing.T) {
	event := &normalizer.NormalizedEvent{
		Repository: "owner/repo",
		Thread: normalizer.Thread{
			Type:   normalizer.ThreadTypePullRequest,
			Number: 42,
		},
		Context: &normalizer.Context{
			ReviewThreadID: 12345,
		},
	}

	got := SessionKey(event)
	expected := "github:owner/repo:pull_request:42:review-thread:12345"
	if got != expected {
		t.Errorf("SessionKey() = %q, want %q", got, expected)
	}
}

func TestSessionKey_Discussion(t *testing.T) {
	event := &normalizer.NormalizedEvent{
		Repository: "owner/repo",
		Thread: normalizer.Thread{
			Type:   normalizer.ThreadTypeDiscussion,
			Number: 7,
		},
	}

	got := SessionKey(event)
	expected := "github:owner/repo:discussion:7"
	if got != expected {
		t.Errorf("SessionKey() = %q, want %q", got, expected)
	}
}

func TestHasOutboundMarker(t *testing.T) {
	event := &normalizer.NormalizedEvent{
		Message: normalizer.Message{
			Text: "Some reply\n<!-- openclaw-outbound -->",
		},
	}

	if !HasOutboundMarker(event, "<!-- openclaw-outbound -->") {
		t.Error("expected outbound marker to be detected")
	}
	if HasOutboundMarker(event, "") {
		t.Error("expected empty marker to not match")
	}
	if HasOutboundMarker(event, "<!-- different-marker -->") {
		t.Error("expected different marker to not match")
	}
}
