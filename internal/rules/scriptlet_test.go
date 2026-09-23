package rules

import (
	"strings"
	"testing"
)

// echo/printf arguments are text being printed, not files being touched, so a
// post-install message mentioning ~/.zshrc must not trip the persistence rule.
// An actual write via redirection still must.
func TestScriptletPersistenceMessagesNotFlagged(t *testing.T) {
	t.Run("PB502 not for instructions echoed to the user", func(t *testing.T) {
		expectNoRule(t, "PB502", map[string]string{
			"PKGBUILD": pkgbuildWith("", "install=foo.install"),
			"foo.install": "post_install() {\n" +
				"  echo \"You have to execute 'cp /usr/share/oh-my-zsh/zshrc ~/.zshrc' to use it.\"\n" +
				"}\n" +
				"post_remove() {\n" +
				"  printf '%s\\n' 'Please remove ~/.zshrc to avoid errors.'\n" +
				"}\n",
		})
	})

	t.Run("PB502 still fires when echo redirects into a persistence path", func(t *testing.T) {
		expectRule(t, "PB502", map[string]string{
			"PKGBUILD":    pkgbuildWith("", "install=foo.install"),
			"foo.install": "post_install() {\n  echo 'export PATH=/opt/foo:$PATH' >> ~/.zshrc\n}\n",
		})
	})

	t.Run("PB502 still fires for non-output commands with persistence args", func(t *testing.T) {
		expectRule(t, "PB502", map[string]string{
			"PKGBUILD":    pkgbuildWith("", "install=foo.install"),
			"foo.install": "post_install() {\n  cp /usr/share/foo/zshrc ~/.zshrc\n}\n",
		})
	})

	t.Run("PB502 not for a path that is only a test operand", func(t *testing.T) {
		expectNoRule(t, "PB502", map[string]string{
			"PKGBUILD": pkgbuildWith("", "install=foo.install"),
			"foo.install": "post_install() {\n" +
				"  if [ -f /etc/xdg/autostart/foo.desktop ]; then\n    echo present\n  fi\n" +
				"}\n",
		})
	})

	t.Run("PB502 fires for an in-place sed on a persistence file", func(t *testing.T) {
		expectRule(t, "PB502", map[string]string{
			"PKGBUILD":    pkgbuildWith("", "install=foo.install"),
			"foo.install": "post_install() {\n  sed -i 's/x/y/' /etc/cron.d/foo\n}\n",
		})
	})

	t.Run("PB502 not for a sed that only filters", func(t *testing.T) {
		expectNoRule(t, "PB502", map[string]string{
			"PKGBUILD":    pkgbuildWith("", "install=foo.install"),
			"foo.install": "post_install() {\n  sed -n '1p' /etc/cron.d/foo\n}\n",
		})
	})

	t.Run("PB502 fires for an -Ei cluster", func(t *testing.T) {
		expectRule(t, "PB502", map[string]string{
			"PKGBUILD":    pkgbuildWith("", "install=foo.install"),
			"foo.install": "post_install() {\n  sed -Ei 's/x/y/' /etc/cron.d/foo\n}\n",
		})
	})

	t.Run("PB502 not for an i inside an attached -e script", func(t *testing.T) {
		// -e swallows the rest of the argument; the i is sed script, not a flag.
		expectNoRule(t, "PB502", map[string]string{
			"PKGBUILD":    pkgbuildWith("", "install=foo.install"),
			"foo.install": "post_install() {\n  sed -es/i/x/ /etc/cron.d/foo\n}\n",
		})
	})

	t.Run("PB502 removal outside a remove hook is still named", func(t *testing.T) {
		// PB405 leaves scriptlet deletions to PB502, so an install-time rm of
		// a hardening file must not vanish from both.
		files := map[string]string{
			"PKGBUILD":    pkgbuildWith("", "install=foo.install"),
			"foo.install": "pre_install() {\n  rm -f /etc/sudoers.d/lockdown\n}\n",
		}
		expectRule(t, "PB502", files)
		for _, f := range lint(t, files) {
			if f.RuleID == "PB502" && f.Severity >= Error {
				t.Errorf("removal reported at %v, want below error: %s", f.Severity, f.Message)
			}
		}
	})
	t.Run("PB502 removal in a remove hook is not persistence", func(t *testing.T) {
		// Cleaning up in pre_remove is correct behaviour (sunloginclient
		// removes its autostart entry there).
		for _, script := range []string{
			"pre_remove() {\n  rm -f /etc/cron.d/foo\n}\n",
			"post_remove() {\n  rm -f /etc/xdg/autostart/awesun.desktop\n  rmdir /etc/cron.d/foo\n}\n",
		} {
			expectNoRule(t, "PB502", map[string]string{
				"PKGBUILD":    pkgbuildWith("", "install=foo.install"),
				"foo.install": script,
			})
		}
	})
}

// A scriptlet that fails to parse is walked by no rule, so the parse failure
// itself has to be reported (PB503) rather than silently dropped.
func TestScriptletParseErrorReported(t *testing.T) {
	t.Run("PB503 unparseable scriptlet", func(t *testing.T) {
		expectRule(t, "PB503", map[string]string{
			"PKGBUILD":    pkgbuildWith("", "install=foo.install"),
			"foo.install": "post_install() {\n  echo hi\n# no closing brace\n",
		})
	})

	t.Run("PB503 not for a well-formed scriptlet", func(t *testing.T) {
		expectNoRule(t, "PB503", map[string]string{
			"PKGBUILD":    pkgbuildWith("", "install=foo.install"),
			"foo.install": "post_install() {\n  echo hi\n}\n",
		})
	})

	t.Run("PB503 not when there is no scriptlet at all", func(t *testing.T) {
		expectNoRule(t, "PB503", map[string]string{"PKGBUILD": cleanPKGBUILD})
	})

	t.Run("finding names the scriptlet and the parse error", func(t *testing.T) {
		var got *Finding
		for _, f := range lint(t, map[string]string{
			"PKGBUILD":    pkgbuildWith("", "install=foo.install"),
			"foo.install": "post_install() {\n  echo hi\n# no closing brace\n",
		}) {
			if f.RuleID == "PB503" {
				got = &f
				break
			}
		}
		if got == nil {
			t.Fatal("expected a PB503 finding")
		}
		if !strings.HasSuffix(got.Path, "foo.install") {
			t.Errorf("path = %q, want it to end in foo.install", got.Path)
		}
		if got.Severity != Error {
			t.Errorf("severity = %v, want error", got.Severity)
		}
		if !strings.Contains(got.Message, "could not be parsed") {
			t.Errorf("message = %q, want it to mention the parse failure", got.Message)
		}
	})
}
