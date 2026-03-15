# OpenClaw GitHub Channel - Usage Guide

## Overview

The OpenClaw GitHub Channel is a webhook-based integration that connects GitHub repositories to the OpenClaw AI agent framework. It enables AI-powered assistance directly within GitHub Issues, Pull Requests, Discussions, and CI/CD workflows.

**Key capabilities:**
- Listen to GitHub webhook events (issues, PRs, reviews, discussions, CI)
- Normalize events into a unified format for the OpenClaw Gateway
- Trigger agent responses based on @mentions, slash commands, labels, or auto-trigger rules
- Send responses back as GitHub comments or PR reviews
- Prevent bot loops via sender filtering and outbound markers
- Support multi-account / multi-installation setups

## Quick Start

### Prerequisites

1. A GitHub App with the following permissions:
   - **Issues**: Read & Write
   - **Pull Requests**: Read & Write
   - **Discussions**: Read & Write (optional)
   - **Metadata**: Read
   - **Contents**: Read (optional, for file context)

2. Webhook events configured on the GitHub App:
   - `issues`
   - `issue_comment`
   - `pull_request`
   - `pull_request_review`
   - `pull_request_review_comment`
   - `discussion` (optional)
   - `discussion_comment` (optional)
   - `check_run` (optional)
   - `workflow_run` (optional)

3. Go 1.21+ installed

### Installation

```bash
# Clone the repository
git clone https://github.com/Iceber/openclaw-channel-github.git
cd openclaw-channel-github

# Build the binary
go build -o openclaw-github-channel ./cmd/openclaw-github-channel/

# Run with a config file
./openclaw-github-channel -config config.json
```

### Minimal Configuration

Create a `config.json` file:

```json
{
  "server": {
    "addr": ":8080"
  },
  "channel": {
    "enabled": true,
    "mode": "app",
    "appId": 123456,
    "installationId": 78901234,
    "privateKeyPath": "/path/to/github-app-private-key.pem",
    "webhookSecret": "your-webhook-secret",
    "repositories": ["your-org/your-repo"],
    "trigger": {
      "requireMention": true,
      "botUsername": "your-bot-name",
      "commands": ["/openclaw"],
      "labels": ["ai-review", "ai-help"]
    },
    "ignoreBots": true
  }
}
```

## Configuration Reference

### Server Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `server.addr` | string | `:8080` | HTTP server listen address |

### Channel Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `channel.enabled` | bool | Yes | Enable/disable the channel |
| `channel.mode` | string | Yes | Authentication mode: `"app"` or `"token"` |
| `channel.appId` | int | Yes (app mode) | GitHub App ID |
| `channel.installationId` | int | Yes (app mode) | GitHub App Installation ID |
| `channel.privateKeyPath` | string | Yes (app mode) | Path to GitHub App private key PEM file |
| `channel.webhookSecret` | string | Yes | Webhook signing secret |
| `channel.repositories` | []string | Yes | Allowlisted repositories (`"owner/repo"` format) |
| `channel.ignoreBots` | bool | No | Ignore events from bot accounts |

### Trigger Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `channel.trigger.requireMention` | bool | `false` | Require `@bot` mention to trigger |
| `channel.trigger.botUsername` | string | - | Bot username for mention detection |
| `channel.trigger.commands` | []string | - | Slash command prefixes (e.g., `["/openclaw"]`) |
| `channel.trigger.labels` | []string | - | Labels that trigger the bot when applied |

### Auto-Trigger Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `channel.autoTrigger.onPROpened` | bool | `false` | Auto-trigger when a PR is opened |
| `channel.autoTrigger.onIssueOpened` | bool | `false` | Auto-trigger when an issue is opened |

### Outbound Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `channel.outbound.mode` | string | `"comment"` | Response mode: `"comment"`, `"review"`, or `"auto"` |
| `channel.outbound.outboundMarker` | string | `<!-- openclaw-outbound -->` | Hidden HTML comment for bot loop prevention |

### Rate Limiting

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `channel.rateLimit.maxEventsPerMinute` | int | 0 (unlimited) | Maximum events to process per minute |

### Multi-Account Configuration

For managing multiple GitHub App installations:

```json
{
  "channel": {
    "enabled": true,
    "accounts": {
      "default": {
        "mode": "app",
        "appId": 123456,
        "installationId": 111,
        "privateKeyPath": "/keys/default.pem",
        "webhookSecret": "secret-1",
        "repositories": ["org-a/repo-1", "org-a/repo-2"]
      },
      "enterprise": {
        "mode": "app",
        "appId": 654321,
        "installationId": 222,
        "privateKeyPath": "/keys/enterprise.pem",
        "webhookSecret": "secret-2",
        "repositories": ["org-b/repo-x"]
      }
    },
    "trigger": {
      "requireMention": true,
      "botUsername": "openclaw-bot"
    }
  }
}
```

## Trigger Model

The channel supports four trigger modes:

### 1. @Mention Trigger
The bot responds when mentioned in a comment:
```
@openclaw-bot Please summarize this issue.
```

### 2. Command Trigger
The bot responds to slash commands:
```
/openclaw review
/openclaw summarize
```

### 3. Label Trigger
The bot responds when a configured label is applied to an issue or PR:
- Apply label `ai-review` → triggers analysis
- Apply label `ai-help` → triggers assistance

### 4. Auto Trigger
The bot automatically responds to certain events:
- PR opened → auto-analyze (when `autoTrigger.onPROpened` is true)
- Issue opened → auto-respond (when `autoTrigger.onIssueOpened` is true)

### Trigger Priority
1. @Mention (highest priority)
2. Slash command
3. Label match
4. Auto-trigger (lowest priority)

## Supported Events

### Core Events (Phase 1)
| GitHub Event | Action | Normalized Type |
|-------------|--------|-----------------|
| `issue_comment` | `created` | `comment` |
| `issues` | `opened` | `issue_body` |
| `pull_request` | `opened` | `pr_body` |
| `pull_request_review` | `submitted` | `review` |
| `pull_request_review_comment` | `created` | `review_comment` |

### Context Events (Phase 3)
| GitHub Event | Action | Normalized Type |
|-------------|--------|-----------------|
| `issues` | `edited` | `context_update` |
| `issues` | `closed` | `context_update` |
| `issues` | `reopened` | `context_update` |
| `issues` | `labeled` | `context_update` |
| `issue_comment` | `edited` | `context_update` |
| `pull_request` | `edited` | `context_update` |
| `pull_request` | `closed` | `context_update` |
| `pull_request` | `synchronize` | `context_update` |
| `pull_request` | `labeled` | `context_update` |

### Discussion Events (Phase 4)
| GitHub Event | Action | Normalized Type |
|-------------|--------|-----------------|
| `discussion` | `created` | `discussion_body` |
| `discussion_comment` | `created` | `comment` |

### CI Events (Phase 4)
| GitHub Event | Action | Normalized Type |
|-------------|--------|-----------------|
| `check_run` | `completed` | `ci_status` |
| `workflow_run` | `completed` | `ci_status` |

## Session Model

Each GitHub thread (Issue, PR, Discussion) maps to a stable session:

```
github:<owner>/<repo>:issue:<number>
github:<owner>/<repo>:pull_request:<number>
github:<owner>/<repo>:discussion:<number>
```

For PR review threads (fine-grained):
```
github:<owner>/<repo>:pull_request:<number>:review-thread:<threadId>
```

Multiple comments within the same thread always map to the same session, enabling continuous conversation context.

## Bot Loop Prevention

Three layers of protection:

1. **Sender type filtering**: Events from `Bot` type accounts are ignored (when `ignoreBots` is true)
2. **Username matching**: Events from the bot's own username are always ignored
3. **Outbound marker**: Responses include a hidden HTML marker (`<!-- openclaw-outbound -->`) that is detected in incoming events to prevent self-triggering

## Security

### Webhook Signature Verification
All webhook payloads are verified using HMAC-SHA256 (`X-Hub-Signature-256` header).

### Repository Allowlist
Only events from explicitly allowlisted repositories are processed.

### Delivery Deduplication
GitHub may retry webhook deliveries. The channel uses delivery ID tracking to prevent duplicate processing.

### Minimal Permissions
Only request the minimum GitHub App permissions needed:
- Issues: Read/Write
- Pull Requests: Read/Write
- Metadata: Read

## Running Tests

```bash
# Run all unit tests
go test ./...

# Run with race detector
go test -race ./...

# Run e2e tests only
go test ./e2e/...

# Run with verbose output
go test -v ./...
```

## Architecture

```
GitHub Webhook POST /webhook
  → HMAC-SHA256 signature verification
  → Delivery ID deduplication
  → Event type parsing
  → Normalization to NormalizedEvent (with context metadata)
  → Repository allowlist check
  → Bot loop prevention (sender type + username + outbound marker)
  → Trigger evaluation (@mention / /command / label / auto)
  → MessageHandler callback
  → Outbound response (comment or PR review)
```

### Package Structure

```
cmd/openclaw-github-channel/   # Entry point
pkg/
  config/                       # Configuration loading and validation
  auth/                         # GitHub App authentication, webhook verification
  events/                       # Webhook event types and parsing
  normalizer/                   # Event normalization to unified format
  routing/                      # Session keys, trigger matching
  outbound/                     # GitHub API client (comments, reviews, reactions)
  state/                        # Idempotency and deduplication
  server/                       # HTTP webhook handler
e2e/                            # End-to-end tests
docs/                           # Documentation
```
