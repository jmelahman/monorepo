# Embedding as a library

Everything the standalone server does is available to other Go applications
through the `orchestrator` package: the same pipeline behind a small
facade, wired into your own HTTP server. The first consumer is
[agentic-kanban](https://github.com/jmelahman/agentic-kanban), which serves
a preview per ticket branch.

```go
import "github.com/jmelahman/local-preview/orchestrator"

orch, err := orchestrator.New(orchestrator.Options{
    DataDir: filepath.Join(dataDir, "previews"),
    Addr:    ":8080", // your server's listen address (for preview URLs)
})
if err != nil { ... }
defer orch.Close()

// Requests to <sha>-<repo>.preview.localhost are served as previews;
// everything else falls through to your application.
srv := &http.Server{Addr: ":8080", Handler: orch.WrapHost(appHandler)}
```

Register repos and trigger deploys from your application's own lifecycle:

```go
// Idempotent: safe to call on every startup.
orch.RegisterRepo(ctx, "myapp", "/path/to/repo")

// Branch, tag, or sha; idempotent per commit.
d, err := orch.RequestDeploy(ctx, "myapp", "feature-branch", false)
// Poll orch.Deploy(d.ID) until StatusReady, then use d.PreviewURL.
```

Notes:

- The orchestrator keeps everything under `Options.DataDir` — its own
  SQLite file, mirror clones, artifacts, state, and logs. Give it a
  directory inside your application's data dir; nothing is shared with your
  schema.
- Worktree branches of a registered repo are deployable with no extra
  plumbing: worktrees share the repo's object store, so the orchestrator's
  mirror fetches them like any branch.
- `Deploy.PreviewURL` is built from `Options.Addr` and
  `Options.PreviewDomain`, which assumes clients reach your server directly.
  If it sits behind a TLS-terminating proxy, set
  `Options.PreviewBaseURL: "https://preview.example.com"` — its scheme, host,
  and port replace the `http://` and listen port that would otherwise be
  guessed. It supersedes `PreviewDomain`; setting both to different hosts is
  an error from `New`.
- `Close()` stops build workers and every supervised backend; previews
  never outlive the embedding process.

## Storage and retention

Every commit previewed leaves artifacts, backend state, and logs behind, so
an embedded instance grows without bound unless something reclaims them. A
retention sweep runs hourly (`Options.RetentionInterval`; negative disables
the background pass), but it evicts nothing until you set a policy:

```go
orch.SetRetentionPolicy(orchestrator.RetentionPolicy{
    MaxDeploysPerRepo: 10, // 0 = unlimited
    MaxAgeDays:        30, // 0 = unlimited
})
```

Eviction reclaims a deploy's bytes and keeps its row as history — a
redeploy revives it. Never evicted: queued and building deploys, each
repo's newest ready deploy, and branch-alias targets. Even with retention
disabled, every sweep collects stale staging leftovers.

```go
report, _ := orch.Storage()      // bytes by category and by repo
result, _ := orch.CollectGarbage() // sweep now; reports what it freed
```

`Storage()` walks the data dir on each call, so surface it behind a user
action rather than a poll.

## Listing deploys

`Deploys(repo)` returns everything. Because eviction keeps a deploy's row as
history, that list grows for as long as the instance lives even while
retention holds bytes flat — so any UI that polls it should page instead:

```go
page, _ := orch.DeploysPage(orchestrator.DeployQuery{
    Repo:   "myapp",              // optional
    Status: orchestrator.StatusReady, // optional; omit for every status
    Query:  "fix-login",          // optional free-text search
    Limit:  25,
    Offset: 50,
})
page.Deploys // this page, newest first
page.Total   // every match, ignoring Limit/Offset
```

Paging is by descending deploy id, so a deploy created between two page
fetches shifts the window rather than corrupting it. Filtering out
`StatusEvicted` is how a caller shows only deploys that still have artifacts
on disk.

`Query` is what a search box sends: it matches a case-insensitive prefix of
the commit sha — so a short sha copied off a listing row finds its deploy —
or a case-insensitive substring of the repo name, branch, ref, author name,
or author email. Wildcard characters match themselves, so user input needs
no escaping. Every field narrows the same result set, so `Query` composes
with `Repo` and `Status` rather than overriding them.

## Custom manifest locations

By default target repos declare themselves in `preview.toml`. Embedders can
accept the same schema from inside their own config file so repos need only
one file:

```go
orchestrator.Options{
    // preview.toml wins when both exist; sources are tried in order.
    ManifestSources: []orchestrator.ManifestSource{
        {Path: "preview.toml"},
        {Path: ".kanban.toml", Table: "previews"},
    },
}
```

Hashes cover the parsed manifest rather than its location, so relocating
unchanged config busts no caches.

`Options.LocalManifestDir` adds a last-resort out-of-repo source: when no
in-repo source matches, `<dir>/<repo>.toml` (plain `preview.toml` schema) is
read from the server's disk at build time — onboarding repos whose upstream
can't carry a manifest.

## Custom build execution

By default build steps run in the environment the manifest and commit
declare — the side's `image` when set, else the repo's
[devcontainer](/reference/preview-toml#devcontainer-default), else the
host. Inject `Options.Runner` to replace that execution entirely:

```go
type dockerRunner struct{ /* your container client */ }

func (r dockerRunner) Run(ctx context.Context, spec orchestrator.RunSpec, out io.Writer) error {
    // Mount spec.ScratchDir into a container, run spec.Argv with the
    // working directory spec.Dir inside the mount, stream output to out.
}
```

`RunSpec` carries the repo name and sha, so a runner can pick (and cache)
the right environment image per repository. It also carries the resolved
`Devcontainer` (image, cache-volume mounts, remote home) so a custom
runner can honor the same default without re-parsing devcontainer.json —
and `Image`, which custom runners should still respect when set: an
explicit image beats environment discovery.
