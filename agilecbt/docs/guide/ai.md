# AI curator & MCP

AgileCBT has one set of AI tools, and two ways to use them:

- **In-app curator**: the chat on the Today screen after you check in, plus
  "Draft with AI" on the Retro page.
- **MCP**: `/mcp` exposes the same tools to Claude Code, Claude Desktop, or any
  MCP client, so you can say "plan my week" from wherever you already talk to
  Claude.

Either way, the AI can only act through these tools, and every change it makes
is logged and can be undone.

## Choosing a backend

Set `APP_LLM` on the server:

| `APP_LLM` | Runs on | Needs | Default model |
| --- | --- | --- | --- |
| `claude-code` (default) | Your Claude subscription (e.g. Max) | `claude` on `PATH`, logged in | Claude Code's default |
| `ollama` | Your own machine; data never leaves it | A running [Ollama](https://ollama.com) with a tool-calling model | `qwen3.8:27b` |
| `anthropic` | The Claude API, billed per token | `ANTHROPIC_API_KEY` | `claude-opus-5` |
| `none` | — | — | Curator off; everything else works |

Use `APP_MODEL` to pick a different model. Settings → AI curator and
`GET /api/health` both show whether the backend is reachable. When it isn't,
the Today screen keeps the manual check-in and hides the chat.

### `claude-code`

The server runs `claude -p` as a subprocess, in a locked-down mode:

- no built-in tools and no user or project settings
- only this app's `/mcp` server, which it reaches over loopback (see
  `APP_SELF_URL`)

Each check-in is one Claude Code session and continues with `--resume`.
Sessions are stored under `<data-dir>/claude`.

Before you start the server, run `claude` once as the same user and log in.

::: warning Subscription vs. API billing
If `ANTHROPIC_API_KEY` is set in the server's environment, Claude Code may use
it instead of your subscription login. To stay on your subscription, leave it
unset.
:::

### `ollama`

The server calls Ollama's `/api/chat` with the tool definitions and runs the
tool loop itself. Reasoning/thinking is turned off (`think: false`) so replies
stay short. For natural check-in conversations and reliable tool calls, stick
to a mid-size tool-calling model (the default is fine). Smaller models work,
but are noticeably weaker.

```sh
ollama pull qwen3.8:27b
APP_LLM=ollama agilecbt serve
```

## Connecting Claude Code over MCP

```sh
claude mcp add --transport http agilecbt http://localhost:8080/mcp \
  --header "Authorization: Bearer $APP_SECRET"
```

Leave out the header if the server has no `APP_SECRET`.

Then, in Claude Code:

- *"Use the agilecbt daily_checkin prompt"*: runs a morning or evening
  check-in.
- *"Look at my roadmap and plan this week — keep it light, I'm low on
  energy"*.

Claude Desktop and other MCP clients use the same URL and header.

## Tools

The read tools are `get_today`, `get_mood_history`, `list_values`,
`list_goals`, `list_steps`, `list_thought_records`, `get_week`, and
`list_curator_notes`.

The write tools are:

- `create_value`, `create_goal`, `update_goal`
- `create_step`, `update_step`, `move_step`, `complete_step`, `let_go_step`
- `record_checkin`, `create_thought_record`, `update_thought_record`
- `set_week_intention`, `save_retro_draft`
- `remember`, `forget`

The AI can't delete steps. It can only move them to the "let go" lane.

## Undo

Every write tool records an AI action holding the state before and after the
change. In the chat, each action shows up as a chip with an **Undo** button.
Undo restores the previous state: a created step is removed, and a moved step
goes back to where it was.

Changes made over MCP are logged the same way. To undo them, use
`GET /api/ai-actions` and `POST /api/ai-actions/{id}/undo`.

## Memory

The curator remembers things with `remember` ("mornings are hardest",
"walking helps") and drops them with `forget`. Those notes are plain rows
that you can read, edit, or delete under **Settings → What the curator
remembers**. The curator sees them at the start of every conversation, along
with:

- today's steps
- the last week of mood
- your active goals

## Safety

The curator is a supportive coach, not a therapist. Its system prompt tells it
to answer any sign of crisis or self-harm directly and warmly, and to point to
the crisis resources you've configured. The same resources are always one tap
away under **Need help now?** in the header.
