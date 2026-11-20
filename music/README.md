# music

The homelab's **music manager**: stream to own.

A skeleton today. The shape it grows into:

- Mixes and search served from YouTube Music, the audio proxied through
  this server so it plays anywhere the homelab is reachable.
- Purchases through pluggable store providers — where it buys from is
  configuration, not code — landing in an owned library on disk.
- The owned library beside the media jellyfin serves, so what is bought is
  also what jellyfin plays.

## Running

```shell
wgo run . serve
```

For development, the page and the shared design reload from disk:

```shell
wgo run . serve --web web
```

The server listens on `:8095`, and the owned library defaults to
`~/media/music`.

## Buying

The Buy button on a streamed track queues it with the store provider that
`MUSIC_STORE` names: an executable run as

```shell
$MUSIC_STORE buy --artist NAME --title NAME --dir LIBRARY
```

which lands the file under `LIBRARY/Artist/Album/` and prints JSON —
`artist`, `album`, `title`, `file`, `alreadyOwned` — or exits non-zero
with the reason on stderr. The server runs one buy at a time, retries a
failed one after a minute, five, then thirty, and saves the queue under
`--state` so a restart picks up where it left off. `api/buys` lists it,
newest first. Without `MUSIC_STORE`, buying answers 503.

## Image

Every push to master and every release builds
[`ghcr.io/jmelahman/music`](https://github.com/jmelahman/music/pkgs/container/music),
the image the homelab's compose file pulls.

```shell
docker build -t ghcr.io/jmelahman/music .
```

## Homelab

The [homelab](https://github.com/jmelahman/monorepo) fronts the client at
`/music` on its nginx, with a card on the dashboard; see
`homelab/docker-compose.yml` there. The compose file mounts `~/media/music`
as the owned library, next to the media jellyfin serves.

Over the tailnet, Tailscale Serve publishes it over HTTPS, which is what
makes the page installable as an app on a phone.

## Shared files

`design/`, `.golangci.yml`, `.github/workflows/pre-commit.yml` and the marked
blocks in `.pre-commit-config.yaml` are shared with the other projects in the
monorepo: edit them there and `git orchard sync` copies them here.

## Offline

The page registers a service worker (`web/sw.js`) that caches the shell and
the library listing, so the installed app opens off the tailnet. Albums are
saved for offline deliberately — the button in the crumbs bar — into the
browser's cache; saved tracks play from there with seeking, and removing
one is the same button again.

## License

GPLv3, like the notebook; see [LICENSE](LICENSE).
