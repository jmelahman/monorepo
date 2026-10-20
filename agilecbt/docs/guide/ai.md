# AI coach & MCP

AgileCBT has one set of AI tools, and two ways to use them:

- **In-app coach**: the Today screen, where the check-in is a conversation
  the coach leads; the chat on the Roadmap page, a full-screen chat where the coach does the
  data entry for your values, goals, and steps; and "Draft with AI" on the Retro page.
- **MCP**: `/mcp` exposes the same tools to Claude Code, Claude Desktop, or any
  MCP client, so you can say "plan my week" from wherever you already talk to
  Claude.

To use your Claude subscription for the in-app coach too, pick the
[Claude Code backend](#claude-code-your-claude-subscription).

Either way, the AI can only act through these tools, and every change it makes
is logged and can be undone.

## Choosing a backend

The coach talks to any OpenAI-compatible chat completions API: a local
[Ollama](https://ollama.com), [OpenRouter](https://openrouter.ai),
llama.cpp's `llama-server`, vLLM, LM Studio, or OpenAI itself. The server
sends the tool definitions and runs the tool loop itself, so the model only
needs to support tool calling.

| Key (`config.toml`) | Environment | Default |
| --- | --- | --- |
| `llm` | `AGILECBT_LLM` | `openai`; `claude-code` uses [Claude Code](#claude-code-your-claude-subscription); `none` turns the coach off |
| `base_url` | `AGILECBT_LLM_BASE_URL` | `http://localhost:11434/v1` (a local Ollama) |
| `api_key` | `AGILECBT_LLM_API_KEY` | none; sent as `Authorization: Bearer` when set |
| `model` | `AGILECBT_MODEL` | `gemma4:26b` (with `claude-code`: Claude Code's default) |
| `reasoning_effort` | `AGILECBT_LLM_REASONING_EFFORT` | `none`; set it to `""` to leave it out of requests |

These are the defaults. **Settings → AI model** can override any of them from
the app. It saves them in the database and applies them without a restart
(see [In-app settings](/guide/configuration#in-app-settings)).

Settings → AI model and `GET /api/health` both show whether the model is
reachable. When it isn't, the Today screen falls back to three scripted
questions and a manual plan, and the Roadmap page switches back to editable
forms.

`reasoning_effort = "none"` turns off thinking on hybrid models such as
Qwen3, so replies stay short and quick. If your provider rejects the
parameter, set it to `""`, or to `low` for a model that must reason.

### Ollama (local, the default)

Your check-ins never leave the machine. For natural check-in conversations
and reliable tool calls, stick to the default model or larger. Smaller models run, but
miss crisis resources and tool calls; see [Choosing a model](/guide/models).

```sh
ollama pull gemma4:26b
agilecbt serve
```

If Ollama runs elsewhere, set `base_url`, e.g. `http://ollama:11434/v1`.

### OpenRouter

One key reaches Claude, GPT, Gemini, and open models, billed per token.

```toml
base_url = "https://openrouter.ai/api/v1"
model = "anthropic/claude-sonnet-5"
```

Put the key in the environment rather than a file you might commit:

```sh
AGILECBT_LLM_API_KEY=sk-or-… agilecbt serve
```

Your check-ins go to OpenRouter and the model's provider, so read their
privacy terms first.

### Claude Code (your Claude subscription)

`llm = "claude-code"` runs each coach turn through the
[Claude Code](https://code.claude.com) CLI (`claude -p`). Replies count
against the Claude Pro or Max plan Claude Code is signed in to, with no API
key. The server needs `claude` on its `PATH`, signed in:

```sh
claude            # log in once
AGILECBT_LLM=claude-code agilecbt serve
```

Or choose **Claude Code** in Settings → AI model.

- `model` takes a Claude Code alias (`opus`, `sonnet`, `haiku`, `fable`) or a
  `claude-…` model id. Any other value, such as an Ollama model, uses Claude
  Code's default.
- `reasoning_effort` becomes Claude Code's `--effort`: `low`, `medium` or
  `high`. `none` and `minimal` run at `low`, and `""` leaves it to Claude Code.
- `base_url` and `api_key` are ignored.
- Claude Code's own tools (shell, files, web) are off. The coach's tools reach
  the app through its [MCP endpoint](#connecting-claude-code-over-mcp) on the
  server's `--addr`, with `AGILECBT_SECRET` as the bearer token when it's set.
  Changes show up as undoable chips like any other coach change.
- Each turn starts a fresh `claude` process and sends the conversation so far
  as a transcript, so a reply takes a few seconds longer than with an API.
- The crisis classifier's `safety_model` still needs an OpenAI-compatible
  `safety_base_url`; without one the phrase list runs alone.
- `agilecbt eval` doesn't support this backend.

Your check-ins go to Anthropic under your Claude account's terms.

## Connecting Claude Code over MCP

```sh
claude mcp add --transport http agilecbt http://localhost:8080/mcp \
  --header "Authorization: Bearer $AGILECBT_SECRET"
```

Leave out the header if the server has no `AGILECBT_SECRET`.

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

The coach remembers things with `remember` ("mornings are hardest",
"walking helps") and drops them with `forget`. Those notes are plain rows
that you can read, edit, or delete under **Settings → Coach → What your coach
remembers**. The coach sees them at the start of every conversation, along
with:

- today's steps
- the last week of mood
- your active goals

## Safety

The coach is supportive, not a therapist. Its system prompt tells it
to answer any sign of crisis or self-harm directly and warmly, and to point to
the crisis resources set by `crisis_resources` in
[`config.toml`](/guide/configuration#crisis-resources), or the built-in list.
The app doesn't rely on the prompt alone: a [crisis classifier](/guide/safety)
checks each message first and, when it flags one, turns the coach's tools off
and makes sure the crisis lines reach the person.
