# Home Lab

## Dashboard

`nginx` serves a dashboard at `/` and every app under a path of its own:

| Address           | Serves                                                            |
| ----------------- | ----------------------------------------------------------------- |
| `/`               | `dashboard/`, linking to the rest                                 |
| `/draw`           | `../image-editor`, from the working tree                          |
| `/write`          | `../text-editor`, from the working tree                           |
| `/view`           | `../document-viewer`, from the working tree                       |
| `/files`          | `..`, the monorepo, for `/write` to save                          |
| `/design`         | `../.config/git-orchard/shared/design/design`, shared by them all |
| `/notebook`       | notebook, built from `../notebook`                                |
| `/music`          | music, built from `../music`                                       |
| `/audiobookshelf` | audiobookshelf                                                    |
| `/video`          | jellyfin                                                          |
| `/pihole/admin`   | pihole                                                            |

These answer on any name or address that reaches the host, other than the
`.home` names, which still go straight to their own service. Being on one
origin, the apps also share a theme, switched from the right of each header.

Two of the services have to be told about their path. For pihole that is
`FTLCONF_webserver_paths_prefix` in `docker-compose.yml`. For jellyfin it is
the Base URL under Dashboard > Networking, or,

```shell
docker exec jellyfin sed -i 's|<BaseUrl />|<BaseUrl>/video</BaseUrl>|' /config/config/network.xml
docker restart jellyfin
```

audiobookshelf only supports `/audiobookshelf`, and needs no configuration.

### Files

The editor at `/write` opens and saves Markdown in this repository through
`/files`, which works in browsers that cannot write to a file themselves.
nginx lists the folders there, and reads and writes nothing but `.md`,
`.markdown`, and `.txt`; anything under a dot directory is refused. Anyone
who can reach the dashboard can read and overwrite those files.

### Notebook

The notebook at `/notebook` turns sources into podcast episodes with Claude
Code, signed in with the host's login: the container shares `~/.claude` and
`~/.claude.json`, so sign in on the host first. It only answers to
`HOMELAB_IP` and the names in `NOTEBOOK_HOSTS`, such as the tailnet name, in
`.env`:

```shell
NOTEBOOK_HOSTS=onix,onix.example.ts.net
```

The image is built from `../notebook`, with the version of Claude Code pinned
in its Dockerfile. After a change to either,

```shell
docker compose up -d --build notebook
```

Episodes are published to `podcasts/`; to see them, add a Podcast library on
`/podcasts` in audiobookshelf once. Anyone who can reach the dashboard can use
the notebook, and through it Claude on your subscription.

### Music

The music client at `/music` streams to discover and buys to own: purchases
land in `~/media/music`, which the compose file mounts beside the media
jellyfin serves, so what is bought is also what jellyfin plays. The image is
published to ghcr.io/jmelahman/music on every push, so after a change,

```shell
docker compose up -d --build music
```

The page carries a manifest, so over Tailscale Serve the browser's
"install app" puts it on a phone's home screen.

### Syncthing

The owned library (`~/media/music`) syncs two-way with the phone's Music
folder, so what is bought lands on the phone and what the phone adds lands
on the server, where jellyfin picks it up too. Sync goes direct over the
tailnet — relays are off, so nothing leaves it — and a deletion on either
side is kept in `.stversions` for two weeks before it goes for good. The
GUI is at http://127.0.0.1:8384, on this machine only.

### Models

`llama` serves the local models with [llama.cpp](https://github.com/ggml-org/llama.cpp)
on port 18080, with an OpenAI-compatible API:

```shell
curl http://localhost:18080/v1/models
curl http://localhost:18080/v1/chat/completions \
  -d '{"model": "gemma4:26b", "messages": [{"role": "user", "content": "hi"}]}'
```

Devcontainers on `kanban-net` reach it as `http://llama:18080/v1`. It runs in
router mode: `llama.cpp/models.ini` lists the models, and llama.cpp loads one
on first request (`--models-max 1`), swapping it out when another is asked for.
Each preset carries the Ollama-era id as an alias, so the names below are what
clients request:

| Model             | Notes                                  |
| ----------------- | -------------------------------------- |
| `gemma4:26b`      | vision, via its mmproj                 |
| `gemma4:e4b`      | text only                              |
| `qwen3.6:35b-a3b` | text only                              |
| `qwen3.8:27b`     | vision, via its mmproj                 |

The GGUFs live in `models/`, which is not committed. They came from Ollama's
blob store (an Ollama model is a GGUF plus metadata). Two of them had a vision
tower fused into the same file, which llama.cpp rejects, so
`llama.cpp/convert-ollama-gguf.py` repacks the header and drops those tensors.
To migrate another Ollama model, find its blob in
`.../volumes/ollama_ollama_data/_data/models/blobs` (the `model` layer in the
manifest under `models/manifests`) and run:

```shell
./llama.cpp/convert-ollama-gguf.py --strip-vision --out models/name.gguf BLOB
```

Add a matching section to `llama.cpp/models.ini`, then

```shell
docker compose up -d llama
```

### Organizing

Choose **Edit** on the dashboard to add, remove, rename, and rearrange apps and
groups; cards can be dragged between groups. An address is a path (`/draw/`),
a port on this host (`:8443`), or a full URL.

**Done** writes the layout to `dashboard/data/apps.json`, so it is the same on
every device and can be committed. nginx accepts a `PUT` for that one file
from anyone who can reach the dashboard.

## Tailscale

[Tailscale Serve](https://tailscale.com/kb/1312/serve) publishes the above to
the tailnet over HTTPS, which the browser apps need for saving in place and
for clipboard access.

```shell
sudo ./serve.sh
```

`agentic-kanban` only works at the root of a host, so it is published on
port 8443 instead of a path. The configuration is kept by `tailscaled` and
survives restarts. To inspect it,

```shell
tailscale serve status
```

Port 443 must be left free for Tailscale, so `nginx` only listens on 80.
