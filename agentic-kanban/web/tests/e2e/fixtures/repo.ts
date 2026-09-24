import { spawnSync } from "node:child_process";
import { mkdir, mkdtemp, writeFile } from "node:fs/promises";
import { join } from "node:path";

// Runs git in `cwd` and returns trimmed stdout, throwing on a non-zero exit
// so a broken fixture fails loudly instead of producing an empty repo.
export function git(cwd: string, ...args: string[]): string {
  const r = spawnSync("git", args, { cwd, encoding: "utf8", timeout: 30_000 });
  if (r.status !== 0) {
    throw new Error(`git ${args.join(" ")} in ${cwd} → ${r.status}: ${r.stderr ?? ""}`);
  }
  return (r.stdout ?? "").trim();
}

// Creates a throwaway monorepo under `parent`: one file at the root and one
// subproject at `projectDir`, committed on `main`. The commit matters —
// session worktrees fork from the board's base branch, and a repo with no
// commits has no branch to fork from.
//
// The identity flags are inline rather than in the repo config so the fixture
// works on a machine with no global git identity (CI, a fresh container).
export async function createMonorepo(parent: string, projectDir: string): Promise<string> {
  const repoPath = await mkdtemp(join(parent, "kanban-e2e-monorepo-"));
  git(repoPath, "init", "--initial-branch=main");
  await writeFile(join(repoPath, "go.work"), "go 1.24\n\nuse ./" + projectDir + "\n");
  await mkdir(join(repoPath, projectDir), { recursive: true });
  await writeFile(join(repoPath, projectDir, "main.go"), "package main\n");
  git(repoPath, "add", "-A");
  git(
    repoPath,
    "-c",
    "user.name=kanban e2e",
    "-c",
    "user.email=e2e@example.com",
    "commit",
    "-m",
    "initial commit",
  );
  return repoPath;
}

// Runs a command inside a session container from its own working directory —
// no `-w`, so what comes back reflects the WorkingDir dockerd was given at
// create time, which is exactly what a project_dir board is supposed to move.
export function containerExec(container: string, ...cmd: string[]): string {
  const r = spawnSync("docker", ["exec", container, ...cmd], {
    encoding: "utf8",
    timeout: 30_000,
  });
  if (r.status !== 0) {
    throw new Error(
      `docker exec ${container} ${cmd.join(" ")} → ${r.status}: ${r.stderr ?? ""}`,
    );
  }
  return (r.stdout ?? "").trim();
}
