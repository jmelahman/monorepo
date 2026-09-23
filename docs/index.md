---
layout: home

hero:
  name: local-preview
  text: A preview deployment per commit.
  tagline: Register a git repo and every commit gets its own servable URL, built once and served from a single Go binary on your own machine.
  image:
    light: /demo_light.png
    dark: /demo_dark.png
    alt: local-preview dashboard screenshot
  actions:
    - theme: brand
      text: Get Started
      link: /guide/install
    - theme: alt
      text: Quickstart
      link: /guide/quickstart
    - theme: alt
      text: View on GitHub
      link: https://github.com/jmelahman/local-preview

features:
  - title: A URL for every commit
    details: Previews are served at <code>&lt;sha&gt;.&lt;repo&gt;.preview.localhost</code>. Browsers resolve <code>*.localhost</code> on their own, so there is no DNS setup, and the same routing works under a real wildcard domain later.
  - title: Content-addressed builds
    details: A commit resolves to a frontend and a backend artifact, hashed from the git tree. Commits that don't touch a side reuse the existing artifact, so a docs-only commit deploys in milliseconds.
  - title: Shared backends, on demand
    details: One supervised process per distinct backend version, started on the first request. Every commit that shares the version shares the process, so hundreds of previews need only a handful of processes.
  - title: State follows git lineage
    details: A new backend version forks its database from the nearest deployed ancestor. Commits on one branch feel like a single persistent database, and diverged branches can't corrupt each other.
  - title: Triggers are adapters
    details: Deploying is one API call, wrapped by the CLI and a git post-commit hook. <code>preview install-hook</code> makes every commit deploy itself.
  - title: One small binary
    details: Reverse proxy, build queue, artifact store, process supervisor, REST API, CLI, and dashboard ship in one self-contained Go binary with SQLite.
---
