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

func TestIsRepoAllowedMultiAccount(t *testing.T) {
	cfg := &Config{
		Channel: ChannelConfig{
			Enabled: true,
			Accounts: map[string]*AccountConfig{
				"acct1": {
					Mode:           "token",
					WebhookSecret:  "s1",
					Repositories:   []string{"org1/repo1"},
				},
				"acct2": {
					Mode:           "token",
					WebhookSecret:  "s2",
					Repositories:   []string{"org2/repo2"},
				},
			},
		},
	}

	if !cfg.IsRepoAllowed("org1/repo1") {
		t.Error("expected org1/repo1 to be allowed via acct1")
	}
	if !cfg.IsRepoAllowed("Org2/Repo2") {
		t.Error("expected Org2/Repo2 to be allowed via acct2 (case-insensitive)")
	}
	if cfg.IsRepoAllowed("other/repo") {
		t.Error("expected other/repo to be disallowed")
	}
}

func TestMultiAccountValidation(t *testing.T) {
	// Valid multi-account config.
	data := []byte(`{
		"channel": {
			"enabled": true,
			"accounts": {
				"production": {
					"mode": "app",
					"appId": 111,
					"installationId": 222,
					"privateKeyPath": "/key.pem",
					"webhookSecret": "sec1",
					"repositories": ["org/prod-repo"]
				},
				"staging": {
					"mode": "token",
					"webhookSecret": "sec2",
					"repositories": ["org/staging-repo"]
				}
			}
		}
	}`)

	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Channel.Accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(cfg.Channel.Accounts))
	}
}

func TestMultiAccountValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "account missing mode",
			json: `{"channel":{"enabled":true,"accounts":{"a1":{"webhookSecret":"s","repositories":["o/r"]}}}}`,
		},
		{
			name: "account missing webhookSecret",
			json: `{"channel":{"enabled":true,"accounts":{"a1":{"mode":"token","repositories":["o/r"]}}}}`,
		},
		{
			name: "account empty repositories",
			json: `{"channel":{"enabled":true,"accounts":{"a1":{"mode":"token","webhookSecret":"s","repositories":[]}}}}`,
		},
		{
			name: "account invalid repo format",
			json: `{"channel":{"enabled":true,"accounts":{"a1":{"mode":"token","webhookSecret":"s","repositories":["noslash"]}}}}`,
		},
		{
			name: "account app mode missing appId",
			json: `{"channel":{"enabled":true,"accounts":{"a1":{"mode":"app","installationId":1,"privateKeyPath":"/k","webhookSecret":"s","repositories":["o/r"]}}}}`,
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

func TestGetAccountFlatConfig(t *testing.T) {
	cfg := &Config{
		Channel: ChannelConfig{
			Mode:           "app",
			AppID:          100,
			InstallationID: 200,
			PrivateKeyPath: "/key.pem",
			WebhookSecret:  "secret",
			Repositories:   []string{"owner/repo"},
		},
	}

	// With no accounts map, any name returns the flat config as default.
	acct := cfg.GetAccount("anything")
	if acct == nil {
		t.Fatal("expected non-nil default account")
	}
	if acct.AppID != 100 {
		t.Errorf("expected AppID 100, got %d", acct.AppID)
	}
	if acct.Mode != "app" {
		t.Errorf("expected mode 'app', got %s", acct.Mode)
	}
}

func TestGetAccountMultiAccount(t *testing.T) {
	cfg := &Config{
		Channel: ChannelConfig{
			Accounts: map[string]*AccountConfig{
				"prod": {Mode: "app", AppID: 1},
				"dev":  {Mode: "token"},
			},
		},
	}

	prod := cfg.GetAccount("prod")
	if prod == nil || prod.AppID != 1 {
		t.Error("expected prod account with AppID 1")
	}

	dev := cfg.GetAccount("dev")
	if dev == nil || dev.Mode != "token" {
		t.Error("expected dev account with mode 'token'")
	}

	missing := cfg.GetAccount("nonexistent")
	if missing != nil {
		t.Error("expected nil for nonexistent account")
	}
}

func TestGetAccountForRepoFlat(t *testing.T) {
	cfg := &Config{
		Channel: ChannelConfig{
			Mode:           "token",
			WebhookSecret:  "s",
			Repositories:   []string{"owner/repo"},
		},
	}

	name, acct := cfg.GetAccountForRepo("owner/repo")
	if name != "default" || acct == nil {
		t.Fatalf("expected default account for owner/repo, got name=%q acct=%v", name, acct)
	}
	if acct.Mode != "token" {
		t.Errorf("expected mode 'token', got %s", acct.Mode)
	}

	name, acct = cfg.GetAccountForRepo("other/repo")
	if name != "" || acct != nil {
		t.Error("expected no account for unmanaged repo")
	}
}

func TestGetAccountForRepoMultiAccount(t *testing.T) {
	cfg := &Config{
		Channel: ChannelConfig{
			Accounts: map[string]*AccountConfig{
				"acct1": {
					Mode:          "token",
					WebhookSecret: "s1",
					Repositories:  []string{"org1/repo1"},
				},
				"acct2": {
					Mode:          "app",
					WebhookSecret: "s2",
					Repositories:  []string{"org2/repo2", "org2/repo3"},
				},
			},
		},
	}

	name, acct := cfg.GetAccountForRepo("org2/repo3")
	if name != "acct2" || acct == nil {
		t.Fatalf("expected acct2 for org2/repo3, got name=%q", name)
	}

	name, acct = cfg.GetAccountForRepo("Org1/Repo1")
	if name != "acct1" || acct == nil {
		t.Fatalf("expected acct1 for Org1/Repo1 (case-insensitive), got name=%q", name)
	}

	name, acct = cfg.GetAccountForRepo("unknown/repo")
	if name != "" || acct != nil {
		t.Error("expected no account for unknown repo")
	}
}

func TestOutboundValidation(t *testing.T) {
	valid := []string{"comment", "review", "auto", ""}
	for _, m := range valid {
		t.Run("valid_"+m, func(t *testing.T) {
			data := []byte(`{"channel":{"enabled":true,"mode":"token","webhookSecret":"s","repositories":["o/r"],"outbound":{"mode":"` + m + `"}}}`)
			if _, err := Parse(data); err != nil {
				t.Errorf("unexpected error for outbound mode %q: %v", m, err)
			}
		})
	}

	t.Run("invalid mode", func(t *testing.T) {
		data := []byte(`{"channel":{"enabled":true,"mode":"token","webhookSecret":"s","repositories":["o/r"],"outbound":{"mode":"invalid"}}}`)
		_, err := Parse(data)
		if err == nil {
			t.Error("expected validation error for invalid outbound mode")
		}
	})
}

func TestRateLimitValidation(t *testing.T) {
	t.Run("valid positive", func(t *testing.T) {
		data := []byte(`{"channel":{"enabled":true,"mode":"token","webhookSecret":"s","repositories":["o/r"],"rateLimit":{"maxEventsPerMinute":60}}}`)
		cfg, err := Parse(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Channel.RateLimit.MaxEventsPerMinute != 60 {
			t.Errorf("expected 60, got %d", cfg.Channel.RateLimit.MaxEventsPerMinute)
		}
	})

	t.Run("negative value", func(t *testing.T) {
		data := []byte(`{"channel":{"enabled":true,"mode":"token","webhookSecret":"s","repositories":["o/r"],"rateLimit":{"maxEventsPerMinute":-1}}}`)
		_, err := Parse(data)
		if err == nil {
			t.Error("expected validation error for negative rate limit")
		}
	})
}

func TestAutoTriggerParsing(t *testing.T) {
	data := []byte(`{
		"channel": {
			"enabled": true,
			"mode": "token",
			"webhookSecret": "s",
			"repositories": ["o/r"],
			"autoTrigger": {
				"onPROpened": true,
				"onIssueOpened": true
			}
		}
	}`)

	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Channel.AutoTrigger.OnPROpened {
		t.Error("expected OnPROpened to be true")
	}
	if !cfg.Channel.AutoTrigger.OnIssueOpened {
		t.Error("expected OnIssueOpened to be true")
	}
}

func TestOutboundMarkerParsing(t *testing.T) {
	data := []byte(`{
		"channel": {
			"enabled": true,
			"mode": "token",
			"webhookSecret": "s",
			"repositories": ["o/r"],
			"outbound": {
				"mode": "comment",
				"outboundMarker": "<!-- bot-marker -->"
			}
		}
	}`)

	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Channel.Outbound.Mode != "comment" {
		t.Errorf("expected outbound mode 'comment', got %s", cfg.Channel.Outbound.Mode)
	}
	if cfg.Channel.Outbound.OutboundMarker != "<!-- bot-marker -->" {
		t.Errorf("expected outbound marker, got %s", cfg.Channel.Outbound.OutboundMarker)
	}
}
