# Quickstart

## Start the server

```sh
app serve
```

The server listens on `:8080`. Open <http://localhost:8080/>.

Add a few items from the web UI, or script it from the CLI:

```sh
app item create --title "Hello"
app item list
app item delete 1
```

## Ephemeral runs

For demos and tests, keep everything in memory — nothing touches disk and all
data is discarded on shutdown:

```sh
app serve --in-memory
```

## Development

Run the backend and frontend separately for hot reload:

```sh
# Terminal 1 — backend on :8080 (wgo restarts on save; plain `go run` works too)
wgo run . serve

# Terminal 2 — frontend on :5173, proxying /api to the backend
cd web && npm install && npm run dev
```
