# Mobile

Kanban works from a phone browser. You can also install it to the home screen and have it notify you when a session needs attention.

## What needs HTTPS

Browsers only allow installing an app and receiving push notifications on a secure connection: `https://`, or `http://localhost` on the machine running the browser.

| Feature                                  | Plain HTTP on the LAN | HTTPS |
| ---------------------------------------- | --------------------- | ----- |
| Boards, tickets, and the session terminal | Yes                   | Yes   |
| Install to the home screen               | No                    | Yes   |
| Push notifications                       | No                    | Yes   |

If you reach kanban at an address like `http://192.168.1.20:7474`, the app works but **Settings → Notifications** explains that it needs a secure connection.

::: warning Kanban has no login
Anyone who can reach the server can use it, and a session terminal is a shell in that session's container. Only expose kanban on a network you trust, such as your LAN or a [Tailscale](https://tailscale.com/) tailnet. Don't put it on a public hostname without an authenticating proxy in front.
:::

## The terminal and the on-screen keyboard

When the on-screen keyboard opens, the session terminal shrinks to the space above it, so the prompt and any permission dialog stay visible. The terminal scrolls to the latest output when it shrinks.

A session's terminal has one size shared by every browser attached to it. While your phone's keyboard is open, a desktop browser viewing the same session shows the shorter terminal too.

## Install the app

Open kanban over HTTPS, then:

- **Android (Chrome):** open the browser menu and choose **Add to Home screen** or **Install app**.
- **iPhone and iPad (Safari):** tap **Share**, then **Add to Home Screen**.
- **Desktop (Chrome, Edge):** click the install icon in the address bar.

The installed app opens without browser chrome. It doesn't work offline, because everything it shows comes from the server.

## Notifications

Notifications arrive even when kanban is closed or the phone is locked. Each browser or installed app is turned on separately.

1. Open **Settings → Notifications**.
2. Check **Send notifications to this device** and allow notifications when the browser asks.
3. Choose which events notify you.
4. Click **send test notification** to check that one arrives.

On iPhone and iPad, [install the app](#install-the-app) first and turn notifications on from the installed app. Safari doesn't offer push to a site in a browser tab.

| Event                                    | When it fires |
| ---------------------------------------- | ------------- |
| An agent is waiting for permission       | A working agent stops to ask you to approve something. |
| An agent finished working                | An agent's turn ends. |
| A session failed or stopped unexpectedly | A session fails to start, or its container dies. Stopping a session yourself doesn't notify. |

Tapping a notification opens that ticket's session. A newer notification for the same session replaces the older one.

Sessions started before this version keep their old Claude Code hooks until they are restarted. Until then, a session notifies you only for the first permission request in a turn. The same applies if your repo has its own `.claude/settings.local.json`, which kanban doesn't change.

The server sends notifications through your browser vendor's push service (Google, Apple, or Mozilla), so the kanban server needs outbound internet access. The notification's content is encrypted for your device. The push service can't read it.

If notifications stop arriving, uncheck and re-check **Send notifications to this device**.

## UI on a different machine

The machine that runs your sessions doesn't have to be the one your phone connects to. `kanban web` serves the UI and forwards API calls and terminal connections to a `kanban serve` running elsewhere:

```sh
# On the dev box, as usual
kanban serve

# On the machine your phone reaches
kanban web --backend http://devbox:7474
```

`kanban web` stores nothing and doesn't need Docker. Boards, sessions and notification subscriptions all stay on the backend. The backend can stay on plain HTTP on your LAN while the machine running `kanban web` provides HTTPS.

Forwarded task ports (`13000`–`13099`) are opened on the backend machine. Their links use the hostname you opened kanban with, so they work when that name is the backend machine itself: its LAN address, or its Tailscale name or `100.x` address from any device on your tailnet. They don't follow you through `kanban web` on a different machine, because that machine doesn't have the ports. The links are plain HTTP even when kanban itself is on HTTPS.

Preview URLs still point at the backend machine.

### HTTPS with Tailscale

On the machine running `kanban web` (or `kanban serve`, if you don't split them):

```sh
tailscale serve --bg 7474
```

kanban is then at `https://<machine>.<tailnet>.ts.net` for devices on your tailnet, with a certificate that phones trust. That is enough to install the app and turn on notifications.

### Behind nginx

If nginx already terminates HTTPS for you, point a `location` at either `kanban web` or `kanban serve`:

```nginx
# In the http block: only ask for an upgrade when the browser did.
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

location / {
    proxy_pass http://127.0.0.1:7474;
    proxy_http_version 1.1;

    # kanban rejects a terminal connection whose Origin doesn't match Host.
    proxy_set_header Host $http_host;

    # Terminal WebSockets
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $connection_upgrade;
    proxy_read_timeout 1d;

    # Live updates are server-sent events
    proxy_buffering off;
}
```

Use `$http_host`, not `$host`. `$host` drops the port, so on anything other than port 80 or 443 it no longer matches what the browser sent.

If the terminal shows `[disconnected]` right away while the rest of the UI works, the `Host` header isn't reaching kanban unchanged.

::: warning
`kanban web` has no login either. Anyone who can reach it gets the same shell access as on the backend. Keep both on your tailnet or LAN.
:::
