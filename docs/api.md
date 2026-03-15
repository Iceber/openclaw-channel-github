# OpenClaw GitHub Channel - API Reference

## HTTP Endpoints

### POST /webhook

Receives GitHub webhook events. This is the main entry point for all GitHub integrations.

**Headers:**

| Header | Required | Description |
|--------|----------|-------------|
| `X-Hub-Signature-256` | Yes | HMAC-SHA256 signature of the payload |
| `X-GitHub-Event` | Yes | GitHub event type (e.g., `issue_comment`) |
| `X-GitHub-Delivery` | Yes | Unique delivery ID for deduplication |
| `Content-Type` | Yes | Must be `application/json` |

**Request Body:** Raw GitHub webhook JSON payload (max 10 MB).

**Response:**

All responses return `200 OK` with a JSON body (except for errors). This prevents GitHub from retrying webhook deliveries for handled events.

| Status | Body | Description |
|--------|------|-------------|
| `200` | `{"status":"processed","session_key":"..."}` | Event was processed successfully |
| `200` | `{"status":"duplicate"}` | Duplicate delivery, already processed |
| `200` | `{"status":"unsupported_event"}` | Event type not supported |
| `200` | `{"status":"skipped"}` | Event action not normalizable |
| `200` | `{"status":"repo_not_allowed"}` | Repository not in allowlist |
| `200` | `{"status":"bot_ignored"}` | Event from bot sender, ignored |
| `200` | `{"status":"outbound_marker_ignored"}` | Event contains outbound marker |
| `200` | `{"status":"no_trigger"}` | No trigger condition matched |
| `401` | `Invalid signature` | Webhook signature verification failed |
| `405` | `Method not allowed` | Non-POST request |
| `500` | Error message | Internal processing error |

**Example:**

```bash
curl -X POST https://your-server.com/webhook \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: issue_comment" \
  -H "X-GitHub-Delivery: abc123" \
  -H "X-Hub-Signature-256: sha256=..." \
  -d '{"action":"created","issue":{...},"comment":{...}}'
```

---

### GET /health

Health check endpoint.

**Response:**

```json
{"status":"ok"}
```

---

## Normalized Event Format

All GitHub events are normalized into a unified `NormalizedEvent` structure:

```json
{
  "provider": "github",
  "accountId": "default",
  "repository": "owner/repo",
  "thread": {
    "type": "issue|pull_request|discussion",
    "number": 42,
    "title": "Issue/PR/Discussion title",
    "url": "https://github.com/owner/repo/issues/42"
  },
  "message": {
    "type": "comment|issue_body|pr_body|review|review_comment|discussion_body|context_update|ci_status",
    "id": "comment-999",
    "text": "Message content",
    "createdAt": "2026-03-15T00:00:00Z"
  },
  "sender": {
    "id": "github:12345",
    "login": "alice",
    "association": "MEMBER",
    "isBot": false
  },
  "trigger": {
    "kind": "mention|command|label|auto",
    "command": "/openclaw"
  },
  "context": {
    "labels": ["bug", "ai-review"],
    "assignees": ["alice", "bob"],
    "state": "open",
    "eventAction": "opened",
    "reviewState": "approved",
    "filePath": "src/main.go",
    "line": 42,
    "ciStatus": "completed",
    "ciConclusion": "success",
    "merged": false,
    "headRef": "feature-branch",
    "baseRef": "main",
    "reviewThreadId": 12345,
    "discussionCategory": "General"
  }
}
```

### Thread Types

| Type | Description |
|------|-------------|
| `issue` | GitHub Issue |
| `pull_request` | GitHub Pull Request |
| `discussion` | GitHub Discussion |

### Message Types

| Type | Source | Description |
|------|--------|-------------|
| `comment` | Issue/PR/Discussion comments | User comment text |
| `issue_body` | Issue opened | Initial issue description |
| `pr_body` | PR opened | Initial PR description |
| `review` | PR review submitted | Review summary text |
| `review_comment` | Inline PR review comment | Inline code comment |
| `discussion_body` | Discussion created | Discussion description |
| `context_update` | Edit/close/label/sync events | State change notification |
| `ci_status` | check_run/workflow_run | CI/CD status update |

### Trigger Kinds

| Kind | Description |
|------|-------------|
| `mention` | Bot was @mentioned in the message |
| `command` | Message starts with a configured slash command |
| `label` | A configured label was found on the issue/PR |
| `auto` | Event matched an auto-trigger rule |

### Context Fields

| Field | Type | When Present |
|-------|------|-------------|
| `labels` | []string | Issue/PR events |
| `assignees` | []string | Issue/PR events |
| `state` | string | Issue/PR events |
| `eventAction` | string | All context events |
| `reviewState` | string | PR review events |
| `filePath` | string | Inline review comments |
| `line` | int | Inline review comments |
| `ciStatus` | string | CI events |
| `ciConclusion` | string | CI events |
| `merged` | bool | PR close events |
| `headRef` | string | PR events |
| `baseRef` | string | PR events |
| `reviewThreadId` | int64 | Inline review comments |
| `discussionCategory` | string | Discussion events |

---

## Session Key Format

Session keys uniquely identify a conversation thread:

```
github:<owner>/<repo>:<threadType>:<number>
```

**Examples:**
- `github:openclaw/openclaw:issue:42`
- `github:openclaw/openclaw:pull_request:100`
- `github:openclaw/openclaw:discussion:7`
- `github:openclaw/openclaw:pull_request:100:review-thread:12345`

---

## Outbound API

The channel sends responses back to GitHub using the following API calls:

### Send Comment

Creates a comment on an issue or PR.

```
POST /repos/{owner}/{repo}/issues/{number}/comments
```

**Headers:**
```
Authorization: Bearer <installation-token>
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
```

**Body:**
```json
{
  "body": "AI response text\n<!-- openclaw-outbound -->"
}
```

### Send PR Review

Creates a PR review with an optional approval action.

```
POST /repos/{owner}/{repo}/pulls/{number}/reviews
```

**Body:**
```json
{
  "body": "Review summary\n<!-- openclaw-outbound -->",
  "event": "COMMENT|APPROVE|REQUEST_CHANGES",
  "comments": [
    {
      "path": "src/main.go",
      "line": 42,
      "body": "Consider refactoring this function."
    }
  ]
}
```

### Add Reaction

Adds a reaction emoji to a comment.

```
POST /repos/{owner}/{repo}/issues/comments/{comment_id}/reactions
```

**Body:**
```json
{
  "content": "+1|heart|rocket|eyes"
}
```

---

## MessageHandler Interface

The webhook handler accepts a `MessageHandler` callback for custom processing:

```go
type MessageHandler func(sessionKey string, event *normalizer.NormalizedEvent) (string, error)
```

**Parameters:**
- `sessionKey` — Stable session identifier (e.g., `github:owner/repo:issue:42`)
- `event` — Normalized event with all context metadata

**Returns:**
- `string` — Reply text to send back to GitHub (empty string = no reply)
- `error` — Processing error (returns 500 to webhook caller)

**Example:**

```go
handler.MessageHandler = func(sessionKey string, event *normalizer.NormalizedEvent) (string, error) {
    // Forward to OpenClaw Gateway
    response, err := gateway.Process(sessionKey, event)
    if err != nil {
        return "", err
    }
    return response.Text, nil
}
```

---

## Configuration JSON Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "server": {
      "type": "object",
      "properties": {
        "addr": {"type": "string", "default": ":8080"}
      }
    },
    "channel": {
      "type": "object",
      "required": ["enabled"],
      "properties": {
        "enabled": {"type": "boolean"},
        "mode": {"type": "string", "enum": ["app", "token"]},
        "appId": {"type": "integer"},
        "installationId": {"type": "integer"},
        "privateKeyPath": {"type": "string"},
        "webhookSecret": {"type": "string"},
        "repositories": {
          "type": "array",
          "items": {"type": "string", "pattern": "^[^/]+/[^/]+$"}
        },
        "ignoreBots": {"type": "boolean", "default": true},
        "accounts": {
          "type": "object",
          "additionalProperties": {
            "type": "object",
            "properties": {
              "mode": {"type": "string", "enum": ["app", "token"]},
              "appId": {"type": "integer"},
              "installationId": {"type": "integer"},
              "privateKeyPath": {"type": "string"},
              "webhookSecret": {"type": "string"},
              "repositories": {
                "type": "array",
                "items": {"type": "string"}
              }
            }
          }
        },
        "trigger": {
          "type": "object",
          "properties": {
            "requireMention": {"type": "boolean"},
            "botUsername": {"type": "string"},
            "commands": {"type": "array", "items": {"type": "string"}},
            "labels": {"type": "array", "items": {"type": "string"}}
          }
        },
        "autoTrigger": {
          "type": "object",
          "properties": {
            "onPROpened": {"type": "boolean"},
            "onIssueOpened": {"type": "boolean"}
          }
        },
        "outbound": {
          "type": "object",
          "properties": {
            "mode": {"type": "string", "enum": ["comment", "review", "auto"]},
            "outboundMarker": {"type": "string"}
          }
        },
        "rateLimit": {
          "type": "object",
          "properties": {
            "maxEventsPerMinute": {"type": "integer", "minimum": 0}
          }
        }
      }
    }
  }
}
```

---

## Supported GitHub Events

### Full Event Matrix

| Event | Action | Phase | Processed As |
|-------|--------|-------|-------------|
| `issues` | `opened` | 1 | `issue_body` message |
| `issues` | `edited` | 3 | `context_update` |
| `issues` | `closed` | 3 | `context_update` |
| `issues` | `reopened` | 3 | `context_update` |
| `issues` | `labeled` | 3 | `context_update` + label trigger |
| `issue_comment` | `created` | 1 | `comment` message |
| `issue_comment` | `edited` | 3 | `context_update` |
| `pull_request` | `opened` | 1 | `pr_body` message |
| `pull_request` | `edited` | 3 | `context_update` |
| `pull_request` | `closed` | 3 | `context_update` (closed/merged) |
| `pull_request` | `synchronize` | 3 | `context_update` (new commits) |
| `pull_request` | `labeled` | 3 | `context_update` + label trigger |
| `pull_request_review` | `submitted` | 1 | `review` message |
| `pull_request_review_comment` | `created` | 1 | `review_comment` message |
| `discussion` | `created` | 4 | `discussion_body` message |
| `discussion_comment` | `created` | 4 | `comment` message |
| `check_run` | `completed` | 4 | `ci_status` update |
| `workflow_run` | `completed` | 4 | `ci_status` update |

---

## Error Handling

### Webhook Processing Errors

| Error | HTTP Status | Behavior |
|-------|-------------|----------|
| Invalid signature | 401 | Rejected immediately |
| Unsupported event | 200 | Acknowledged, not processed |
| Duplicate delivery | 200 | Acknowledged, not re-processed |
| Repository not allowed | 200 | Acknowledged, not processed |
| Internal error | 500 | Error logged, GitHub may retry |

### Outbound Errors

When outbound API calls fail:
- Error is logged with session context
- HTTP 500 returned to webhook caller
- GitHub may retry the webhook delivery
- Idempotency store prevents duplicate processing on retry

### Long Text Handling

GitHub comments have a ~65,536 character limit. The channel automatically truncates responses that exceed this limit, appending `"..."` to indicate truncation.
