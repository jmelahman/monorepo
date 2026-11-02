# Desktop app

The desktop app opens kanban in its own window on Linux. It's a client for a server that's already running, such as the container from `docker compose up -d`. It doesn't start the server or store boards, and closing it leaves your sessions running.

## Build it

The app isn't part of kanban's releases yet, so build it from the repo. You need [Bun](https://bun.sh/).

```sh
cd desktop
bun install
bun run start     # run it from source
bun run package   # build an AppImage and a .deb in desktop/dist/
```

## Choose a server

The app connects to `http://localhost:7474` by default. It uses the first of these that's set:

1. The `--server=<url>` flag
2. The `KANBAN_URL` environment variable
3. `server` in `~/.config/kanban-desktop/settings.json`

```json
{ "server": "http://devbox:7474" }
```

If the server isn't reachable, the app shows a waiting page and connects as soon as the server answers.

::: warning Kanban has no login
Anyone who can reach the server gets a shell in its session containers. Connect to a remote server only over a network you trust, such as a [Tailscale](https://tailscale.com/) tailnet or an SSH tunnel.
:::

## Notifications

Browser push doesn't work in the desktop app, so it shows its own notifications while it's open. It notifies you when an agent is waiting for permission, an agent finishes, or a session fails. Clicking a notification opens that ticket, which reloads the window.

To turn them off, uncheck **View → Show Notifications**. Press <kbd>Alt</kbd> to show the menu bar.

Session failures can take up to 30 seconds to appear. Unlike [mobile notifications](/guide/mobile#notifications), nothing arrives while the app is closed.

## Keyboard shortcuts

<kbd>Ctrl</kbd>+<kbd>W</kbd>, <kbd>Ctrl</kbd>+<kbd>R</kbd> and <kbd>Ctrl</kbd>+<kbd>Q</kbd> go to the session terminal, so the app's own shortcuts use <kbd>Shift</kbd>:

| Shortcut                                    | Action       |
| ------------------------------------------- | ------------ |
| <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>N</kbd> | New window   |
| <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>W</kbd> | Close window |
| <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>Q</kbd> | Quit         |
| <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>R</kbd> | Reload       |

Links to previews, forwarded ports and pull requests open in your default browser.
