---
og-title: "Browser apps with no build step, served from my homelab"
title-prefix: "Blog"
date: 2026-10-11
nav_blog: true
---

# Browser apps with no build step, served from my homelab

I love software that is a folder of files.

I recently wrote two small apps that are exactly that:
[Write](https://jamison.lahman.dev/write/), a WYSIWYG Markdown editor, and
[Draw](https://jamison.lahman.dev/draw/), a basic image editor.
Each is an `index.html`, a `style.css` and an `app.js`.
There is no bundler, no `node_modules`, and no server; nothing you type or draw leaves the browser.

This post is about the setup around them: how they are served at home, how they get HTTPS, and
how the same files end up on the public internet.

## Served from the working tree

My homelab already ran `nginx` in front of a few services.
The apps are just two more volumes, mounted read-only straight from where I edit them,

```yaml
  nginx:
    image: nginx
    ports:
      - 80:80
    volumes:
      - ./nginx/:/etc/nginx/:ro
      - ./dashboard/:/srv/dashboard/:ro
      - ../image-editor/:/srv/draw/:ro
      - ../text-editor/:/srv/write/:ro
```

and each gets a path of its own,

```nginx
location = /write { return 301 /write/; }
location /write/ {
  alias /srv/write/;
}

location ~ /\. {
  return 404;
}
```

The deploy step is saving the file.
The second block matters more than it looks: a working tree has a `.git` in it, and I would
rather not serve that.

## Paths instead of subdomains

Everything lives under one host: `/write`, `/draw`, `/audiobookshelf`, `/video` for Jellyfin, and
`/pihole`.
Static files do not care what path they are under, as long as their links are relative.
Services do care, and each one wants to be told in its own way:

- Jellyfin has a Base URL setting under Dashboard > Networking.
- Pi-hole takes `FTLCONF_webserver_paths_prefix`, and expects the proxy to strip the prefix.
- Audiobookshelf only supports `/audiobookshelf`, which at least means there is nothing to configure.

One app of mine only works at the root of a host, so it gets a port of its own instead of a path.

## HTTPS, because the browser insists

The apps worked fine over plain HTTP on the LAN, until they didn't.
Saving back to the file you opened (the
[File System Access API](https://developer.mozilla.org/en-US/docs/Web/API/File_System_API)) and
writing to the clipboard are both limited to
[secure contexts](https://developer.mozilla.org/en-US/docs/Web/Security/Secure_Contexts), which
means HTTPS or `localhost`.

[Tailscale Serve](https://tailscale.com/kb/1312/serve) fixes this in two commands,

```shell
tailscale serve --bg http://127.0.0.1:80
tailscale serve --bg --https=8443 http://127.0.0.1:7474
```

Tailscale terminates TLS with a real certificate for the machine's tailnet name and forwards to
`nginx`.
The configuration is kept by `tailscaled`, so it survives restarts.
The only change on my side was to stop `nginx` from listening on 443, which Tailscale needs.

As a bonus, the apps are now on my phone, wherever my phone is.

## A dashboard that edits itself

With everything under one host, `/` was free for a dashboard.
It is also a folder of files: a static page that reads its layout from `apps.json`.

I wanted to rearrange it from the browser without adding a backend.
It turns out `nginx` can accept a `PUT` for exactly one file,

```nginx
location = /data/apps.json {
  root /srv/dashboard;
  dav_methods PUT;
  client_max_body_size 64k;
  add_header Cache-Control no-store;
}
```

The dashboard has an Edit button, and Done writes the JSON back.
The layout is the same on every device and it is a file I can commit.
Anyone who can reach the dashboard can rewrite it, which on a tailnet of one person is a risk I
am comfortable with.

## Publishing

The apps live in my monorepo, and [git-orchard](https://github.com/jmelahman/git-orchard) mirrors
each directory to a repository of its own.
Those repositories ([write](https://github.com/jmelahman/write),
[draw](https://github.com/jmelahman/draw)) are served by GitHub Pages as they are.
With no build step there is nothing to run in CI.

## Things that bit me

**Firefox has no save dialog for web pages.**
The File System Access API is Chromium-only.
In Firefox, "Save" is a download and "Open recent" can only reopen a copy kept in the browser, so
both needed a fallback.
Most of my Firefox bugs came from writing the nice path first.

**Caching.**
The public site sits behind Cloudflare, which held on to an old `style.css` for four hours while
serving a new `index.html`.
A no-build site has no hashed filenames to save you.

**Tailnets belong to accounts.**
I renamed my tailnet and nothing changed, because the server had been logged in with a different
account and was sitting in another tailnet entirely.

## Was it worth it?

Each app is a few files I can read top to bottom, they load instantly, and they will keep working
for as long as browsers do.
The whole "platform" is one `nginx.conf` and a two-line script.

I would do it again.
