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

	if c.Channel.Trigger.RequireMention && c.Channel.Trigger.BotUsername == "" {
		errs = append(errs, "channel.trigger.botUsername is required when requireMention is true")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// IsRepoAllowed checks if a repository is in the allowlist.
func (c *Config) IsRepoAllowed(fullName string) bool {
	for _, repo := range c.Channel.Repositories {
		if strings.EqualFold(repo, fullName) {
			return true
		}
	}
	return false
}
