package cli

import (
	"solod.dev/so/bytes"
	"solod.dev/so/fmt"
	"solod.dev/so/mem"
	"solod.dev/so/os"
	"solod.dev/so/strings"
)

const hookRepoURL = "https://github.com/jmelahman/undot"
const hookRev = "v0.3.0"
const preCommitYAML = ".pre-commit-config.yaml"
const preCommitYML = ".pre-commit-config.yml"

const hookTypesKey = "default_install_hook_types"

// The symlinks must come back after any operation that rewrites the
// worktree, and plain `pre-commit install` only installs the pre-commit
// stage, hence the explicit hook types.
const hookTypesLine = "default_install_hook_types: [pre-commit, post-checkout, post-merge, post-rewrite]"

const freshPreCommitConfig = `default_install_hook_types: [pre-commit, post-checkout, post-merge, post-rewrite]
repos:
  - repo: https://github.com/jmelahman/undot
    rev: v0.3.0
    hooks:
      - id: undot
`

// ensurePreCommitConfig registers the undot hook in the repository's
// pre-commit config, creating the file if needed. Edits are plain text: the
// repo block is inserted before the first existing `- repo:` entry, reusing
// its indentation. Returns 1 if changed, 0 if already configured, -1 on error.
func ensurePreCommitConfig(a mem.Allocator, dryRun bool) int {
	path := preCommitYAML
	data, err := os.ReadFile(a, path)
	if err != nil {
		data2, err2 := os.ReadFile(a, preCommitYML)
		if err2 == nil {
			path = preCommitYML
			data = data2
			err = nil
		}
	}

	if err != nil {
		if dryRun {
			fmt.Printf("undot: [dry-run] would create %s with the undot hook\n", preCommitYAML)
			return 1
		}
		if werr := os.WriteFile(preCommitYAML, []byte(freshPreCommitConfig), 0o644); werr != nil {
			fmt.Fprintf(os.Stderr, "undot: cannot write %s: %s\n", preCommitYAML, werr.Error())
			return -1
		}
		fmt.Printf("undot: created %s\n", preCommitYAML)
		return 1
	}

	content := string(data)
	if strings.Contains(content, hookRepoURL) || strings.Contains(content, "id: undot") {
		return 0
	}
	if dryRun {
		fmt.Printf("undot: [dry-run] would add the undot hook to %s\n", path)
		return 1
	}

	lines := strings.Split(a, content, "\n")

	// Where to insert default_install_hook_types: the top of the document,
	// unless the key is already there.
	insertTypesAt := 0
	if strings.Contains(content, hookTypesKey) {
		insertTypesAt = -1
	} else if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		insertTypesAt = 1
	}

	// Where to insert the repo block: before the first `- repo:` entry,
	// reusing its indentation, or right after a bare `repos:` key.
	insertRepoAt := -1
	reposIdx := -1
	indent := "  "
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if reposIdx == -1 && strings.HasPrefix(t, "repos:") && !strings.HasPrefix(ln, " ") && !strings.HasPrefix(ln, "\t") {
			reposIdx = i
			continue
		}
		if reposIdx != -1 && strings.HasPrefix(t, "- repo:") {
			insertRepoAt = i
			dash := strings.IndexByte(ln, '-')
			if dash > 0 {
				indent = ln[:dash]
			}
			break
		}
	}
	if insertRepoAt == -1 && reposIdx != -1 {
		insertRepoAt = reposIdx + 1
	}

	buf := bytes.NewBuffer(a, []byte{})
	for i, ln := range lines {
		if i == insertTypesAt {
			buf.WriteString(hookTypesLine)
			buf.WriteByte('\n')
		}
		if i == insertRepoAt {
			writeRepoBlock(&buf, indent)
		}
		buf.WriteString(ln)
		if i < len(lines)-1 {
			buf.WriteByte('\n')
		}
	}
	if insertRepoAt == -1 {
		// No repos: key anywhere; append one at the end.
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			buf.WriteByte('\n')
		}
		buf.WriteString("repos:\n")
		writeRepoBlock(&buf, "  ")
	}

	if werr := os.WriteFile(path, buf.Bytes(), 0o644); werr != nil {
		fmt.Fprintf(os.Stderr, "undot: cannot write %s: %s\n", path, werr.Error())
		return -1
	}
	fmt.Printf("undot: added the undot hook to %s\n", path)
	return 1
}

func writeRepoBlock(buf *bytes.Buffer, indent string) {
	buf.WriteString(indent)
	buf.WriteString("- repo: " + hookRepoURL + "\n")
	buf.WriteString(indent)
	buf.WriteString("  rev: " + hookRev + "\n")
	buf.WriteString(indent)
	buf.WriteString("  hooks:\n")
	buf.WriteString(indent)
	buf.WriteString("    - id: undot\n")
}
