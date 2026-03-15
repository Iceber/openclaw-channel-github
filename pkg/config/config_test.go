package config

import (
	"testing"
)

func TestParseValidConfig(t *testing.T) {
	data := []byte(`{
		"server": {"addr": ":9090"},
		"channel": {
			"enabled": true,
			"mode": "app",
			"appId": 12345,
			"installationId": 67890,
			"privateKeyPath": "/path/to/key.pem",
			"webhookSecret": "secret123",
			"repositories": ["owner/repo"],
			"trigger": {
				"requireMention": true,
				"botUsername": "openclaw-bot",
				"commands": ["/openclaw"]
			},
			"ignoreBots": true
		}
	}`)

	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Addr != ":9090" {
		t.Errorf("expected addr :9090, got %s", cfg.Server.Addr)
	}
	if !cfg.Channel.Enabled {
		t.Error("expected channel to be enabled")
	}
	if cfg.Channel.AppID != 12345 {
		t.Errorf("expected appId 12345, got %d", cfg.Channel.AppID)
	}
	if cfg.Channel.InstallationID != 67890 {
		t.Errorf("expected installationId 67890, got %d", cfg.Channel.InstallationID)
	}
	if !cfg.Channel.Trigger.RequireMention {
		t.Error("expected requireMention to be true")
	}
	if cfg.Channel.Trigger.BotUsername != "openclaw-bot" {
		t.Errorf("expected botUsername 'openclaw-bot', got %s", cfg.Channel.Trigger.BotUsername)
	}
}

func TestParseDisabledChannel(t *testing.T) {
	data := []byte(`{
		"channel": {"enabled": false}
	}`)

	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Channel.Enabled {
		t.Error("expected channel to be disabled")
	}
	// Default addr should be set
	if cfg.Server.Addr != ":8080" {
		t.Errorf("expected default addr :8080, got %s", cfg.Server.Addr)
	}
}

func TestValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "missing mode",
			json: `{"channel":{"enabled":true,"webhookSecret":"s","repositories":["o/r"]}}`,
		},
		{
			name: "missing appId for app mode",
			json: `{"channel":{"enabled":true,"mode":"app","installationId":1,"privateKeyPath":"/k","webhookSecret":"s","repositories":["o/r"]}}`,
		},
		{
			name: "missing webhookSecret",
			json: `{"channel":{"enabled":true,"mode":"app","appId":1,"installationId":1,"privateKeyPath":"/k","repositories":["o/r"]}}`,
		},
		{
			name: "empty repositories",
			json: `{"channel":{"enabled":true,"mode":"app","appId":1,"installationId":1,"privateKeyPath":"/k","webhookSecret":"s","repositories":[]}}`,
		},
		{
			name: "invalid repo format",
			json: `{"channel":{"enabled":true,"mode":"app","appId":1,"installationId":1,"privateKeyPath":"/k","webhookSecret":"s","repositories":["noslash"]}}`,
		},
		{
			name: "requireMention without botUsername",
			json: `{"channel":{"enabled":true,"mode":"app","appId":1,"installationId":1,"privateKeyPath":"/k","webhookSecret":"s","repositories":["o/r"],"trigger":{"requireMention":true}}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.json))
			if err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

func TestIsRepoAllowed(t *testing.T) {
	cfg := &Config{
		Channel: ChannelConfig{
			Repositories: []string{"owner/repo", "Org/Project"},
		},
	}

	tests := []struct {
		repo    string
		allowed bool
	}{
		{"owner/repo", true},
		{"Owner/Repo", true},  // case-insensitive
		{"Org/Project", true},
		{"org/project", true}, // case-insensitive
		{"other/repo", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.repo, func(t *testing.T) {
			if got := cfg.IsRepoAllowed(tc.repo); got != tc.allowed {
				t.Errorf("IsRepoAllowed(%q) = %v, want %v", tc.repo, got, tc.allowed)
			}
		})
	}
}
