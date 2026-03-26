# lazyclaude PM-Worker Message API

lazyclaude MCP server provides a REST API for PM-Worker communication.

All endpoints require authentication via the `X-Auth-Token` header.

## Endpoints

### POST /msg/send

Send a message to another session.

```bash
curl -s -X POST http://127.0.0.1:${PORT}/msg/send \
  -H "X-Auth-Token: ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "from": "<your-session-id>",
    "to": "<target-session-id>",
    "type": "review_request",
    "body": "Please review the changes on branch feat-xyz. Summary: ..."
  }'
```

**Response:**
```json
{"id": "<message-uuid>"}
```

**Message types:**

| type | direction | description |
|------|-----------|-------------|
| `review_request` | Worker -> PM | Request PR review |
| `review_response` | PM -> Worker | Review result (approved / changes_requested) |
| `status` | any | Status update |
| `done` | Worker -> PM | Task completed |

### GET /msg/poll?session=\<id\>

Retrieve unread messages addressed to you. Messages are marked as read after retrieval.

```bash
curl -s "http://127.0.0.1:${PORT}/msg/poll?session=${SESSION_ID}" \
  -H "X-Auth-Token: ${TOKEN}"
```

**Response:**
```json
[
  {
    "id": "abc-123",
    "from": "worker-session-id",
    "to": "pm-session-id",
    "type": "review_request",
    "body": "Please review branch feat-xyz",
    "created_at": "2026-03-26T10:00:00Z",
    "read": false
  }
]
```

Returns `[]` when no unread messages exist.

### GET /msg/sessions

List all active sessions with their roles.

```bash
curl -s "http://127.0.0.1:${PORT}/msg/sessions" \
  -H "X-Auth-Token: ${TOKEN}"
```

**Response:**
```json
[
  {"id": "abc-123", "name": "pm", "role": "pm", "path": "/project"},
  {"id": "def-456", "name": "feat-login", "role": "worker", "path": "/project/.claude/worktrees/feat-login"}
]
```

## Authentication

The port and token are provided in your system prompt at session startup.
Use the `X-Auth-Token` header for all requests.
