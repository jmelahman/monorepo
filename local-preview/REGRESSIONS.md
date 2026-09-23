# Regressions

Long-form write-ups of bugs that have bitten this project and are likely to
recur. Each entry gets a one-line tripwire in `CLAUDE.md`'s "Recurring
regression notes" section pointing here, so anyone touching the affected area
reads the full story first.

An entry should cover: the symptom, the root cause, why the fix works, and
what kind of change would reintroduce it.

## Container tests in CI see the runner host's daemon, not their own filesystem

**Symptom.** `TestAutoRunnerContainer` passed locally but failed on CI with
`container output on host: "", open .../out.txt: no such file or directory` —
the build container ran fine (its workdir output appeared), yet the file it
wrote never showed up in the test's scratch dir.

**Root cause.** The `golang` CI job runs inside the devcontainer image, and
GitHub Actions mounts the runner host's Docker socket into job containers.
Any container the test launches is therefore a *sibling* on the host daemon,
and bind-mount paths in `HostConfig.Binds` are resolved on the **daemon's
host**, not inside the CI job container. The scratch dir under the job
container's `/tmp` doesn't exist on the host, so the daemon auto-creates an
empty host-side directory, the build container writes its output there, and
the test process never sees it.

**Fix.** The test probes the precondition it actually needs: it writes a
marker file into the scratch dir and runs a throwaway container that checks
the marker is visible through a bind mount. If the probe fails, the daemon
doesn't share the test's filesystem and the test skips. Real coverage is
preserved anywhere docker is local (dev machines, docker-in-docker).

**What would reintroduce it.** Any new test (or product path) that launches
a container and expects bind-mounted output to round-trip will break the
same way whenever the code runs inside a container that talks to an outside
daemon — CI job containers, devcontainers with the socket mounted, remote
`DOCKER_HOST`. Gate such tests on the same marker-file probe. For product
code, note that `autoRunner` has the same blind spot: builds "succeed" but
outputs land on the daemon host.

## Side publishes rename their subtree out of the shared scratch dir

**Symptom.** Downloadable-artifact builds failed with `chdir
<scratch>/backend: no such file or directory` when they ran after the
backend build of the same deploy — even though the directory was extracted
and the backend build had just used it successfully.

**Root cause.** All of a deploy's builds share one extracted commit tree,
and `store.publish` lands artifacts with an atomic `os.Rename` of the built
subtree — `PublishBackend` moves `<scratch>/<backend.path>` itself into the
artifact store, and static/process frontends do the same with their dist or
path tree. Publishing a side therefore *consumes* part of the scratch tree;
any later step that reads it (a build cwd, declared output files) finds it
gone.

**Fix.** Originally, downloadable artifacts built first: their publish only
*copies* the declared files out, leaving the tree intact for the frontend
and backend publishes that follow. Since artifacts moved to a post-ready
build phase (they must not gate the preview), they take the other escape
hatch instead: `buildArtifacts` extracts its own tree and never touches
`buildDeploy`'s scratch dir.

**What would reintroduce it.** Adding a new output kind, reordering the
build sequence, or any post-publish step that touches the scratch tree
(checksums, manifests, uploads) after `PublishFrontend`/`PublishBackend`
ran. Anything that must read the extracted tree has to run before the
rename-based publishes — or take its own extraction, as the artifact phase
does.

## React `autoFocus` doesn't survive `<dialog>.showModal()`

**Symptom.** The `ConfirmDialog` confirm button carried React's `autoFocus`
prop so that pressing Enter right after the modal opened would confirm the
destructive action. It didn't: Enter cancelled instead, because focus sat on
the header close button.

**Root cause.** React implements the `autoFocus` prop as an imperative
`.focus()` call during commit — it does *not* render a real `autofocus`
attribute. `Modal` opens the dialog with `dialog.showModal()` from a *parent*
effect, which runs after the child button mounts. `showModal()` then runs the
native dialog-focusing steps, and with no `autofocus` attribute in the DOM to
delegate to, it focuses the first focusable descendant (the close button),
clobbering React's earlier `.focus()`.

**Fix.** Focus after the dialog is modal, not before. `Modal` calls
`dialog.querySelector("[data-autofocus]")?.focus()` immediately after
`showModal()`, and the control that wants initial focus opts in with a plain
`data-autofocus` attribute. Running last, this focus sticks. Using a data
attribute (not `autoFocus`) also sidesteps Biome's `a11y/noAutofocus` rule.

**What would reintroduce it.** Reaching for the `autoFocus` prop on any
control inside a `<dialog>` that opens via `showModal()` — it will lose the
focus race every time. Mark the target with `data-autofocus` instead. Same
trap applies to focusing in a child effect: child effects run before the
parent's `showModal()`, so that focus is clobbered too.

## On-disk leftovers must never gate DB-owned decisions

**Symptom.** A repo deleted from the "Registered repositories" modal could
not be re-registered: `POST /api/repos` failed with "already exists" even
though the repo was gone from the DB and the UI. With the registration gone,
`DELETE /api/repos/{name}` 404s — a permanent dead end with no UI escape.

**Root cause.** `gitrepo.Add` refused to clone when `repos/<name>.git`
already existed on disk. Deletion removes the mirror clone best-effort
*after* the DB rows, and the mirror can survive it: an in-flight fetch
(watcher poll, deploy ref-resolution) re-creates `refs/`/`objects/` subdirs
while `RemoveAll` walks the tree, and `--in-memory` sessions register
mirrors into the persistent data dir but forget the rows on shutdown. Either
way the name was bricked by state the DB no longer knew about.

**Fix.** The repos table is the sole authority on name ownership. `Add` now
clones into a temp dir inside the repos dir and swaps it over `<name>.git`,
replacing any orphaned leftover (temp-then-rename so a failed clone never
destroys a healthy mirror). The `409` conflict comes only from the DB check.

**What would reintroduce it.** Any new code path that treats presence on
disk (mirror clones, artifact/state/log dirs) as proof of registration, or
that errors instead of overwriting when creating a resource whose uniqueness
the DB already enforces. Disk contents are a cache; rows are the truth.

## A startup backlog must not be enqueued from the goroutine that serves HTTP

**Symptom.** After an instance replacement the orchestrator container ran with
zero CPU, printed nothing, never listened on its port, and the load balancer
returned 502 indefinitely. `SIGQUIT` showed goroutine 1 parked in `[chan send]`
at `build.(*Queue).enqueue`, called from `Queue.Start`.

**Root cause.** `Start` re-enqueued every unfinished deploy *before* launching
its workers. `work` is buffered at 256, so a data volume carrying more than 256
`queued`/`building` rows — trivially reached by a repo-wide backfill that was
interrupted — filled the buffer and blocked the caller. That caller is `run()`,
which had not yet reached `ListenAndServe`. Nothing could drain the channel and
nothing could accept the request that would have cancelled the work.

**Fix.** `Start` launches its workers first, then resumes the backlog from a
goroutine whose send selects on `ctx.Done()`. Workers drain behind it, and a
backlog of any size delays only its own resume.

**What would reintroduce it.** Any bounded-channel send on the startup path
before the server listens, or a `Start`-time loop that assumes the DB holds
fewer rows than a buffer. Startup work that can be proportional to accumulated
state belongs in a goroutine, and its sends need a cancellation arm.

## Uploads must hash only the side being uploaded

**Symptom.** `preview upload frontend …` (or backend, or one artifact) failed
with an unrelated side's error — e.g. `artifacts.cli: artifact partition
matches no files at this commit` — even though the frontend it was publishing
was perfectly valid.

**Root cause.** `Queue.Upload` computed the target slot by calling
`resolveHashes`, which hashes *every* side of the commit (that's what
`buildDeploy` needs — it decides what to build across all sides). Any side
whose partition is empty or otherwise unhashable at that commit made the whole
call fail, so uploading side A depended on sides B and C being valid.

**Fix.** Split the per-commit `hashInputs` (devcontainer + `LsTree`) from the
per-side computations (`feHashOf`/`beHashOf`/`artHashOf`). `resolveHashes`
composes all three for the build; `Upload` calls only the one for the side it
is publishing. Both paths share the same per-side funcs, so an upload still
lands byte-for-byte in the slot a build would target.

**What would reintroduce it.** Having `Upload` (or any single-side path) reach
for `resolveHashes` again for convenience. Hash exactly the side you're
touching — never the whole commit — unless you genuinely need every hash.

## GitHub OIDC upload auth: custom audience, and verify before repo lookup

**Symptom (latent).** Two ways this feature can be quietly wrong. (1) If the
server's `--github-oidc-audience` is left at GitHub's *default* audience — the
repository owner URL, `https://github.com/<owner>` — then any workflow in the
org can mint a token with that `aud`, so the per-repo `source` binding is the
only thing standing between org repo A and uploading to registered repo B.
(2) If the repo were looked up *before* the token is verified, an
unauthenticated caller could probe which repos are registered by reading
404-vs-401 on the upload path.

**Root cause / fix.** (1) The audience must be a value unique to this server;
we default the client to request the server URL and document setting
`--github-oidc-audience` to the same. A custom audience scopes tokens to this
service so they can't be replayed from another workload in the org. (2)
`authorizeUpload` verifies the bearer token first and only then calls
`GetRepoByName` — an unauthenticated caller always gets `401`, never a repo-
existence signal. Authorization then binds `claims.repository` to the repo's
`source` via `normalizeGitURL`, so repo A's workflow can never upload to repo B.

**What would reintroduce it.** Documenting or shipping a default/empty audience;
or reordering the gate to resolve the repo before verifying the token.

## A dead process needs a record that outlives it

**Symptom.** A preview whose backend exited on its own — panic on startup, an
OOM kill, a bad migration — kept showing `idle` in the dashboard and the API,
the same badge as a deploy nobody had visited yet. The only hint that anything
was wrong was the preview not answering.

**Root cause.** `Status` reported on `m.procs`, and every exit path (`cmd.Wait`
watcher, container watcher, the start-failure `fail` closure) ends in
`m.forget`, which deletes the entry. Once the process is gone the manager holds
nothing about it, so "died two seconds ago" and "never started" are the same
observation: no entry, therefore `idle`. Correct for a supervisor that starts
on demand — the next request *does* boot it either way — but it discards the
one fact a user needs.

**Fix.** A separate `failures map[Key]Failure` that outlives the process. A
non-zero exit of a process that had gone healthy records via `noteExit`; a
start attempt that never got there records via `fail` (which knows what it was
waiting for). `Status` returns `crashed` when the key has no process but has a
failure, and the API/orchestrator surface the detail as `process_error`. The
record is cleared where it stops being true: on the next start attempt, on an
explicit `Stop`, and per-repo in `StopRepo`.

Exactly one path reports per lifecycle — `noteExit` returns early unless the
process was `isReady`, so a start attempt that dies before healthy belongs to
`fail` alone and can't double-report.

**What would reintroduce it.** Adding an exit path that calls `forget` without
recording, or clearing failures somewhere that isn't a real acknowledgement.
More generally: any UI state derived only from a live-process table inherits
that table's amnesia. If a state means "nothing here", check that it isn't
covering for something that was here and failed.

**Bug found on the way.** The health-timeout path called `isExited(p)` after
`<-p.done`, where every process looks exited, so the `health_timeout` event
never fired. Reading liveness after waiting for death answers a different
question than the one asked — sample it before the wait.

## Preview-access cookie must be stripped before proxying to the previewed app

**Symptom (latent).** With SSO gating previews, a browser holds a
`preview_grant` cookie scoped to `.<preview-domain>`, so it's sent to every
preview host. `httputil.ReverseProxy` forwards request headers — including
`Cookie` — verbatim, so without intervention the orchestrator hands its own
preview-access credential to the previewed repo's (untrusted) backend on every
proxied request. A malicious preview backend could then replay that cookie
value against the control plane from its own server-side HTTP calls, entirely
outside the browser's Origin/SameSite sandbox.

**Root cause / fix.** Two defenses, both required. (1) Preview access uses a
*separate* credential from the dashboard: `sessions.scope` is `'preview'` vs
`'apex'`, and the apex auth middleware only ever accepts `'apex'` sessions — so
a leaked preview cookie is useless against `/api/*` even if replayed. (2)
`ensureAndProxy`'s `Rewrite` calls `stripCookie(pr.Out, previewGrantCookieName)`
so the cookie never reaches the previewed process at all. The apex session
cookie is host-only (never `Domain`-scoped to the preview domain), so it's never
sent to a preview host in the first place.

**What would reintroduce it.** Scoping one cookie to cover both the apex host
and the preview subdomains; dropping the `stripCookie` call in the proxy
`Rewrite`; or letting the apex middleware accept a `'preview'`-scope session.

## An OIDC token authenticates nothing until it is bound to a named repo

**Symptom.** `preview upload frontend … --oidc --deploy` uploaded fine and then
failed with `POST /api/deploys: invalid or unauthorized token`. The upload
endpoints are auth-exempt and self-gate on OIDC, but `/api/deploys` went through
the session middleware, which treats *every* bearer token as a personal-access
token and verifies it against the GitHub user API. An OIDC JWT is not a PAT, so
it was rejected — making `--oidc` and `--deploy`, both documented as composable
global flags, unusable together. CI could publish an artifact but never deploy
it.

**Root cause / fix.** The middleware now falls back to OIDC verification when
the PAT check fails, and attaches the verified claims to the request context.
The subtlety is that verifying is not authorizing: a valid token proves only
which workflow minted it, and any repo in the org can mint one for a given
audience. So the fallback is gated on `oidcRoute` — currently just
`POST /api/deploys` — and `handleCreateDeploy` calls `oidcMayActOn` to require
that the `repo` in the body has the token's GitHub repository as its registered
source. Same rule the uploads already applied, same helper now.

**Caught the same day, one layer down.** The first fix allowed only
`POST /api/deploys`, on the theory that a by-ID route can't be bound to a repo.
That was wrong twice over: `--deploy` then polls `GET /api/deploys/{id}` and
failed there instead, and the deploy row names its repo, so the binding was
available all along. The read is now eligible too — but it answers a mismatch
with `404`, not `403`: deploy IDs are sequential, and a `403` would separate
"someone else's deploy" from "no such deploy" and make the table enumerable by a
token from any repo in the org.

**What would reintroduce it.** Adding a route to `oidcRoute` that has no repo to
bind against, or accepting the claims in a handler without checking the binding
— either authorizes every repo's CI for that route. On read paths, also
returning `403` where `404` is required.

## A derived runtime state can't be filtered by a stored-status predicate

**Symptom.** With the deployments list filtered to "ready", crashed deploys
showed up in it — the one state a user has to act on, hiding among the
previews that can still serve. `preview deploy list` had the mirror-image bug:
a crashed deploy printed as `ready`.

**Root cause / fix.** "Crashed" is not a build status. The row's `status`
column says `ready` (the build succeeded); the crash lives in the
supervisor's in-memory `failures` map, and both UIs derive the displayed
state by merging the row with the live process states. The filter, though,
was a plain `d.status = ?` — it only ever saw the column, so every crashed
deploy answered to `status=ready` and nothing answered to `status=crashed`.

The API now bridges the two: `Manager.CrashedKeys` returns the currently
crashed keys, the handler translates them into the deploy columns that
identify them (`db.ProcKey`), and `deployWhere` renders them as an
inclusion (`CrashedOnly`) or exclusion (`CrashedNone`) predicate. Keying
matters: a backend is identified by `be_hash` alone — every deploy sharing
that artifact is served by the same process, so all of them are crashed —
while a process-mode frontend needs the `fe_hash`/`be_hash` pair, since the
instance is specific to the backend it was started against. The predicate
goes through `deployWhere` precisely so `ListDeploys` and `CountDeploys`
can't disagree, which would corrupt the pager.

**What would reintroduce it.** Adding another derived state to the status
dropdown (`running`, `idle`, `starting` are the obvious next ones) and
wiring it straight to `DeployFilter.Status`, or filtering the fetched page
in the frontend instead — a client-side filter silently breaks paging and
`X-Total-Count`. Any state the DB doesn't store has to be resolved into a
row predicate before the query runs.

## A truncated artifact must never land under a content-addressed key

**Symptom (latent).** The durable artifact tier (`internal/s3store`) uploads
each build to S3 keyed by content hash, and `Save` skips the upload when the
key already exists (content-addressed: same key ⇒ same bytes). If a *partial*
object ever lands under that key, the skip makes it permanent — every future
`hydrate` extracts a corrupt tree, and a reconcile pass that only checks
existence never notices.

**Root cause / fix.** The persist worker reads the published store directory
*after* the build, and retention's `GCDeploy` can delete that directory out
from under it (eviction racing a fresh build of a shared hash). A streaming
`tarDir → zstd → PutObject` that returned the walk error without failing the
put could let the multipart *complete* with fewer bytes than intended. Two
guards prevent a bad object from materializing:

- `Save` compresses to a temp file first and only then puts it with a known
  size — a walk/tar failure (including `ErrSourceGone` when the source dir
  vanished mid-walk) aborts before any bytes reach the bucket.
- `Save` records the uncompressed byte size as object metadata; `Open` streams
  through a verifier whose `Close` fails if the decompressed length doesn't
  match. `hydrate` treats that failure like a miss and rebuilds.

`ErrSourceGone` is a *benign* drop, not a retryable error: GC won the race, so
there's nothing to persist — the next redeploy rebuilds and re-uploads.

**What would reintroduce it.** Switching `Save` back to a streaming pipe
without `CloseWithError` on the writer side; making `Save` record the object
*before* the bytes are known to be complete; or making the reconcile/back-fill
pass assert only `StatObject != nil` instead of checking the size metadata.
Also: a serve-only second node is a correctness (not efficiency) dependency on
that reconcile pass — it can't rebuild a missing artifact, so no artifact may
be missing before one exists.

## Absence of on-disk files must not be read as eviction

**Symptom (latent).** Once local disk is a *cache* of the durable tier
(`--cache-max-artifact-bytes`), two very different states look identical on
disk — the artifact directory is simply absent:

| | Decided by | Correct user-facing outcome |
|---|---|---|
| **Evicted** | DB (`retain` policy marked the deploy evicted) | proxy 410 "cleaned up" page |
| **Not resident** | cache watermark swept a *live* deploy's artifact | nothing — hydrate and serve |

If a `HasFrontend`/`HasBackend`/`HasArtifact` call site treats "absent" as
"evicted", a live deploy whose artifact was swept for space serves a 410
instead of transparently re-hydrating. This is the exact inverse of the older
rule *"on-disk leftovers must never gate DB-owned decisions"* — here a DB-owned
decision (evicted) must not be inferred from disk absence.

**Root cause / fix.** Eviction is a DB fact; residency is a cache fact. The
serving path (`supervise.Manager.start`) no longer fails when `spec.dir` is
missing — it calls `store.Hydrate` first and only reports the artifact gone if
the *tier* also lacks it. `store.Hydrate` returns `ErrNotInTier` for a genuine
miss (distinct from "no tier configured"), so callers can tell a reconcile gap
from a plain single-node instance. Cache eviction uses a dedicated
`EvictCacheToWatermark` — **not** `RemoveBackend`, whose doc invariant ("confirm
no surviving deploy still uses the hash first") deliberately does not apply when
the deploy stays live and only the local copy goes away. Eviction is a no-op
when no tier is configured (local disk is then the only copy), skips artifacts
younger than a grace period (their background persist may still be in flight),
and skips artifacts with a live process (a container may have them bind-mounted).

**What would reintroduce it.** A new `Has*`-then-410 path on the serving side
that skips the hydrate attempt; wiring cache eviction through `RemoveBackend`
(which also nukes the mutable state dir — never in the tier); evicting when
`tier == nil`; or a reconcile pass that lets an artifact leave disk before it is
known to exist durably (a serve-only node cannot rebuild — Phase 1b is the
correctness net, not an optimization).

## A `nofail` data volume must gate the service that bind-mounts it

**Symptom.** After a reboot, the orchestrator comes up with no registered
repos, no deploys, and an empty dashboard — indistinguishable from total data
loss. The data volume is fine and even mounts correctly moments later; the
running server just never sees it.

**Root cause / fix.** cloud-init writes the data volume to `/etc/fstab` with
`nofail`, so a boot where the volume is slow to attach proceeds without it
rather than dropping to emergency mode. `local-preview.service` declared only
`Requires=docker.service`, so on such a boot it started anyway and
bind-mounted the empty host directory into the container — where the server
created a fresh SQLite database. Mounting the volume afterwards does not
repair it: a bind mount resolves its source once, at container start, and does
not follow a later mount at that path. The fix is `RequiresMountsFor=` on the
unit, which the `local-preview-stack@` unit beside it already carried — the
orchestrator unit had simply been missed.

This stayed latent for as long as the host booted once, at create time, where
cloud-init mounts the volume synchronously before writing the unit. It becomes
a daily exposure the moment reboots are routine — an instance scheduled off
outside business hours, an ASG replacing a node, or a spot reclaim.

**What would reintroduce it.** Any new unit that bind-mounts the data dir
without `RequiresMountsFor=`; dropping `nofail` from fstab and assuming
ordering is therefore guaranteed (it makes a failed mount block boot instead,
which is a different failure, not an equivalent one); or relying on
`After=network-online.target` as a proxy for the volume being ready.

## An unknown `user_data` silently defeats `user_data_replace_on_change`

The example module sets `user_data_replace_on_change = true` on
`aws_instance.server`, and that setting is the only thing that makes a config
change reach a running box: `user_data` is a cloud-init script, cloud-init runs
once per instance, and neither an in-place update nor a stop/start re-runs it.
Replacing the instance is how the unit file gets rewritten.

Terraform can only trigger that replacement if it can *compare* the new
`user_data` to the old one at plan time. Feed the template a value that is
unknown until apply — the classic case being a bucket name taken from a
resource created in the same run, `aws_s3_bucket.artifacts.id` — and the whole
rendered script becomes `(known after apply)`. There is nothing to compare, so
the replacement is not planned. Terraform plans `will be updated in-place`
instead, the apply succeeds, `user_data_base64` is faithfully updated in state
and in the instance attribute, and *nothing on the machine changes*. The new
image is never pulled and the new flags are never passed. The failure is
entirely silent: a green apply, a clean plan afterwards, and a server still
running the old configuration.

This bit the S3 artifact-tier rollout, where `--s3-bucket` was wired straight to
the bucket resource. The fix is to keep every value that lands in `user_data`
known at plan time — put the bucket name in a `locals` literal and let the
`aws_s3_bucket` resource take *its* name from the same local, rather than the
other way round. The plan then reads `must be replaced`, which is both correct
and visible.

Watch for it whenever a new server arg is threaded through `extra_server_args`.
The tell is in the plan: an instance whose `user_data_base64` shows
`-> (known after apply)` under `will be updated in-place` is a no-op apply, not
a config change.

## Every path that extracts a committed tree must reject escaping symlinks

`gitrepo.Archive` walks the *full committed tree* of an untrusted target repo
and, for each symlink entry, recreated it verbatim with `os.Symlink(linkTarget,
target)`. `linkTarget` is the blob content — fully attacker-controlled by
anyone who can get a commit built (a push to a watched branch, a webhook, a
deploy request). A committed `dist/leak -> /etc/passwd` (or a `../`-escape) was
recreated in the build scratch dir and then *followed* by two serving sinks:
`http.FileServer(http.Dir(FrontendDir(...)))` streams the host file to any
anonymous preview visitor (previews are public unless SSO is configured), and
`PublishArtifactFiles` did `os.Stat`+`copyFile`, publishing the target's real
content as a downloadable artifact. Either one discloses the orchestrator's
SQLite DB (session-token hashes), env secrets, and mounted cloud credentials to
an unauthenticated network client.

The hardened tar extractor (`store.ExtractTar`) already defended against exactly
this — reject an absolute link target, and refuse any relative target that
resolves outside the extraction root (`withinRoot`) — but that hardening was
never applied to `gitrepo.Archive`. The fix mirrors it: `extractSymlink` now
takes the absolute `root`, rejects absolute targets, and refuses a target whose
`filepath.Join(dir(target), link)` escapes root. Defense in depth in
`PublishArtifactFiles`: `os.Lstat` (not `os.Stat`) the declared source and
refuse a symlink outright — a build's declared `files` must be genuine outputs.

The lesson generalizes: *any* code that materializes an untrusted repo tree or
archive onto disk is a link-following surface. There must be exactly one
hardened rule (absolute-reject + within-root), and every extractor must apply
it — a tar extractor being hardened says nothing about a git-tree extractor
sharing the same scratch dir and the same public file server.

**Carve-out, learned when the rule broke production hydration:** the rule's
danger model is a *trusted host-side reader following the link* — the public
frontend file server and the artifact publisher. A **backend artifact** is an
executed payload with no such reader (run containers resolve links in their own
filesystem; the persist tar-writer Readlinks without following; eviction
removes without following), and a venv legitimately carries absolute symlinks
into its run image (onyx: `.venv/bin/python → /opt/uv/...`). Blanket-rejecting
those made every backend hydrate and CI upload fail. `ExtractTarPayload` lifts
the symlink-target restriction for backend trees only; entry names and
hardlinks (which `os.Link` resolves host-side at extract time) stay strict
everywhere, and frontend/artifact trees keep the full rule. Do not "simplify"
the two policies back into one in either direction.

## A worker resolves run specs from the wire, not its own DB

Artifact *files* travel to a worker through the S3 tier (hydrate-on-serve),
but the run *contract* — argv, run image, devcontainer, env, health path,
timeouts, init steps, `InitDoneAt`, the state dir — lived only in the control
node's `frontend_artifacts`/`backend_artifacts` rows. A `--role=worker` node
has a fresh, empty SQLite DB, so `loadRunSpec` failed "artifact not
provisioned" and every remote ensure was a 502. The transport tests never
caught it because `workerapi_test` exercised only a `fakeSup` — the HTTP
plumbing was covered, a real `supervise.Manager` on a row-less node never was.

The fix keeps the control node the single authority and the worker a dumb
executor: the control node resolves `supervise.WireSpec` from its DB
(`ResolveWireSpec`) and ships it inside every ensure request (plus the peer
backend's spec for a process-mode frontend, which the worker starts itself for
`{backend_url}`); the worker offers it to its Manager (`OfferWireSpec`), whose
spec lookup consults the DB first and falls back to the wire cache. Rules that
must hold:

- **The state dir never travels as a path.** The control row stores an
  absolute control-node path; each node recomputes it from identity
  (repo, hash) against its own store root, and creates it fresh — lineage
  forking is a build-time, control-node concern (documented `{state_dir}`
  worker limitation).
- **Wire `InitDone` is sticky per node.** The control node has no record of an
  init that ran on a worker and re-sends `InitDone=false` forever; without
  stickiness in `OfferWireSpec`, every cold start re-runs init.
- **Keep the regression test honest**: `TestEnsureWireSpecOnEmptyDB` drives a
  real Manager over an empty DB through the real Client/Server. A fake
  supervisor can never see this class of gap.

## Every publish path must persist to the durable tier — reconcile is repair

`enqueuePersist` ran only on the build path; a CI *upload* published locally
and relied on the periodic reconcile pass to reach the S3 tier. On a worker
tier that gap is user-visible: a worker serves a freshly uploaded deploy by
hydrating from the tier, so every cold start between the upload and the next
reconcile tick failed "artifact not present in durable tier" → 502. The fix
enqueues a persist after each upload publish (fe/be/dl), same as a build.
The rule: reconcile exists to close crash windows and pre-tier history, not
to be any publish path's primary route to the tier — anything that lands an
artifact locally must enqueue its persist in the same breath.

**Sequel — `ready` must mean durable, not just enqueued.** Enqueuing the
persist closed the "never persisted" hole but not the *timing* one: the persist
pool is asynchronous, and `SetDeployReady` (which unlocks both auto-start and
the first on-demand serve) fired the instant the sides published locally. On a
worker tier that is still a race — a fresh commit whose backend was a cache-hit
(so its 300 MB upload had barely started) went `ready`, auto-started on a
worker, and the worker hydrated the peer backend from S3 ~30 s before the Save
finished: "not present in durable tier". The S3 object appeared seconds after
the error. The fix gates the ready transition on a *synchronous* `ensureDurable`
of fe/be before `SetDeployReady` (build.go `process`): `Save` is skip-if-exists,
so it only waits out the tail of the in-flight async upload and no-ops when the
pool already won. The invariant to preserve: **a deploy is not `ready` until a
serve-only worker could hydrate it** — never mark ready (or auto-start, or serve
on request) off a merely-published, not-yet-durable artifact. A persist failure
here fails the deploy loudly rather than shipping a `ready` row that 502s.

## Never mix inline SG rules with standalone rule resources on one group

`aws_security_group.server` defines its rules inline, and the worker tier
attached its deps-stack ingress as standalone
`aws_vpc_security_group_ingress_rule` resources on the same group. Terraform
treats an inline-rule SG as owning *every* rule on the group, so the next
in-place SG update — an unrelated instance retype — silently deleted the
standalone rules in production: workers could no longer open new connections
to postgres/opensearch/redis/minio, invisible until warm connections cycled.
The fix is one-owner-per-group: the stack ingress lives on its own
`aws_security_group.stack_ingress`, attached to the server instance alongside
the base SG. Any future rule added from outside the module must follow the
same pattern — a new group, never a rule resource pointed at an
inline-managed group.

## IMDS must be one hop out of a preview container's reach

**Symptom.** None yet — caught in a security review, not an outage. But the
exposure is direct: a preview runs arbitrary branch code, and that code could
reach the instance metadata service (IMDSv2) and mint credentials for the
node's IAM role — which grants read on the S3 artifact bucket and the SSM
`PREVIEW_SECRET_*` parameters (every dependency-stack password). A malicious
commit could exfiltrate all of them.

**Root cause.** `metadata_options` set `http_tokens = "required"` (IMDSv2) but
left `http_put_response_hop_limit` at the AWS default of **2**. The hop limit
is the IP TTL the metadata service allows on the token PUT: 1 means "only the
host itself," 2 means "the host and one network hop past it." A container on
the docker bridge is exactly one hop past the host, so hop limit 2 lets bridge
containers — every preview — talk to IMDS. The orchestrator itself runs
`--network host`, so it shares the host's namespace and reaches IMDS at hop 1
regardless; the deps stack and previews reach each other by **private IP**,
never IMDS. So nothing legitimate needs the second hop.

**Fix.** `http_put_response_hop_limit = 1` on every node's `metadata_options`
(control and workers). Host-network orchestrator: still works. Bridge-network
previews: blocked from IMDS, which is what we want.

**Reintroduces it.** Removing the line (reverting to the default 2), or
running the orchestrator in a bridge network instead of `--network host` and
"fixing" its lost IMDS access by raising the hop limit — that would re-open
the path to every preview. If the orchestrator ever must run off host
networking, give it credentials another way (a mounted role, a sidecar),
never a higher hop limit.

## Onyx SSO on previews: widen the cookie, validate the return marker

**Symptom class.** Two ways this breaks. (1) You log in on the canonical onyx
host but every per-commit preview still shows a login page — the session never
reaches them. (2) A crafted link (or a malicious previewed backend) sends a
just-logged-in user off to an attacker's site right after login.

**Why the session must be widened.** onyx's `CookieTransport` sets a
**host-only** session cookie (no `Domain`), so a cookie minted on
`app.<preview-domain>` is sent to `app.` only — not to `<sha>-onyx.<preview-
domain>`. The previews can only *validate* it (they run `AUTH_BACKEND=jwt`, a
stateless HS256 JWT signed by the shared `USER_AUTH_SECRET`, so no shared
session store is needed — but the browser still has to present the cookie).
The proxy therefore rewrites the canonical host's `Set-Cookie` to add
`Domain=.<preview-domain>` (`rewriteOnyxCookieDomain`). Only the auth cookie is
touched; CSRF/PKCE cookies stay host-only.

**Why the cross-host return can't live in onyx.** onyx's `sanitize_next_url`
rejects any `next` carrying a scheme or netloc (open-redirect guard), so after
login on the canonical host onyx can only land you back on the canonical host —
never on the preview you came from. The return hop is orchestrated by the proxy
instead: bouncing to login stashes an `onyx_return` cookie (domain-scoped, so it
survives the hop), and the canonical host consumes it once a session appears.

**The trap.** `onyx_return` is domain-scoped, so a **previewed backend can set
it** (a subdomain may set a parent-domain cookie). If the post-login redirect
trusted it blindly, any preview could redirect a freshly-authenticated user
anywhere. `safeOnyxReturn` requires the target host to be the preview domain or
a label under it before redirecting. Keep that check — do not "simplify" the
handoff by trusting the cookie value.

**Also.** The preview's own `/auth/login` is intercepted unconditionally (even
with a cookie present): single-tenant onyx renders a password form there, and
an expired session redirects to it, so letting it through reintroduces the
password login we bounce past. And logout is best-effort: onyx clears the
cookie host-only, which can't remove the domain-scoped one — acceptable only
because the JWT is stateless and the whole surface is team-gated.

**Sequel — the return-dance must widen the cookie too, not just redirect.** The
widening above hangs off `rewriteOnyxCookieDomain`, which only fires when onyx
*sets* the cookie — i.e. on a **fresh** login response. An **already-logged-in**
user never re-triggers that: they arrive at the canonical host already carrying
a host-only session, the return-dance (`serveReserved`) sees the cookie + a
valid `onyx_return` and 302s them back to the preview — but the browser's cookie
is scoped to `app.<preview-domain>` and is *never sent to the preview host*. So
`needsOnyxLogin` fires again on arrival, bounces back to the canonical login,
the dance fires again, and the browser loops `preview/app → canonical/auth/login
→ preview/app` forever (never showing a password form — it's a redirect loop,
not the disabled-auth symptom). The fix: the dance itself mints a
`Domain=.<preview-domain>` copy of the session (`setWidenedOnyxCookie`) before
redirecting. Request cookies carry no attributes, so mirror onyx's own
(HttpOnly, `Path=/`, Lax, Secure-per-deployment) and give it a TTL matching
onyx's session default — the JWT's `exp` is the real authority, so an over-long
cookie just forces a fresh login, never a stale-auth accept. Don't lean on the
login-response widening alone: it covers first login, not the returning user.

## Scale-from-zero is a demand signal, not a load signal; load must be suppressed at zero workers

The worker tier autoscales on two custom CloudWatch metrics the control node
publishes (`internal/cloudscale`). It is tempting to drive all of it off fleet
utilization, but utilization is **degenerate at zero workers**: with no capacity,
`fleet.Registry.LoadRatio` scores a saturated `1` — indistinguishable from a
fleet that is genuinely full. So it cannot tell "idle at zero" from "slammed",
and cannot launch the first node.

The keystone is therefore **`UnservedDemand`** — the count of **distinct
placement keys** that found no fresh worker (`ErrNoWorker`, tracked via
`unservedDemand`/`DrainDemand`). A request arriving with nowhere to run is the
unambiguous "someone wants a preview and there are zero workers" signal, and
`place()` only returns `ErrNoWorker` when there are zero *fresh* workers (it
falls back to `leastLoaded` when workers are merely full), so demand ≥ 1 means
exactly that. This metric is published **every interval, including 0**, so a gap
means the control node is down — the demand alarm treats missing data as
`notBreaching`.

Two lessons from the first production night, both of the same shape — **demand
is a boolean per preview, not a request counter, and the response to it is one
node, not a proportional fleet**:

- **Dedupe demand by placement key, and scale out by exactly +1.** The first
  cut counted raw `EnsureRunning` calls and fed a proportional step ladder
  (+1/+2/+3 by demand value). Within minutes of going live it launched three
  nodes for one deploy's worth of pre-warm traffic, then drained two of them
  idle. Both halves were wrong: retries of one preview (a refresh loop, a
  pre-warm retry, the be+fe sides of one deploy — the frontend co-places on its
  backend's key) inflate a raw counter with phantom load, and once *any* worker
  registers, `place()` stops returning `ErrNoWorker` at all, so every waiting
  preview lands on the first node to arrive — the extra nodes the ladder bought
  boot into a fleet that no longer reports demand. Distinct keys per drain
  interval; a single +1 step (`estimated_instance_warmup` covering worst-case
  boot-to-registration so the still-breaching alarm doesn't stack a second
  node); genuine breadth is `load_high`'s job, which has real utilization to
  act on.
- **Whatever registers demand must retry until the node it summoned arrives.**
  The post-build pre-warm (`autoStartDeploy`) was fire-once: at zero workers it
  registered demand, failed, and never came back — so the node its demand
  launched booted to nothing, idled, and was reclaimed ~15 minutes later,
  a pure-waste cycle on every push that landed while the fleet was at zero.
  It now retries `ErrNoWorker` (only that error — anything else means a worker
  existed and the start itself is broken) every 30s for ~8 min, which is also
  what keeps the demand signal alive while the node boots. Proxy-driven demand
  needs no such loop — the human refreshes.

`FleetLoad` (utilization %) drives scale-out-under-load and scale-in-when-idle,
but it must be **published only when at least one worker is fresh**
(`LoadSample` returns `(0, 0)` at zero workers, and `publishMetrics` skips the
datum). Publish a `0` at zero workers and the scale-in alarm fires on an empty
fleet it can do nothing about, sitting in ALARM forever. The scale-in alarm
likewise treats missing data as `notBreaching` — a gap means zero workers, and
there is nothing to reclaim.

Two more invariants around it:

- **Drain-aware scale-in via instance protection, keyed off the fleet, not an
  AWS query.** The control node scale-in-protects a worker with `Running > 0`
  (`BusyByInstance`) so an ASG scale-in drains an idle node instead of killing a
  preview. It derives busy-ness from the live registry, so the IAM grant is only
  `cloudwatch:PutMetricData` (namespace-conditioned) + `autoscaling:SetInstance-
  Protection` (ASG-scoped) — **no** `DescribeAutoScalingInstances`. Don't add a
  describe call and widen the role; the fleet already knows who is busy.
- **The ASG name reaching `user_data` must be a plan-time literal.** The control
  node's `--worker-asg-name` is rendered into `user_data`; use the literal
  `"${var.name}-worker"`, never `aws_autoscaling_group.worker[0].name`. A
  resource reference renders the script `(known after apply)` and silently
  defeats `user_data_replace_on_change` — the same trap as the bucket-name entry
  above.

## Manifest env is baked into the artifact at build time — an env-borne fix never reaches existing deploys

The first stress test exhausted the shared deps Postgres (`sorry, too many
clients already`): every onyx api server holds a sync and an async SQLAlchemy
pool, so a handful of previews with default pools (40+10 per engine) ate
`max_connections`. The fix had two halves with very different reach:

- **Server-side settings are retroactive.** `max_connections`, and later
  `idle_session_timeout`, apply to every preview the moment the server has
  them — old artifacts included.
- **Manifest env is not.** The env block is part of the artifact hash and is
  baked in at build time, so capping pools via manifest env
  (`POSTGRES_API_SERVER_POOL_SIZE`) fixes only deploys built *after* the
  change. Everything already built keeps the fat pools until something
  produces a new artifact — which the env change itself guarantees for any
  *redeploy* (env → new hash → rebuild), but nothing rebuilds existing
  deploys spontaneously. The stress fleet promptly exhausted the raised 600
  cap with old-env artifacts.

So: an operational fix delivered through manifest env MUST be paired with a
server-side guard that covers the legacy artifacts. Here that guard is
Postgres `idle_session_timeout=10min` — safe because onyx defaults
`pool_pre_ping=true`, so a server-side kill of a parked connection is
invisible (the pool re-dials on next checkout). Set via `ALTER SYSTEM` (lives
on the data volume) and mirrored in the stack's command line for fresh
volumes.

## Worker re-registration must be idempotent — replacing live state blinds the fleet

**Symptom.** (Found by external review, confirmed against the code.) Healthy
workers intermittently vanished from placement: cache-affinity broke, the
scale-in reconciler churned redundant SetInstanceProtection calls, and — on a
small fleet — a request could land in a window where every worker read stale,
get the waking page, and register phantom unserved demand against a fleet
that was fully up.

**Root cause.** Workers re-announce every ~20s (so a restarted control node
re-learns them), and `fleet.Registry.Add` replaced the `workerState` on every
call: zero `lastBeat` (not fresh until the next 10s heartbeat poll — blind
~a quarter of wall-clock), zero hit-counter banks, `reachable=false`.

**Fix.** `Add` is idempotent for a known (id, instanceID) pair and reports
`isNew` so the registrar only logs genuine joins. The one replacing case is
the same endpoint returning with a different instance-id — an ASG replacement
reusing the IP is a different machine, and heartbeat-blind fresh state is
exactly right for it.

**What would reintroduce it.** Any periodic "refresh" path that recreates
live-tracked state instead of updating it — the same shape as the
on-disk-leftovers rule: re-announcement is a liveness signal, not a reset.

## Fleet statistics must ship from where the data is born

**Symptom.** The dashboard's startup-latency percentiles read empty on the
fleet deployment: `handleStats` reads the control node's `process_events`,
but processes run on workers, which record events into their own — ephemeral
— databases.

**Fix.** Workers buffer `ProcEventRecord`s (capped) and drain them into the
heartbeat (`Heartbeat.Events`, at-most-once — a lost response loses its
batch, an accepted trade for statistics). The control's heartbeat loop hands
batches to an event sink that replays them via `AddProcessEventAt`,
**preserving the original `occurred_at`** — the percentiles are timestamp
deltas between paired rows, so stamping at replay time would read every
fleet cold start as ~0s.

**What would reintroduce it.** Any new dashboard statistic derived from a
control-DB table that fleet-mode writes on workers (the warm/cold hit
counters already ride the heartbeat for the same reason) — and any replay
path that lets the database default a timestamp the metric depends on.

## A worker's init result must flow back to the control DB, or init re-runs on every fresh worker

**Symptom.** On the fleet deployment (onyx previews), the manifest's backend
`init` steps re-ran on nearly every idle→starting wake — visible as an init
phase on the starting page and needless cold-start latency. The heavy work
was skipped (the per-artifact database already existed, alembic no-op'd), so
it looked like "the seed restores more than it should" rather than a bug.

**Root cause.** `init_done_at` lives in the control node's DB, but
`markInitDone` only writes the DB when the init ran on the node that owns the
row (`!spec.fromWire`). On a worker, completion went into the in-memory
`wireSpecs` cache alone, and nothing carried it back — `ensureResp` was
port-only and the fleet path never marked. On a control node that only
routes (the production shape), `init_done_at` was never set for any
artifact, every ensure shipped `InitDone=false`, and only `OfferWireSpec`'s
per-node stickiness masked it. A fleet resting at zero workers made fresh
workers the common case, so the mask almost never applied.

**Fix.** No wire change: a successful backend ensure already proves init
succeeded (the worker 502s the ensure otherwise), so `workerapi.Client`
gained an `InitMarker` hook — `supervise.AdoptRemoteInitDone`, wired where
`SpecResolver` is — invoked when a backend ensure succeeds and the shipped
spec had `InitDone=false`. Works against old workers too. Frontend ensures
never mark: they only start the co-placed backend when `{backend_url}` is
referenced, and the proxy ensures the backend directly on API routes anyway.
Adoption is sound because init is once per ARTIFACT, not per node — its
effects live in external services keyed on `{hash}`; `{state_dir}` in init
on workers is already loudly unsupported.

**What would reintroduce it.** Any new per-artifact fact recorded where the
work happens (a worker) but consulted where placement happens (the control
DB) — same family as "fleet statistics must ship from where the data is
born". And any init semantics change that makes init effects node-local:
adoption then skips work node B genuinely needs.

## server_args render unquoted into systemd ExecStart — a spaced value crash-loops the service

**Symptom.** Deploying 0.30.0 with `--min-warm-window "Mon-Fri 08:00-18:00
America/Los_Angeles"` in `extra_server_args` took the whole control plane
down: the orchestrator container exited on unknown positional args and
systemd restart-looped it (~5 minutes of dashboard + preview outage until a
hand-edit of the unit).

**Root cause.** The terraform module joined `server_args` with spaces into
the unit's `ExecStart` line. systemd word-splits that line, so a flag value
containing spaces became one flag and two stray positional args.

**Fix.** Every arg is double-quoted at the three `join(" ", ...)` sites
(control unit, static and elastic worker units) — systemd's ExecStart
parser honors double quotes. The trade documented in place: args must not
themselves contain double quotes or `$` (the unit travels through an
unquoted heredoc).

**What would reintroduce it.** Any new template that splices a joined arg
list into a shell or systemd line without per-arg quoting — the next
multi-word flag value hits the same split.

## Response-writer wrappers silently drop Flush/Hijack/deadline control

**Symptom.** Found while building `preview exec` (a WebSocket endpoint on
the apex): the upgrade handshake failed with "hijack not supported" even
though the gzip middleware explicitly forwarded `Hijack`. The same wrapper
chain was silently degrading SSE through the preview proxy — `Flush` calls
never reached the real connection.

**Root cause.** `logRequests`'s `statusRecorder` embeds only
`http.ResponseWriter`. Embedding an interface promotes just that
interface's methods, so the wrapper erased the underlying writer's
`Flusher`/`Hijacker` capabilities, and — with no `Unwrap` method —
`http.ResponseController` couldn't reach past it either. The gzip
middleware's own `Hijack` asserted its inner writer directly and hit the
recorder, not the connection. The chain looked correct because each layer
compiled; the capability loss only shows at runtime.

**Fix.** `statusRecorder` forwards `Flush` and `Hijack` and exposes
`Unwrap() http.ResponseWriter`; `gzipWriter` gained `Unwrap` too, so
`ResponseController` can walk the chain for what the wrappers don't
intercept (per-request deadline clearing, which hijacked exec sessions need
to survive the server's 60s `ReadTimeout`). `gzipWriter` also passes a 101
through immediately — a protocol switch must hit the wire before the
hijack.

**What would reintroduce it.** Any new middleware that wraps
`http.ResponseWriter` in a struct without `Flush`/`Hijack` passthroughs
*and* an `Unwrap`. If it wraps, it must forward — and `Unwrap` is the
future-proof half, because `ResponseController` discovers capabilities
through it.

## Per-deploy docker networks leaked until the daemon's address pools ran dry

**Symptom.** After ~6 hours of normal traffic on a fresh worker, every
containered start failed with `create network local-preview-onyx-<hash>:
POST /networks/create: could not find an available, non-overlapping IPv4
address pool among the defaults to assign to the network`. `docker network
ls` on the worker showed 29 managed networks against 12 running containers.
29 is exactly the daemon's stock budget: 172.17–172.31/16 (minus docker0's
and the VPC's) plus sixteen /20s from 192.168/16.

**Root cause.** `startContainer` get-or-creates a bridge network per backend
hash (`local-preview-<repo>-<beHash[:12]>`) so a process-mode frontend can
reach its backend by alias — and nothing ever removed it. The only removal
sites were `ReclaimOrphans` (startup) and `PurgeRepoContainers` (repo
delete). Every distinct preview served leaked one network for the life of
the node; on a scale-to-zero fleet, "the life of the node" is a working day.

**Fix.** The container reaper releases the deploy network right after it
removes the container (`releaseNetwork`), and the create/join/start failure
paths do the same. Docker refuses to delete a network with an endpoint
attached, so this is last-one-out with no refcount: the backend's exit
leaves the network to a frontend still on it, and the frontend's exit takes
it. A `Manager.netMu` is held from `EnsureNetwork` through `StartContainer`
(endpoints attach at *start*, not create) and around every removal, so a
peer's reaper can't delete a just-resolved network before the new
container is on it. The worker image also carves docker's default address
pools into /24s (`worker-data.sh.tftpl` writes `daemon.json`), so a burst
hits the disk long before the pool. Regression: the container tests now
assert the network is gone after the last side stops and survives while the
peer holds it.

**What would reintroduce it.** A new path that creates a labeled network
and hands the container off without a matching release on exit; or moving
the network lookup outside `netMu` (a peer exiting in the lookup→start
window deletes the network under the container that just resolved it).

## Worker disks filled with hydrated artifacts nothing ever evicted

**Symptom.** Backend starts on a worker failed with `be artifact files
missing (evicted?): hydrate onyx be/<hash>: extract: mkdir
/var/lib/local-preview/tmp/hydrate-…: no space left on device`. The 60 GB
root disk sat at 99% — 47 GB of it hydrated artifacts (34 be + 14 fe for a
dozen live previews), the rest run images.

**Root cause.** Cache eviction only runs when `--cache-max-artifact-bytes`
is set, and only the control node's `extra_server_args` set it. A worker
hydrates every artifact it is ever asked to serve and, with no cap, keeps
all of them: one ~1.5 GB venv per backend hash, forever, on a disk that
also holds the docker data-root. The "evicted?" in the error is a red
herring — nothing had been evicted; there was nowhere left to hydrate *to*.

**Fix.** A `--role worker` node with a durable tier and no explicit cap
defaults `--cache-max-artifact-bytes` to half the filesystem holding its
data dir (`workerCacheCap`, statfs at startup, logged). The sweeper then
keeps the coldest artifacts off disk exactly as on the control node; a
live process's artifacts stay protected. The module's `root_volume_size_gb`
comment now says what the disk has to hold (images + 2× working set).

**What would reintroduce it.** Any new node role, or a deployment that
overrides the cap to `0` on a worker, without an eviction path; or a change
that makes the worker's data dir live on a different filesystem than the
one statfs measures (the derivation assumes the bind mount preserves the
host path).

## A permanent `init_done_at` bricked previews whose external init effect was reaped

**Symptom.** Onyx previews that had served fine for days crash-looped at
startup, every request rendering the "failed to start" page over
`psycopg2.OperationalError: connection to server ... FATAL: database
"preview_<12-hex>" does not exist`. Restarting the process, rebooting the
worker, and re-placing the preview on a fresh node all reproduced it
identically. The only thing that fixed a bricked preview was deleting the
deploy (which GCs the artifact row) and re-deploying the same sha.

**Root cause.** Two correct-in-isolation decisions met. The manifest's `init`
steps create the preview's own Postgres database (`preview_{hash}`, cloned
from a template, then `alembic upgrade head`) on the shared deps host —
an effect that lives in a service this system does not own. And init ran *at
most once per artifact, ever*: `supervise.(*Manager).start` skipped it
whenever `spec.initDone`, which was set permanently by
`MarkBackendInitDone` and, in fleet mode, carried to workers as a
`WireSpec.InitDone` that `OfferWireSpec` made sticky forever per node and
`AdoptRemoteInitDone` wrote back into the control DB off any successful
ensure. Nothing but `DeleteBackendArtifact` ever unset it. Meanwhile a daily
reaper on the deps host drops every `preview_*` database beyond the newest
20, on the premise that "a dropped one just re-clones on its next cold
start." It never re-cloned: the flag said init was done, so the step that
would have recreated the database was skipped on every start forever.

**Fix.** Init-done became revocable. A backend start that *skipped* init
(the flag said done) and then failed to go healthy — the process exited
during startup, or the health wait timed out — calls
`Manager.revokeInitDone`: it records an `init_revoked` process event, clears
`init_done_at` on a node that owns the row (`Store.ClearBackendInitDone`),
and sets an in-memory `initRevoked[Key]`, which outranks *both* the sticky
wire cache and the control node's next offer (that offer mirrors a DB row
that may still say done). `markInitDone` clears the revocation. The fleet
mirror is `workerapi.Client.InitRevoker` → `Manager.RevokeInitDone`, fired
when an ensure that shipped `InitDone=true` comes back as a failure the
worker *itself* reported — a 502 from the ensure route, distinguished from a
transport error by the typed `*workerapi.HTTPError` that `postJSON` now
returns. So the next start re-runs init, which is idempotent, and the
preview repairs itself in one cold start.

The trigger is deliberately narrow. A start whose init actually ran in that
attempt never revokes: it has already paid, and revoking there would re-run
init on every retry of a backend that simply cannot start. The one unbounded
path left is the intended one — init re-runs until a start succeeds, at the
cost of one idempotent init per failed cold start.

**What would reintroduce it.** Recording an init-done fact anywhere that
only a GC can clear (a new per-node cache, another sticky wire field) without
a revocation path; making `OfferWireSpec` or `AdoptRemoteInitDone`
authoritative over a local revocation; or widening `InitRevoker` to fire on
transport errors — a network blip would then re-run init for every preview on
an unreachable worker. Documenting init as "runs exactly once" again would
also invite non-idempotent steps, which this design cannot support.
