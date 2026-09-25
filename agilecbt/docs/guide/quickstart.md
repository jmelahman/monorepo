# Quickstart

## Start the server

```sh
agilecbt serve
```

The server listens on `:8080`. Open <http://localhost:8080/>.

By default, the AI curator runs through the `claude` CLI on your Claude
subscription. If you don't have that, or want everything to stay local, see
[AI curator & MCP](/guide/ai). `APP_LLM=none` turns the curator off, and every
other feature keeps working without it.

## A first week

1. **Today**: check in with mood, energy, and anxiety. If the curator is on,
   chat with it about what fits today.
2. **Roadmap**: add a value or two, such as *Health* or *Connection*. Then add a
   goal under each one, and a few small steps.
3. **Board**: drag a few steps into *This week*, and one or two into *Today*.
   Tap a Today step when you finish it, and rate how much accomplishment and
   enjoyment it gave you.
4. **Thoughts**: when something knocks you sideways, write a thought record.
5. **Retro**: at the end of the week, reflect, or let the curator draft it.
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
