// Package config provides configuration types and loading for the OpenClaw GitHub Channel.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Config is the top-level configuration for the GitHub Channel.
type Config struct {
	// Server holds the HTTP server configuration.
	Server ServerConfig `json:"server"`
	// Channel holds the GitHub channel-specific configuration.
	Channel ChannelConfig `json:"channel"`
}

// ServerConfig defines the HTTP server settings.
type ServerConfig struct {
	// Addr is the address to listen on, e.g. ":8080".
	Addr string `json:"addr"`
}

// ChannelConfig defines the GitHub channel settings.
type ChannelConfig struct {
	// Enabled controls whether the channel is active.
	Enabled bool `json:"enabled"`
	// Mode is the authentication mode: "app" or "token".
	Mode string `json:"mode"`
	// AppID is the GitHub App ID (required when mode is "app").
	AppID int64 `json:"appId"`
	// InstallationID is the GitHub App installation ID.
	InstallationID int64 `json:"installationId"`
	// PrivateKeyPath is the path to the GitHub App private key PEM file.
	PrivateKeyPath string `json:"privateKeyPath"`
	// WebhookSecret is the secret used to verify webhook signatures.
	WebhookSecret string `json:"webhookSecret"`
	// Repositories is the allowlist of repositories in "owner/repo" format.
	Repositories []string `json:"repositories"`
	// Trigger holds the trigger configuration.
	Trigger TriggerConfig `json:"trigger"`
	// IgnoreBots controls whether to ignore events from bot accounts.
	IgnoreBots bool `json:"ignoreBots"`

	// Accounts maps account names to their configurations for multi-installation support (Phase 4).
	// When empty, the flat fields above serve as the default account.
	Accounts map[string]*AccountConfig `json:"accounts,omitempty"`
	// RateLimit holds rate limiting configuration (Phase 3).
	RateLimit RateLimitConfig `json:"rateLimit,omitempty"`
	// Outbound holds outbound response configuration (Phase 2).
	Outbound OutboundConfig `json:"outbound,omitempty"`
	// AutoTrigger holds auto-trigger configuration (Phase 2).
	AutoTrigger AutoTriggerConfig `json:"autoTrigger,omitempty"`
}

// AccountConfig holds per-account configuration for multi-installation support.
type AccountConfig struct {
	// Mode is the authentication mode: "app" or "token".
	Mode string `json:"mode"`
	// AppID is the GitHub App ID (required when mode is "app").
	AppID int64 `json:"appId"`
	// InstallationID is the GitHub App installation ID.
	InstallationID int64 `json:"installationId"`
	// PrivateKeyPath is the path to the GitHub App private key PEM file.
	PrivateKeyPath string `json:"privateKeyPath"`
	// WebhookSecret is the secret used to verify webhook signatures.
	WebhookSecret string `json:"webhookSecret"`
	// Repositories is the allowlist of repositories in "owner/repo" format.
	Repositories []string `json:"repositories"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	// MaxEventsPerMinute is the maximum number of events to process per minute.
	MaxEventsPerMinute int `json:"maxEventsPerMinute,omitempty"`
}

// OutboundConfig holds outbound response settings.
type OutboundConfig struct {
	// Mode controls how the bot responds: "comment", "review", or "auto".
	Mode string `json:"mode,omitempty"`
	// OutboundMarker is a hidden marker text embedded in responses for bot loop prevention.
	OutboundMarker string `json:"outboundMarker,omitempty"`
}

// AutoTriggerConfig controls automatic triggering on certain events.
type AutoTriggerConfig struct {
	// OnPROpened triggers the bot when a pull request is opened.
	OnPROpened bool `json:"onPROpened,omitempty"`
	// OnIssueOpened triggers the bot when an issue is opened.
	OnIssueOpened bool `json:"onIssueOpened,omitempty"`
}

// TriggerConfig defines when the channel should respond to events.
type TriggerConfig struct {
	// RequireMention requires the bot to be @mentioned to trigger.
	RequireMention bool `json:"requireMention"`
	// BotUsername is the username of the bot (used for mention detection).
	BotUsername string `json:"botUsername"`
	// Commands is a list of command prefixes that trigger the bot, e.g. ["/openclaw"].
	Commands []string `json:"commands"`
	// Labels is a list of labels that trigger the bot when applied.
	Labels []string `json:"labels"`
}

// LoadFromFile loads configuration from a JSON file.
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	return Parse(data)
}

// Parse parses configuration from JSON bytes.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}
	return &cfg, nil
}

// Validate checks the configuration for required fields and consistency.
func (c *Config) Validate() error {
	var errs []string

	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}

	if !c.Channel.Enabled {
		return nil
	}

	if len(c.Channel.Accounts) > 0 {
		// Multi-account mode: validate each account independently.
		for name, acct := range c.Channel.Accounts {
			errs = append(errs, validateAccountConfig(name, acct)...)
		}
	} else {
		// Single-account (flat) mode: validate top-level fields.
		switch c.Channel.Mode {
		case "app":
			if c.Channel.AppID == 0 {
				errs = append(errs, "channel.appId is required when mode is 'app'")
			}
			if c.Channel.InstallationID == 0 {
				errs = append(errs, "channel.installationId is required when mode is 'app'")
			}
			if c.Channel.PrivateKeyPath == "" {
				errs = append(errs, "channel.privateKeyPath is required when mode is 'app'")
			}
		case "token":
			// Token mode validation can be added later.
		case "":
			errs = append(errs, "channel.mode is required (use 'app' or 'token')")
		default:
			errs = append(errs, fmt.Sprintf("channel.mode %q is not supported (use 'app' or 'token')", c.Channel.Mode))
		}

		if c.Channel.WebhookSecret == "" {
			errs = append(errs, "channel.webhookSecret is required")
		}

		if len(c.Channel.Repositories) == 0 {
			errs = append(errs, "channel.repositories must contain at least one repository")
		}
		for _, repo := range c.Channel.Repositories {
			parts := strings.SplitN(repo, "/", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				errs = append(errs, fmt.Sprintf("channel.repositories: %q must be in 'owner/repo' format", repo))
			}
		}
	}

	if c.Channel.Trigger.RequireMention && c.Channel.Trigger.BotUsername == "" {
		errs = append(errs, "channel.trigger.botUsername is required when requireMention is true")
	}

	// Validate outbound mode.
	switch c.Channel.Outbound.Mode {
	case "", "comment", "review", "auto":
		// valid
	default:
		errs = append(errs, fmt.Sprintf("channel.outbound.mode %q is not supported (use 'comment', 'review', or 'auto')", c.Channel.Outbound.Mode))
	}

	// Validate rate limit.
	if c.Channel.RateLimit.MaxEventsPerMinute < 0 {
		errs = append(errs, "channel.rateLimit.maxEventsPerMinute must not be negative")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// validateAccountConfig validates a single named account configuration.
func validateAccountConfig(name string, acct *AccountConfig) []string {
	prefix := fmt.Sprintf("channel.accounts[%s]", name)
	var errs []string

	switch acct.Mode {
	case "app":
		if acct.AppID == 0 {
			errs = append(errs, prefix+".appId is required when mode is 'app'")
		}
		if acct.InstallationID == 0 {
			errs = append(errs, prefix+".installationId is required when mode is 'app'")
		}
		if acct.PrivateKeyPath == "" {
			errs = append(errs, prefix+".privateKeyPath is required when mode is 'app'")
		}
	case "token":
		// Token mode validation can be added later.
	case "":
		errs = append(errs, prefix+".mode is required (use 'app' or 'token')")
	default:
		errs = append(errs, fmt.Sprintf("%s.mode %q is not supported (use 'app' or 'token')", prefix, acct.Mode))
	}

	if acct.WebhookSecret == "" {
		errs = append(errs, prefix+".webhookSecret is required")
	}

	if len(acct.Repositories) == 0 {
		errs = append(errs, prefix+".repositories must contain at least one repository")
	}
	for _, repo := range acct.Repositories {
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			errs = append(errs, fmt.Sprintf("%s.repositories: %q must be in 'owner/repo' format", prefix, repo))
		}
	}

	return errs
}

// IsRepoAllowed checks if a repository is in the allowlist.
// It checks both the flat repositories list and all account repositories.
func (c *Config) IsRepoAllowed(fullName string) bool {
	for _, repo := range c.Channel.Repositories {
		if strings.EqualFold(repo, fullName) {
			return true
		}
	}
	for _, acct := range c.Channel.Accounts {
		for _, repo := range acct.Repositories {
			if strings.EqualFold(repo, fullName) {
				return true
			}
		}
	}
	return false
}

// GetAccount returns the account config for the given name.
// When the accounts map is empty, it returns a default AccountConfig
// derived from the flat ChannelConfig fields.
func (c *Config) GetAccount(name string) *AccountConfig {
	if len(c.Channel.Accounts) > 0 {
		return c.Channel.Accounts[name]
	}
	// Fall back to the flat config as the default account.
	return &AccountConfig{
		Mode:           c.Channel.Mode,
		AppID:          c.Channel.AppID,
		InstallationID: c.Channel.InstallationID,
		PrivateKeyPath: c.Channel.PrivateKeyPath,
		WebhookSecret:  c.Channel.WebhookSecret,
		Repositories:   c.Channel.Repositories,
	}
}

// GetAccountForRepo finds the account that manages the given repository.
// It returns the account name and its configuration. When the accounts map
// is empty, it falls back to the default (flat) config with name "default".
// Returns ("", nil) if no account manages the repo.
func (c *Config) GetAccountForRepo(repo string) (string, *AccountConfig) {
	for name, acct := range c.Channel.Accounts {
		for _, r := range acct.Repositories {
			if strings.EqualFold(r, repo) {
				return name, acct
			}
		}
	}
	if len(c.Channel.Accounts) == 0 {
		for _, r := range c.Channel.Repositories {
			if strings.EqualFold(r, repo) {
				return "default", c.GetAccount("default")
			}
		}
	}
	return "", nil
}
