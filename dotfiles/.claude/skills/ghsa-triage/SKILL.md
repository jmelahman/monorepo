---
name: ghsa-triage
description: Triage and fix application-code vulnerabilities reported as GitHub Security Advisories (GHSAs) in the Onyx repo. Use when the user mentions a GHSA ID, a CVE that affects Onyx source code, a security advisory URL, or asks to investigate or remediate a reported vulnerability. Out of scope (refer the user elsewhere) when the fix is purely a dependency version bump.
---

# GHSA Triage and Fix

End-to-end workflow for taking a GitHub Security Advisory from report to merged fix on the Onyx codebase. Modelled on GitHub Security Lab's Taskflow pipeline (collection → audit → report) and Onyx's own conventions. Pauses for human approval before every irreversible action.

## Accepted inputs

- GHSA ID: `GHSA-xxxx-xxxx-xxxx`
- Full advisory URL: `https://github.com/onyx-dot-app/onyx/security/advisories/GHSA-xxxx-xxxx-xxxx`
- CVE ID: `CVE-YYYY-NNNNN` — resolve it to a GHSA via `gh api /advisories -f cve_id=<CVE-ID>`

If the input is ambiguous, ask the user to confirm before proceeding.

## Scope

**In scope**: application-code vulnerabilities in Onyx's own source (backend Python, frontend TypeScript, worker code, infra-as-code inside this repo).

**Out of scope**: vulnerabilities that are fully fixed by bumping a dependency in `pyproject.toml`, `uv.lock`, `package.json`, or `package-lock.json`. If the advisory boils down to "upgrade package X", stop and tell the user this skill is for app-code fixes; a dependency bump is a one-line change that does not need this workflow.

## Prerequisites

Before Phase 1, verify:

1. `gh auth status` reports an authenticated account with `repo` and `security_events` scopes. If not, stop and ask the user to run `gh auth login --scopes repo,security_events`.
2. Working tree is clean (`git status --porcelain` is empty). If dirty, stop and ask the user to commit or stash.
3. A suitable working branch exists. If on `main` or an unrelated branch, propose a new branch via `git checkout -b`. **Branch names must not leak advisory content** — use a generic, topic-based name like `<gh-username>/harden-<area>-<issue>` (e.g. `jmelahman/harden-chat-session-authz`). Do not embed the GHSA ID, CVE ID, or reporter-supplied terminology in the branch name until the advisory is published.

## Embargo handling

Treat every advisory as **embargoed** unless the `gh api` payload has a non-null `published_at`. For embargoed advisories, **no public artifact** produced by this workflow may contain any of:

- The GHSA ID or CVE ID
- The advisory summary or description text
- Reporter handles, PoC payloads, or screenshot URLs
- Any language that would let a reader reconstruct the vulnerability before disclosure

Public artifacts include: branch names, commit subjects, commit bodies, source code comments, test names, test docstrings, PR titles, PR descriptions, and any file content committed to git. Keep GHSA references in ephemeral places only: chat with the user, the private advisory thread, and (after publication) a follow-up advisory comment.

When the advisory is already published (`published_at` is set), the restrictions above relax — you may reference the GHSA/CVE in commits, comments, and PRs. The skill still prefers generic branch names for consistency.

## Phase 1: Collection

Fetch the advisory payload:

```bash
gh api repos/onyx-dot-app/onyx/security-advisories/<GHSA-ID>
```

Extract and record in chat (concise bullet list, no copy-paste of the full JSON):

- `summary` and `description`
- `severity`, `cvss.score`, `cvss.vector_string`
- `cwe_ids`
- `vulnerabilities[]` — affected packages/versions (expect `onyx` itself for app-code fixes)
- Any reporter-supplied PoC in the description (code blocks, curl invocations, payload examples)
- Private-fork link if the reporter pushed a patch

If the advisory is a draft and the `gh api` call returns 404, tell the user the account lacks access to the draft and stop. Do not attempt to scrape the HTML page.

## Phase 2: Locate affected code

Find the code paths the advisory describes:

- `Grep` / `Glob` for symbol names, file paths, route handlers, or distinctive strings mentioned in the description.
- If scope is uncertain (more than ~3 plausible locations, or the advisory is vague), spawn an `Explore` subagent with the advisory text and ask it to return the minimum file set that contains the vulnerability.
- Read each candidate file and trace the data flow from untrusted input to the sink.

Write a one-paragraph **root-cause summary** to chat: what the attacker controls, what path the tainted data travels, what invariant is violated, and which line is the sink. Name files with `path:line` so the user can jump to them.

## Phase 3: Audit (false-positive check)

Before writing any code, verify the advisory is actually exploitable:

- Is the sink reachable from an unauthenticated or low-privilege attacker? If the code path requires admin, note it and reconsider the CVSS.
- Are there existing mitigations upstream (CSRF middleware, auth dependency, input validation via Pydantic, parameterized queries) that already defuse the attack?
- Does the claimed CVSS vector match the real exploit surface, or is the reporter overstating impact?

If the advisory is a false positive or materially misrated, stop. Produce a short reply explaining the reasoning, with `path:line` references, and suggest the user respond on the advisory thread. Do not patch.

## Phase 4: Design fix

Propose the minimal change that closes the vuln at the root cause, not the symptom. Enumerate affected files before editing.

Constraints the fix must respect (enforced by `/workspace/CLAUDE.md`):

- Raise `OnyxError` from `onyx.error_handling.exceptions`. Never `HTTPException`, never `starlette.status` constants.
- DB access goes through `backend/onyx/db/` or `backend/ee/onyx/db/`. Never run queries from other modules.
- Parameterized queries only — no string-built SQL.
- Validate untrusted input at the boundary with Pydantic.
- No backwards-compat shims, no feature flags, no `_var` renames for removed code. Delete cleanly.
- No new comments unless the WHY is non-obvious.

Post the file list + one-sentence fix description to chat before editing. This is the last checkpoint before touching disk.

## Phase 5: Implement

Apply the patch with `Edit` / `Write`. Keep the diff tight — do not refactor surrounding code, do not rename variables, do not add unrelated cleanup.

Add **exactly one regression test** that fails on `main` and passes after the fix. Pick one type, in this priority order (see `CLAUDE.md` "Testing Strategy"):

1. **Integration test** in `backend/tests/integration/` — preferred. Use the appropriate `Manager` class from `backend/tests/integration/common_utils/` and the standard fixtures (e.g. `admin_user`).
2. **External-dependency unit test** in `backend/tests/external_dependency_unit/` — when the fix needs controlled mocking of one component.
3. **Playwright test** in `web/tests/e2e/` — when the vuln is a frontend/backend flow that only reproduces through the browser.
4. **Unit test** in `backend/tests/unit/` — last resort, only for isolated pure-logic fixes.

The test name and docstring must describe the behavior being verified (e.g. `test_stop_chat_session_rejects_non_owner`), not the advisory.

```python
def test_stop_chat_session_rejects_non_owner(...):
    """Non-owner callers must not be able to stop another user's chat session."""
    ...
```

## Phase 6: Verify

Run, in order:

1. The new regression test alone — confirm it passes.
2. Adjacent existing tests in the same file/module — confirm nothing regressed.
3. `prek run --all-files` — mandatory. Fix any lint/type errors.
4. If UI was touched: start the dev server and manually exercise the fixed flow plus one happy path to check for regressions. If you cannot run the UI, say so explicitly in chat.
5. Invoke the harness `/security-review` skill on the pending diff. Paste its findings into chat.

Do not proceed to Phase 7 until every step above is green or explicitly waived by the user.

## Phase 7: Pause for approval

Show the user:

- The full diff (`git diff`).
- The test output.
- The `/security-review` findings.
- The proposed commit message (Phase 8 template).

Then stop and wait. Do not commit, do not push, do not open a PR without an explicit "approved" / "ship it" / "go ahead" from the user. A previous approval earlier in the conversation does not carry forward.

## Phase 8: Commit and PR

On explicit approval. First re-check `published_at` — if still null, the advisory is embargoed and commit/PR content must stay generic.

### Published advisory

Commit message template:

```
fix(security): <short description> to patch <GHSA-ID>

<one-paragraph summary of root cause and fix>

Advisory: https://github.com/onyx-dot-app/onyx/security/advisories/<GHSA-ID>
<CVE-ID if assigned>
```

PR body may reference the GHSA directly.

### Embargoed advisory

Propose a generic subject describing the defensive change, not the vulnerability (e.g. `fix(security): harden chat session stop endpoint authorization`). Confirm the subject with the user before committing.

Commit body: one paragraph describing the hardening in general terms — what invariant the change now enforces, not what attack it prevents. Do not include the GHSA ID, CVE ID, advisory URL, reporter handle, PoC excerpts, or phrasing copied from the advisory text. The in-repo commit must read like a routine hardening change to anyone browsing `git log`.

PR title and body follow the same rule. Use generic language describing the behavior change. Do not link the advisory; avoid words like "IDOR", "auth bypass", "exploit" that would tip off a reader. Track the GHSA linkage privately — comment on the advisory thread with the PR URL after it is opened, never the reverse.

Push the branch, then open the PR:

```bash
gh pr create --title "<commit subject>" --body "$(cat <<'EOF'
## Description
- <generic bullet describing the behavior change>
- <generic bullet describing the behavior change>

## How Has This Been Tested?
- [x] New regression test passes
- [x] `pre-commit run --all-files` clean
- [x] `/security-review` findings addressed
EOF
)"
```

Return the PR URL to the user. Then invoke the harness `/check-pr` skill to catch anything CI or reviewers might flag.

## Failure modes to watch for

- **Patching the symptom, not the root cause** — if the fix is a string filter rather than a structural change, stop and reconsider in Phase 4.
- **Over-scoping** — one GHSA, one minimal diff, one regression test. Resist the urge to refactor.
- **Silent commits** — never commit without the explicit Phase 7 approval, even if the fix looks trivial.
- **Leaking advisory content** — for embargoed advisories, no GHSA/CVE/reporter/PoC references belong in any committed artifact: branch name, commit message, source comments, test names, PR title, PR body. Treat every committed artifact as public.
