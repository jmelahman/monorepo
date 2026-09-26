# AI coach & MCP

AgileCBT has one set of AI tools, and two ways to use them:

- **In-app coach**: the Today screen, where the check-in is a conversation
  the coach leads; the chat on the Roadmap page, a full-screen chat where the coach does the
  data entry for your values, goals, and steps; and "Draft with AI" on the Retro page.
- **MCP**: `/mcp` exposes the same tools to Claude Code, Claude Desktop, or any
  MCP client, so you can say "plan my week" from wherever you already talk to
  Claude. This is how to use your Claude subscription with the app.

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
| `llm` | `APP_LLM` | `openai`; `none` turns the coach off |
| `base_url` | `APP_LLM_BASE_URL` | `http://localhost:11434/v1` (a local Ollama) |
| `api_key` | `APP_LLM_API_KEY` | none; sent as `Authorization: Bearer` when set |
| `model` | `APP_MODEL` | `qwen3.8:27b` |
| `reasoning_effort` | `APP_LLM_REASONING_EFFORT` | `none`; set it to `""` to leave it out of requests |

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
and reliable tool calls, stick to a mid-size tool-calling model (the default
is fine). Smaller models work, but are noticeably weaker.

```sh
ollama pull qwen3.8:27b
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
APP_LLM_API_KEY=sk-or-… agilecbt serve
```

Your check-ins go to OpenRouter and the model's provider, so read their
privacy terms first.

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
