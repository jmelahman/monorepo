# REST API

All endpoints are JSON over HTTP, rooted at `/api/`. Errors return a JSON body
of the form `{"error": "message"}`.

| Status | Meaning |
| --- | --- |
| `200 OK` | Read, update, or action (`complete`, `undo`, `draft`, `import`) |
| `201 Created` | A `POST` that created a resource; the body is the resource |
| `204 No Content` | Delete, login, logout |
| `400 Bad Request` | Invalid JSON, id, or field value |
| `401 Unauthorized` | Auth is on and the request has no valid session or token |
| `404 Not Found` | No such resource |
| `503 Service Unavailable` | The AI coach is required but unavailable |

## Authentication

When `APP_SECRET` is set, everything under `/api/` (except `health`, `login`,
and `logout`) and `/mcp` requires either:

- the `agilecbt_session` cookie set by `POST /api/login`, or
- an `Authorization: Bearer <APP_SECRET>` header (CLI, scripts, MCP clients).

The cookie holds an HMAC derived from the secret, not the secret itself, so
rotating `APP_SECRET` logs out every browser.

### `POST /api/login`

Request `{"secret": "…"}`. Sets the session cookie on success. A wrong secret
is `401` after a short delay.

### `POST /api/logout`

Clears the session cookie.

## Health

### `GET /api/health`

Always open. The web UI uses it to decide between the login screen and the app,
and whether to show the coach chat.

```json
{
  "status": "ok",
  "version": "v0.1.0",
  "auth_required": true,
  "authenticated": false,
  "llm": { "backend": "openai", "available": true, "detail": "qwen3.8:27b at localhost:11434" }
}
```

When the coach is turned off, `llm` is
`{ "backend": "none", "available": false, "detail": "The AI coach is turned off" }`.

## Today and mood

### `GET /api/today`

The home-screen snapshot for the server's local date:

```json
{
  "date": "2026-09-25",
  "morning": { "id": 3, "kind": "morning", "mood": 4, "energy": 3, "anxiety": 6, "…": "…" },
  "evening": null,
  "today": [{ "id": 7, "title": "Short walk", "energy_cost": 1, "carried_over": false, "…": "…" }],
  "done_today": [],
  "week": { "id": 2, "start_date": "2026-09-21", "intention": "Move a little every day" },
  "week_steps": []
}
```

`carried_over` is true when the step was put in Today on an earlier day.

### `GET /api/mood?days=14`

Returns one entry per day, oldest first, for the last `days` days (1–366,
default 14). Each value is the latest reading from that day's check-ins, or `null`
on days with no check-in.

```json
[{ "date": "2026-09-24", "mood": 5, "energy": 4, "anxiety": 3 }]
```

## Values

The life areas that goals serve.

| Method | Path | Body / notes |
| --- | --- | --- |
| `GET` | `/api/values` | |
| `POST` | `/api/values` | `{"name", "description"?, "color"?}` |
| `PATCH` | `/api/values/{id}` | any of `name`, `description`, `color` |
| `DELETE` | `/api/values/{id}` | goals keep existing, unlinked |

## Goals

| Method | Path | Body / notes |
| --- | --- | --- |
| `GET` | `/api/goals?status=active` | `status` is optional: `active`, `resting`, `done` |
| `POST` | `/api/goals` | `{"title", "why"?, "horizon"?, "status"?, "value_id"?}` |
| `GET` | `/api/goals/{id}` | |
| `PATCH` | `/api/goals/{id}` | any field above; `"clear_value": true` unlinks the value |
| `DELETE` | `/api/goals/{id}` | its steps keep existing, unlinked |

## Steps

Steps are small actions on the board. Their `lane` is one of `someday`,
`week`, `today`, `done`, or `let_go`. Instead of deleting a step, the UI moves
it to `let_go`.

| Method | Path | Body / notes |
| --- | --- | --- |
| `GET` | `/api/steps?lane=week,today&goal_id=1` | both filters optional |
| `POST` | `/api/steps` | `{"title", "lane"?, "energy_cost"? (1–3), "notes"?, "goal_id"?, "predicted_pleasure"?}` |
| `GET` | `/api/steps/{id}` | |
| `PATCH` | `/api/steps/{id}` | edit fields and/or move (see below) |
| `POST` | `/api/steps/{id}/complete` | `{"mastery"?, "pleasure"?}` (0–10); moves to `done` |
| `DELETE` | `/api/steps/{id}` | permanent; prefer `let_go` |

A new step goes at the end of its lane. `lane` defaults to `someday`.

**Moves.** `PATCH` with `lane` and optionally `index` (0-based position in the
lane; omit it to append):

- Moving to a different lane stamps `lane_changed_at`. Moving into `done` sets
  `completed_at`, and moving out of `done` clears it.
- An `index` with no lane change only reorders the step.
- Sending the step's current lane with **no** `index` re-commits it. This
  refreshes `lane_changed_at`, so a carried-over Today step stops being marked
  `carried_over` ("keep for today").

## Weeks and retros

A week starts on Monday and is created the first time it's needed.

| Method | Path | Body / notes |
| --- | --- | --- |
| `GET` | `/api/weeks` | newest first |
| `GET` | `/api/weeks/current` | the current week's review (below) |
| `PATCH` | `/api/weeks/{id}` | `{"intention"}` |
| `GET` | `/api/weeks/{id}/review` | everything the retro looks at |
| `PUT` | `/api/weeks/{id}/retro` | `{"went_well"?, "was_hard"?, "try_next"?, "ai_draft"?}` (upsert) |
| `POST` | `/api/weeks/{id}/retro/draft` | the coach fills the retro; `503` if unavailable |

A review looks like this:

```json
{
  "week": { "id": 2, "start_date": "2026-09-21", "intention": "" },
  "end_date": "2026-09-27",
  "checkins": [],
  "completed": [],
  "thought_records": [],
  "retro": null
}
```

## Check-ins

| Method | Path | Body / notes |
| --- | --- | --- |
| `GET` | `/api/checkins?from=2026-09-01&to=2026-09-30` | both optional |
| `POST` | `/api/checkins` | `{"kind": "morning"\|"evening"\|"adhoc", "mood"?, "energy"?, "anxiety"?, "note"?, "topic"?, "intro"?, "date"?}`; `date` defaults to today |
| `GET` | `/api/checkins/{id}` | includes `messages` and `actions` (the chat transcript and AI writes) |
| `PATCH` | `/api/checkins/{id}` | any field above plus `summary` |

`mood`, `energy`, and `anxiety` are 0–10.

`intro` is the opening of the conversation, stored as its first messages
without a coach turn: `[{"role": "assistant"|"user", "text": "…"}]`, at most
20. The Today screen uses it for the standup questions and answers, then sends
the last answer with `POST /api/checkins/{id}/messages` so the coach replies.

`topic` says what the conversation is about: `""` (the default) is a daily
check-in, and `"roadmap"` is a planning chat from the Roadmap page (use it with
`"kind": "adhoc"`). Only `""` check-ins count toward Today.

| Method | Path | Body / notes |
| --- | --- | --- |
| `GET` | `/api/conversations/roadmap` | today's roadmap conversation with `messages` and `actions`, or `null` if there isn't one yet |

### `POST /api/checkins/{id}/messages`

Sends one message (`{"text": "…"}`) to the AI coach and streams its reply as
[server-sent events](https://html.spec.whatwg.org/multipage/server-sent-events.html):

```text
event: text
data: {"text":"That sounds like a lot. "}

event: action
data: {"id":12,"tool":"create_step","summary":"Added \"Short walk\" to Today","entity":"step","entity_id":7,"…":"…"}

event: text
data: {"text":"Done, a short walk is on Today."}

event: done
data: {}
```

- `text` events carry deltas of the assistant's reply. Concatenate them.
- An `action` event arrives for each change the coach makes, in order with
  the text. Each one can be undone.
- `error` (`{"error": "…"}`) reports a failed turn. The stream still ends with
  `done`.

This returns `503` when the coach is unavailable (see `llm` in `/api/health`).

## Thought records

| Method | Path | Body / notes |
| --- | --- | --- |
| `GET` | `/api/thoughts?limit=20` | newest first |
| `POST` | `/api/thoughts` | see fields below |
| `GET` | `/api/thoughts/{id}` | |
| `PATCH` | `/api/thoughts/{id}` | any field |
| `DELETE` | `/api/thoughts/{id}` | |

The fields are:

- `situation`
- `emotions` and `rerated_emotions`: `[{"name", "intensity" 0–100}]`
- `automatic_thought`
- `distortions`: keys such as `catastrophizing` or `mind_reading`
- `evidence_for` and `evidence_against`
- `balanced_thought`

## Coach notes

The coach's memory. Everything it remembers is listed here, and you can edit
any of it.

| Method | Path | Body |
| --- | --- | --- |
| `GET` | `/api/notes` | |
| `POST` | `/api/notes` | `{"text"}` |
| `PATCH` | `/api/notes/{id}` | `{"text"}` |
| `DELETE` | `/api/notes/{id}` | |

## Settings

### `GET /api/settings` / `PATCH /api/settings`

Settings are a flat string map. `PATCH` takes any subset of the keys and
returns the full map. An empty string restores a key's default.

| Key | Values |
| --- | --- |
| `checkin_times` | `morning`, `evening`, or `both` (default) |

Crisis lines aren't a setting; they're set with `crisis_resources` in
[`config.toml`](/guide/configuration#crisis-resources).

### `GET /api/support`

The crisis lines the coach shares, as shown in Settings → Support. Read-only.

```json
{ "crisis_resources": "If you might act on thoughts of harming yourself, please reach out now:\n\n- US: call or text 988 …" }
```

## Coach model

The coach's model settings (Settings → AI model) start from `config.toml` and
the environment, and can be overridden here. Overrides are stored in the
database and apply immediately, without a restart.

### `GET /api/llm`

```json
{
  "effective": {
    "llm": "openai",
    "base_url": "https://openrouter.ai/api/v1",
    "model": "anthropic/claude-sonnet-5",
    "reasoning_effort": "none",
    "api_key_set": true
  },
  "defaults": {
    "llm": "openai",
    "base_url": "http://localhost:11434/v1",
    "model": "qwen3.8:27b",
    "reasoning_effort": "none",
    "api_key_set": false
  },
  "overrides": { "base_url": "https://openrouter.ai/api/v1", "model": "anthropic/claude-sonnet-5" },
  "api_key_saved": true
}
```

`effective` is what the coach uses, `defaults` comes from `config.toml` and
the environment, and `overrides` lists the fields saved in the app.
`api_key_saved` is true when a key saved in the app is in use. The API key
itself is never returned.

### `PATCH /api/llm`

Takes any of `llm` (`openai` or `none`), `base_url`, `model`,
`reasoning_effort` (`none`, `minimal`, `low`, `medium`, `high`, or `""`),
and `api_key`, and returns the same shape as `GET`.

- A field that's missing is left alone.
- `null` removes the override, going back to the default. An empty
  `base_url` or `model` does the same.
- `reasoning_effort: ""` keeps the parameter out of requests.
- `api_key: null` or `""` removes the saved key.

A saved API key is bound to the base URL in effect when it was saved,
including a `base_url` sent in the same request. The key from `config.toml`
or `APP_LLM_API_KEY` is only sent to the configured base URL. So changing the
URL never sends a key to another server. Saved keys are left out of exports
and ignored on import.

Invalid values return `400`.

### `GET /api/llm/models`

Returns the model ids the server lists (`GET {base_url}/models`), sorted. The
list is `[]` when the coach is off, or when the server can't be reached or
doesn't list its models.

## AI actions

Every change made by the coach or over MCP is logged along with the state
before and after.

| Method | Path | Notes |
| --- | --- | --- |
| `GET` | `/api/ai-actions?checkin_id=3&limit=50` | newest first |
| `POST` | `/api/ai-actions/{id}/undo` | restores the prior state and sets `undone_at` |

## Export / import

### `GET /api/export`

Returns all data as one JSON document: values, goals, steps, weeks, check-ins
with their transcripts, thought records, retros, notes, settings, and AI
actions.

### `POST /api/import`

Loads an export. The target database must be empty; otherwise this returns
`400`.

## MCP

`/mcp` is a [Model Context Protocol](https://modelcontextprotocol.io/) server
over streamable HTTP. It uses the same auth as `/api/`. It exposes the
coach's tools and a `daily_checkin` prompt. See
[AI coach & MCP](/guide/ai).
