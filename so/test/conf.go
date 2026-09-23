package main

import (
	"solod.dev/so/os"
	"solod.dev/so/strings"
	"solod.dev/so/testing"
)

func writeRepoConf(content string) bool {
	if os.Mkdir(".config", 0o755) != nil {
		return false
	}
	return os.WriteFile(".config/undot.conf", []byte(content), 0o644) == nil
}

func TestConfIgnore(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	os.Mkdir(".claude", 0o755)
	os.Mkdir(".cursor", 0o755)
	if !writeRepoConf("# ignore claude\n.claude\n") {
		t.Fatal("cannot write conf")
		return
	}

	if runCLI("migrate") != 0 {
		t.Fatal("migrate failed")
		return
	}
	if !isRealDir(".claude") || exists(".config/.claude") {
		t.Error(".claude should be ignored by the scan")
	}
	if !isLinkTo(".cursor", ".config/.cursor") {
		t.Error(".cursor should still be migrated")
	}

	// Naming an ignored entry explicitly migrates it anyway.
	if runCLI2("migrate", ".claude") != 0 {
		t.Fatal("explicit migrate of ignored entry failed")
		return
	}
	if !isLinkTo(".claude", ".config/.claude") {
		t.Error("explicit migrate should override the ignore rule")
	}
}

func TestConfNegateAndGlob(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	// .env is skipped by default; !env opts it back in. The glob ignores
	// every .cursor* entry.
	os.WriteFile(".env", []byte("SECRET=1\n"), 0o600)
	os.Mkdir(".cursor", 0o755)
	os.WriteFile(".cursorignore", []byte("x\n"), 0o644)
	if !writeRepoConf("!.env\n.cursor*\n") {
		t.Fatal("cannot write conf")
		return
	}

	if runCLI("migrate") != 0 {
		t.Fatal("migrate failed")
		return
	}
	if !isLinkTo(".env", ".config/.env") {
		t.Error("!.env should opt .env back into the scan")
	}
	if exists(".config/.cursor") || exists(".config/.cursorignore") {
		t.Error(".cursor* entries should be ignored")
	}
}

func TestUserConfAndRepoOverride(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	// enterTempRepo sets XDG_CONFIG_HOME to the repo dir, so the user-level
	// file is ./undot/undot.conf.
	os.Mkdir("undot", 0o755)
	os.WriteFile("undot/undot.conf", []byte(".claude\n"), 0o644)
	os.Mkdir(".claude", 0o755)
	os.Mkdir(".cursor", 0o755)

	if runCLI("migrate") != 0 {
		t.Fatal("migrate failed")
		return
	}
	if exists(".config/.claude") {
		t.Error("user-level conf should ignore .claude")
	}
	if !isLinkTo(".cursor", ".config/.cursor") {
		t.Error(".cursor should still migrate")
	}

	// The repository conf loads after the user conf, so it wins.
	os.WriteFile(".config/undot.conf", []byte("!.claude\n"), 0o644)
	if runCLI("migrate") != 0 {
		t.Fatal("second migrate failed")
		return
	}
	if !isLinkTo(".claude", ".config/.claude") {
		t.Error("repo conf !.claude should override the user ignore")
	}
}

func TestConfFileNeverLinked(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	os.Mkdir(".claude", 0o755)
	if !writeRepoConf("# nothing ignored\n") {
		t.Fatal("cannot write conf")
		return
	}

	if runCLI("migrate") != 0 || runCLI("link") != 0 {
		t.Fatal("migrate/link failed")
		return
	}
	if exists("undot.conf") {
		t.Error(".config/undot.conf must not be symlinked into the root")
	}
	// A stray root symlink left by an older undot gets cleaned up.
	os.Symlink(".config/undot.conf", "undot.conf")
	if runCLI("link") != 0 {
		t.Fatal("link failed on stray conf symlink")
		return
	}
	if exists("undot.conf") {
		t.Error("link should remove a stray root conf symlink")
	}
	ignore := readFile(".gitignore")
	if strings.Contains(ignore, "undot.conf") {
		t.Error(".gitignore should not mention undot.conf")
	}
	if runCLI2("migrate", "undot.conf") == 0 {
		t.Error("migrating undot.conf itself should fail")
	}
}

func TestForceGuarded(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	os.Mkdir(".github", 0o755)
	os.WriteFile(".github/f.yml", []byte("x\n"), 0o644)

	if runCLI2("migrate", ".github") == 0 {
		t.Error("migrating .github without --force should fail")
	}
	if !isRealDir(".github") {
		t.Error(".github must be untouched without --force")
	}

	if runCLI3("migrate", "--force", ".github") != 0 {
		t.Error("migrating .github with --force should succeed")
	}
	if !isLinkTo(".github", ".config/.github") {
		t.Error(".github should be a symlink after --force migrate")
	}

	// --force without explicit entries is refused.
	if !enterTempRepo() {
		t.Fatal("cannot set up second temp repo")
		return
	}
	if runCLI2("migrate", "--force") == 0 {
		t.Error("--force without entries should be refused")
	}
}

func TestCIConfigsSkippedByDefault(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	os.WriteFile(".goreleaser.yaml", []byte("version: 2\n"), 0o644)
	os.Mkdir(".claude", 0o755)

	if runCLI("migrate") != 0 {
		t.Fatal("migrate failed")
		return
	}
	if exists(".config/.goreleaser.yaml") {
		t.Error(".goreleaser.yaml is read by CI from hook-less checkouts; scan must skip it")
	}
	if runCLI2("migrate", ".goreleaser.yaml") != 0 {
		t.Error("explicit migrate of .goreleaser.yaml should work")
	}
	if !isLinkTo(".goreleaser.yaml", ".config/.goreleaser.yaml") {
		t.Error(".goreleaser.yaml should be migrated when named explicitly")
	}
}
