# pkglint

Never execute anything from analyzed input: PKGBUILDs are parsed (mvdan.cc/sh),
package archives are parsed (debug/elf) — no sourcing, no `ldd`, no interpreters.
This holds absolutely for every analysis path — `lint`, `--fix`, `--add-ignores`,
all of `internal/`. The single exception is `pkglint build`
(`internal/cli/build.go`), which runs makepkg because that is the only way to
get the artifact the PB8xx rules inspect: a separately named verb `pkglint
<path>` never reaches, gated on the static findings, and confined to the
`runCmd`/`lookPath` seams. Do not grow a second one.

The gate is the whole justification, so nothing in the analyzed input may weaken
it: it reads the findings with the file's own `# pkglint: ignore=` directives
disregarded, a file argument must be the `PKGBUILD` makepkg will actually build,
and makepkg's `-p`/`-D`/`--config` and positional `BUILDFILE=`/output-directory
overrides are refused. A new way to reach `runCmd` has to answer "could a
PKGBUILD talk its way through this?" first.

A container build copies the package tree in (no bind mounts) and runs makepkg
as the caller's uid; that user gets no sudo, NOPASSWD or passwd entry, ever.
`-- -s` there is stripped from makepkg's arguments and pkglint itself runs
`pacman -Syu --needed --noconfirm -- <deps>` as root first, with the names from
the static parse (`pkgbuild.SyncDeps`, never `--printsrcinfo`/.SRCINFO), each
checked by `validDepName`; a list that needs running the PKGBUILD is an error.
The CARCH is the container's `uname -m`, through `containerArches`; an unknown
machine refuses the sync rather than guessing. An argument `splitSyncdeps`
cannot read (an unknown short cluster) is refused, not passed on.

`pkglint build --fix` is the one place a fix reads two files: a PB8xx finding
comes out of the archive and the remedy is a line of the PKGBUILD. The archive
never supplies text — the owner names written come from the pacman local
database — and the fixers re-check the pairing themselves
(`fixablePackageArchive`), so a mismatched pair yields no edit. Keep both
properties on any fix added to `internal/rules/fixpkg.go`.

(`--fix` does start `git ls-remote` to resolve a VCS ref — a subprocess, not the
input — and probes https URLs; both live in `internal/netprobe`, the only
network access `--fix` performs (`pkglint build` is the exception: makepkg
fetches the sources, and the sync runs pacman). The git URL passes
`allowedGitURL`'s scheme allowlist, which exists to keep
git's `ext::` "run this command" transport out. Keep it that way.)

- The command lives in `internal/cli`; the module root is a one-line `main`
  shim over `cli.Main`, which is what keeps `go install
  github.com/jmelahman/pkglint@latest` resolving. There is deliberately no
  `cmd/` — a second entry point would only duplicate that shim. `version` is
  stamped in `internal/cli`, not in `main`, so `.goreleaser.yml` and
  `hatch_build.py` both name that symbol in `-X`.
- `tools/fptriage` is a maintainer program, not a second pkglint entry point:
  it lints the site's snapshot cache and asks TypeSafe (typesafe-sdk-go, which
  nothing else imports) whether sampled findings are false positives, ranking
  rules by it. It sends PKGBUILD text out and never runs it. Verdicts append to
  `fptriage.jsonl`, which is also its cache:
  `go run ./tools/fptriage -cache .cache -rules PB914 -dry-run`, then drop
  `-dry-run` with `TYPESAFE_API_KEY` set.
- `go test ./internal/cli -run TestGolden -update` regenerates
  `internal/cli/testdata/*/expected.txt`.
- The `coverage` pre-commit hook (`scripts/coverage.sh`, `-coverpkg=./...`)
  enforces a statement-coverage floor, and test.yml runs a fuzz smoke over the
  untrusted-input parsers; a crasher found by fuzzing is
  checked in under `*/testdata/fuzz/` as a regression seed. Ratchet the floor
  up as gaps close; never lower it.
- `PKGLINT_SMOKE=1 go test ./internal/pkgfile/ -run Inventory -v` sweeps the host's pacman cache.
- Rule contracts (examples, severity-range pinning) are test-enforced; the failures say what to add.
- `build-constraints.txt`/`pyproject.toml` are for the PyPI wheel only; Go deps live in `go.mod`.

## Report-card site

No generated output lives on master. `.github/workflows/site.yml` rebuilds the
site nightly — or on demand: `gh workflow run site.yml` — and force-pushes the
`site` branch as a single root commit holding `docs/`, which Pages serves from
that branch's `/docs`, beside the `data/state.jsonl` that produced it and the
`data/feed.jsonl` feed history. Keep it that way: 48 nightly `docs/` snapshots on master are what made a 2MB source
tree a 157MB clone, and a branch that is always one commit deep cannot repeat
it. `docs/` and `data/` are gitignored so a local run cannot re-add them.

The corpus has two sources, and the same gate covers both. The AUR is
indexed from its metadata dump and fetched as cgit snapshots. The official
repositories (`-repos`, default `core,extra,multilib`; `site/official.go`) are
indexed from the pacman sync databases and each PKGBUILD tree is fetched as
its release tag's archive from gitlab.archlinux.org, whose project names and
tags follow devtools' rules (`gitlabProject`, `gitlabTag`) — a wrong name gets
a 200 sign-in page, which is why `fetchTo` refuses HTML. The site's
namespace is flat: a base in both gets one page, graded from the official
PKGBUILD, and the AUR copy is logged as shadowed. State records carry `repo`;
blank means AUR, and a record only counts as prior for a base from the same
repository. An official base's maintainers are not in the sync database: they
come from archlinux.org's package-search JSON (`loadMaintainers`), swept whole
(~64 pages of 250) once a day and cached beside the databases; the packager
stays the database's `%PACKAGER%` name. GitLab is throttled per host
(`throttles`) under its unauthenticated 600-per-ten-minutes limit, archlinux.org
at a page a second; keep any new host's pacing there too.

Local preview, with the published state as a starting point (`-repos ""` for
an AUR-only run):

```shell
curl -sfLo /tmp/state.jsonl https://raw.githubusercontent.com/jmelahman/pkglint/site/data/state.jsonl
go run ./site -maintainer Jamison -since-days 90 -top 500 -budget 200 -state /tmp/state.jsonl -out /tmp/site-preview
```

`state.jsonl` caches findings per base, keyed on LastModified (the build date,
for an official package) plus a rule registry fingerprint, so registry changes
re-lint the corpus automatically over a few nightly runs. Changing rule *logic*
without changing the registry shape needs a `stateEpoch` bump in
`site/state.go`.

`feed.xml` (`site/feed.go`) is an Atom feed of bases whose latest change
introduced a critical finding, an error from `feedErrorRules`, or provenance
drift, or that the scanner refused — for triaging uploads as they land. A base emits only when its
LastModified moved, or when it is first seen within 48h of changing, so
registry/epoch re-lints and the corpus filling in never flood it. Findings are
counted per rule, path and message, not line, and a finding that climbs in
severity is new; against a prior record from another registry only criticals
are fed. What triage clears goes in
`site/feedignore.txt`: exact bases only, never patterns, capped at the
severity judged. Entries carry PKGBUILD text: keep them `type="text"` and
passed through `visible`.

Between nightly runs `.github/workflows/feed.yml` (:15 and :45, same
concurrency group, off the hour so a poll never replaces a pending nightly)
runs `go run ./site -poll-aur`: the AUR's recently-modified RSS, one RPC
`info` call, a scan of the bases that moved, and only `feed.xml` rendered.
It skips official bases by the state's records and by `data/official.txt`,
which the nightly writes from the sync databases so the poll never downloads
them.
`scripts/publish-feed.sh` patches the site commit with git plumbing — blobless
depth-one fetch, three blobs swapped in the index, a new parentless commit,
`--force-with-lease` against the commit the scan read (it stands down if the
branch moved). It fetches the three files' old blobs on purpose: `push --thin`
fails without them, and they turn the 40MB `state.jsonl` into a delta of a few
hundred bytes. Never let it touch `docs/` blobs.
