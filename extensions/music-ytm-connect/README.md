# music-ytm-connect

Firefox extension that connects the homelab [music](../../music) server to
your YouTube Music account, without exporting a cookies.txt by hand.

- **Send my sign-in** reads YouTube's cookies from the window you click it
  in and sends them, in the Netscape format, to the server's
  `PUT /api/yt/account/cookies`.
- **Use this playlist as my Supermix** appears on a page with a `list=`
  in its address (open your Supermix on music.youtube.com) and sends that
  id to `PUT /api/yt/account/supermix`.
- **Test** asks the server to fetch the Supermix there and then.

The server's address is typed into the popup and kept in the extension's
storage; nothing in this repository names it. Saving it asks Firefox for
access to that one origin, which lets the popup call the server. The
cookies go only there.

## Using it

1. Load the extension (below) and click its icon.
2. Type the music server's address as you open it, e.g. `homelab/music`,
   and Save.
3. On music.youtube.com, signed in, click **Send my sign-in**.
4. Open your Supermix and click **Use this playlist as my Supermix**.
5. **Test**.

The cookies come from the clicked window's cookie store, so a private
window or a container sends its own sign-in. A dedicated private window
keeps the sign-in from being rotated by everyday browsing; allow the
extension under *Manage Extension → Run in Private Windows* for that.
Signing out of that session invalidates the cookies the server holds.

## Development

```bash
bun install

# Run unit tests (cookies.txt, playlist ids, server addresses)
bun run test

# Lint the extension
bun run lint

# Launch Firefox with the extension loaded
bun run start

# Package a .zip
bun run build
```

To load it manually instead: open `about:debugging#/runtime/this-firefox` →
**Load Temporary Add-on…** → select `manifest.json`.

## Layout

```
manifest.json            MV3, Firefox 142+
lib/connect.js           cookies.txt, playlist ids, server addresses (Bun-testable)
popup/                   the popup: server address, send, test
test/                    node:test suites
```

## License

GPLv3; see [LICENSE](LICENSE).
