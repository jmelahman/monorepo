# Home Lab

## Dashboard

`nginx` serves a dashboard at `/` and every app under a path of its own:

| Address           | Serves                                   |
| ----------------- | ---------------------------------------- |
| `/`               | `dashboard/`, linking to the rest        |
| `/draw`           | `../image-editor`, from the working tree |
| `/write`          | `../text-editor`, from the working tree  |
| `/files`          | `..`, the monorepo, for `/write` to save |
| `/audiobookshelf` | audiobookshelf                           |
| `/video`          | jellyfin                                 |
| `/pihole/admin`   | pihole                                   |

These answer on any name or address that reaches the host, other than the
`.home` names, which still go straight to their own service.

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
