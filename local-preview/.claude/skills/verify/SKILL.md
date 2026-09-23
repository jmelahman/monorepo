---
name: verify
description: Build, launch, and drive local-preview to verify a change end-to-end.
---

# Verifying local-preview changes

## Launch

Backend (`:8080`, auto-reloads on Go file changes) and frontend (`:5173`):

```bash
wgo run . serve --in-memory                     # run_in_background
cd web && npm install && npm run dev -- --host 0.0.0.0   # run_in_background
until curl -sf -m 1 http://localhost:8080/api/health >/dev/null \
   && curl -sf -m 1 http://localhost:5173/ >/dev/null; do sleep 2; done
```

Drive the UI with the Playwright MCP tools (`browser_navigate` to
`http://localhost:5173/`, then `browser_snapshot`), never ad-hoc `npx
playwright`.

## Gotchas

- `--in-memory` only makes the *database* ephemeral. Mirror clones,
  artifacts, and logs still land in `~/.local/share/preview/` and survive
  restarts — registering a repo name whose `repos/<name>.git` mirror exists
  from an earlier session fails with "clone failed: repo already exists".
  Use a fresh repo name per session.
- `wgo` restarts the backend on any Go file save; with `--in-memory` that
  resets all state mid-session. Re-register fixtures after editing Go code.
- Preview subdomains work through curl with a Host header:
  `curl http://localhost:8080/ -H "Host: <sha>-<repo>.preview.localhost:8080"`.

## Deployable fixture repo

`preview.toml` requires a `[backend]` section. Minimal fixture that builds
instantly and whose backend never actually starts (it only launches on the
preview's first request — don't request it):

```bash
mkdir -p /tmp/.../demoapp/web /tmp/.../demoapp/srv && cd /tmp/.../demoapp
cat > preview.toml <<'EOF'
[frontend]
path  = "web"
build = [["sh", "-c", "mkdir -p dist && cp index.html dist/"]]
dist  = "dist"

[backend]
path        = "srv"
build       = [["true"]]
run         = ["./never-started"]
health_path = "/api/health"
EOF
echo '<html>hi</html>' > web/index.html
echo backend-ish > srv/main.txt
git init -qb main && git add -A && git commit -qm v1   # set user.name/email
```

Register it in the UI with its absolute path as the source, deploy ref
`main`. New commits (edit files, commit) are picked up on the next deploy of
`main` — no re-register needed.
