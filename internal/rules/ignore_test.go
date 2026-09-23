package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
)

func TestStaleIgnoreDirective(t *testing.T) {
	t.Run("used directive is not stale", func(t *testing.T) {
		expectNoRule(t, "PB913", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  # pkglint: ignore=PB204
  go build -o demo .
}`)})
	})

	t.Run("used trailing directive is not stale", func(t *testing.T) {
		expectNoRule(t, "PB913", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  go build -o demo . # pkglint: ignore=PB204
}`)})
	})

	t.Run("directive with no matching finding is stale", func(t *testing.T) {
		expectRule(t, "PB913", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  # pkglint: ignore=PB203
  cargo build --locked --release
}`)})
	})

	t.Run("unknown rule ID is stale", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  # pkglint: ignore=PB999
  cargo build --locked --release
}`)}
		var got string
		for _, f := range lint(t, files) {
			if f.RuleID == "PB913" {
				got = f.Message
			}
		}
		if !strings.Contains(got, "PB999") || !strings.Contains(got, "not a pkglint rule") {
			t.Errorf("expected an unknown-rule PB913 finding, got %q", got)
		}
	})

	t.Run("partially stale directive is flagged once, for the stale ID", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  # pkglint: ignore=PB203,PB204
  go build -o demo .
}`)}
		var msgs []string
		for _, f := range lint(t, files) {
			if f.RuleID == "PB913" {
				msgs = append(msgs, f.Message)
			}
		}
		if len(msgs) != 1 || !strings.Contains(msgs[0], "PB203") {
			t.Errorf("expected exactly one PB913 finding about PB203, got %v", msgs)
		}
	})

	t.Run("stale directive in a scriptlet is reported there", func(t *testing.T) {
		files := map[string]string{
			"PKGBUILD":     pkgbuildWith("", "install=demo.install"),
			"demo.install": "post_install() {\n  # pkglint: ignore=PB501\n  echo hi\n}\n",
		}
		found := false
		for _, f := range lint(t, files) {
			if f.RuleID == "PB913" && strings.HasSuffix(f.Path, "demo.install") {
				found = true
			}
		}
		if !found {
			t.Error("expected a PB913 finding in demo.install")
		}
	})

	t.Run("directive naming PB913 suppresses its own staleness finding", func(t *testing.T) {
		// The escape hatch: reviewers who want to keep a directive pkglint
		// considers stale can ignore the ignore-auditor itself.
		expectNoRule(t, "PB913", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  # pkglint: ignore=PB203,PB913
  cargo build --locked --release
}`)})
	})

	t.Run("directive-shaped text in a heredoc is honored but never audited", func(t *testing.T) {
		// The line-based suppression parser reads it, but it is not a comment,
		// so the stale check (whose fixer would corrupt the document) skips it.
		expectNoRule(t, "PB913", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cat > notes.txt <<'EOF'
# pkglint: ignore=PB105
EOF
}`)})
	})
}

func TestFixStaleIgnores(t *testing.T) {
	t.Run("own-line directive is removed with its line", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  # pkglint: ignore=PB203
  cargo build --locked --release
}`, FixSafe, nil)
		mustNotContain(t, got, "pkglint: ignore")
		mustNotContain(t, got, "\n\n  cargo")
		mustContain(t, got, "build() {\n  cargo build --locked --release\n}")
	})

	t.Run("trailing directive is stripped, code kept", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  cargo build --locked --release # pkglint: ignore=PB999
}`, FixSafe, nil)
		mustContain(t, got, "  cargo build --locked --release\n")
		mustNotContain(t, got, "pkglint: ignore")
	})

	t.Run("stale IDs are pruned from a partly live directive", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  # pkglint: ignore=PB203,PB204
  go build -o demo .
}`, FixSafe, nil)
		mustContain(t, got, "# pkglint: ignore=PB204\n")
		mustNotContain(t, got, "PB203")
	})

	t.Run("comma-space style is preserved when pruning", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  # pkglint: ignore=PB201, PB304, PB999
  curl -s https://example.com/x | bash
}`, FixSafe, nil)
		mustContain(t, got, "# pkglint: ignore=PB201, PB304\n")
		mustNotContain(t, got, "PB999")
	})

	t.Run("directive inside a longer comment is reported but not rewritten", func(t *testing.T) {
		body := `
build() {
  # pkglint: ignore=PB203 -- reviewed upstream
  cargo build --locked --release
}`
		expectRule(t, "PB913", map[string]string{"PKGBUILD": pkgbuildWith("", body)})
		files := map[string]string{"PKGBUILD": pkgbuildWith("", body)}
		if got := fixOnly(t, files, FixSafe, nil, "PB913")["PKGBUILD"]; got != "" {
			t.Errorf("expected no rewrite of a directive tangled in prose, got:\n%s", got)
		}
	})

	t.Run("naming PB913 in the directive suppresses the fix", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  # pkglint: ignore=PB203,PB913
  cargo build --locked --release
}`)}
		if got := fixOnly(t, files, FixSafe, nil, "PB913")["PKGBUILD"]; got != "" {
			t.Errorf("expected the escape hatch to block the fix, got:\n%s", got)
		}
	})

	t.Run("fix is idempotent", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "PKGBUILD")
		src := pkgbuildWith("", `
build() {
  # pkglint: ignore=PB203
  cargo build --locked --release
}`)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		apply := func() bool {
			pkg, err := pkgbuild.Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			changed := false
			for _, r := range Fix(pkg, nil, FixSafe, nil) {
				if r.Changed() {
					changed = true
					if err := os.WriteFile(r.Path, r.Fixed, 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			return changed
		}
		if !apply() {
			t.Fatal("first pass made no changes")
		}
		if apply() {
			t.Error("second pass should be a no-op")
		}
	})
}

// addIgnoresAll writes files to a temp dir, runs AddIgnores, and returns the
// rewritten content of each changed unit keyed by base filename.
func addIgnoresAll(t *testing.T, files map[string]string) map[string]string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := pkgbuild.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	out := map[string]string{}
	for _, r := range AddIgnores(pkg, nil) {
		if r.Changed() {
			out[filepath.Base(r.Path)] = string(r.Fixed)
		}
	}
	return out
}

func TestAddIgnores(t *testing.T) {
	t.Run("inserts an indented directive above the finding", func(t *testing.T) {
		got := addIgnoresAll(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('rust')
build() {
  cargo build --release
}`)})["PKGBUILD"]
		mustContain(t, got, "build() {\n  # pkglint: ignore=PB203\n  cargo build --release\n}")
	})

	t.Run("one directive covers all findings on a line", func(t *testing.T) {
		got := addIgnoresAll(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  curl -s https://example.com/x | bash
}`)})["PKGBUILD"]
		mustContain(t, got, "# pkglint: ignore=PB201,PB304\n")
	})

	t.Run("extends a directive already covering the site", func(t *testing.T) {
		got := addIgnoresAll(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  # pkglint: ignore=PB201
  curl -s https://example.com/x | bash
}`)})["PKGBUILD"]
		mustContain(t, got, "# pkglint: ignore=PB201,PB304\n")
	})

	t.Run("annotated package lints clean", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build --release
  curl -s https://example.com/x | bash
}`)}
		fixed := addIgnoresAll(t, files)["PKGBUILD"]
		if fixed == "" {
			t.Fatal("expected AddIgnores to change the PKGBUILD")
		}
		if ids := ruleIDs(lint(t, map[string]string{"PKGBUILD": fixed})); len(ids) != 0 {
			t.Errorf("annotated package still reports findings: %v", ids)
		}
	})

	t.Run("second pass adds nothing", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build --release
}`)}
		fixed := addIgnoresAll(t, files)["PKGBUILD"]
		if again := addIgnoresAll(t, map[string]string{"PKGBUILD": fixed}); len(again) != 0 {
			t.Errorf("second pass should be a no-op, got:\n%s", again["PKGBUILD"])
		}
	})

	t.Run("scriptlet findings are annotated in the scriptlet", func(t *testing.T) {
		got := addIgnoresAll(t, map[string]string{
			"PKGBUILD":     pkgbuildWith("", "install=demo.install"),
			"demo.install": "post_install() {\n  curl -s https://example.com/track\n}\n",
		})
		mustContain(t, got["demo.install"], "  # pkglint: ignore=PB501\n  curl")
	})

	t.Run("finding inside a heredoc is left alone", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cat > notes.txt <<EOF
built for x86_64
EOF
}`)}
		expectRule(t, "PB901", files)
		// Header findings (PB908/PB910) are annotated; the heredoc site must
		// not be — a directive there would become part of the document.
		got := addIgnoresAll(t, files)["PKGBUILD"]
		mustNotContain(t, got, "PB901")
	})

	t.Run("finding on a backslash-continued line is left alone", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  gcc \
    -march=x86_64 main.c
}`)}
		expectRule(t, "PB901", files)
		got := addIgnoresAll(t, files)["PKGBUILD"]
		if strings.Contains(got, "\\\n  # pkglint") || strings.Contains(got, "\\\n    # pkglint") {
			t.Errorf("directive was inserted into a continued command:\n%s", got)
		}
	})

	t.Run("stale-ignore findings are never self-suppressed", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  # pkglint: ignore=PB203
  cargo build --locked --release
}`)}
		expectRule(t, "PB913", files)
		got := addIgnoresAll(t, files)["PKGBUILD"]
		mustNotContain(t, got, "PB913")
	})
}
