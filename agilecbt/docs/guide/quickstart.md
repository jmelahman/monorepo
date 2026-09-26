# Quickstart

## Start the server

```sh
agilecbt serve
```

The server listens on `:8080`. Open <http://localhost:8080/>.

By default, the AI coach uses a local [Ollama](https://ollama.com)
(`ollama pull qwen3.8:27b`). Any OpenAI-compatible API works, including
OpenRouter; see [AI coach & MCP](/guide/ai). `APP_LLM=none` turns the coach
off, and every other feature keeps working without it.

## A first week

1. **Today**: the screen is a chat with your coach. It opens with "How are
   you doing?". Answer in your own words or tap a quick reply, and the coach
   takes it from there: what you'd like to get done, what might get in the
   way, and one to three steps that fit your energy. Mood, energy, and anxiety
   sliders sit in a slim strip above the chat, and you can skip them. Today's
   plan is beside the chat on wide screens and a tap away on phones. With the
   coach off, you get three short questions instead and plan the day yourself.
2. **Roadmap**: another full-screen chat. The coach asks what matters to you,
   with a few suggestions above the message box to get started. Answer with
   something like *Health* or *Connection*, and it turns that into values,
   goals, and a few small steps. Your roadmap is beside the chat on wide
   screens and a tap away on phones. With the coach off, you add them
   yourself.
3. **Board**: drag a few steps into *This week*, and one or two into *Today*.
   Tap a Today step when you finish it, and rate how much accomplishment and
   enjoyment it gave you.
4. **Thoughts**: when something knocks you sideways, write a thought record.
5. **Retro**: at the end of the week, reflect, or let the coach draft it.
   What you'll "try next" becomes a suggested intention for the next week.

## From your phone

Set a secret and listen on your LAN or tailnet address:

```sh
APP_SECRET='something long' agilecbt serve --addr :8080
```

Serve it over HTTPS, for example with `tailscale serve 8080`. Then open it on
your phone, log in with the secret, and use "Add to Home Screen".

## Script it

```sh
agilecbt today
agilecbt step add "Short walk" --lane today --energy 1
agilecbt step done 1 --mastery 6 --pleasure 7
```

See the [CLI reference](/reference/cli).

## Ephemeral runs

For demos and tests, keep everything in memory. Nothing touches disk, and all
data is discarded on shutdown:

```sh
agilecbt serve --in-memory
```

## Development

Run the backend and frontend separately for hot reload:

```sh
# Terminal 1: backend on :8080 (wgo restarts on save; plain `go run` works too)
wgo run . serve

# Terminal 2: frontend on :5173, proxying /api to the backend
cd web && bun install && bun run dev
```
