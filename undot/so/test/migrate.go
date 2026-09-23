package main

import (
	"solod.dev/so/os"
	"solod.dev/so/strings"
	"solod.dev/so/testing"
)

func TestMigrateBasic(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	os.Mkdir(".claude", 0o755)
	os.WriteFile(".claude/settings.json", []byte("{}\n"), 0o644)
	os.WriteFile(".myrc", []byte("x=1\n"), 0o644)
	os.Mkdir(".github", 0o755)
	os.WriteFile(".gitignore", []byte("node_modules/\n"), 0o644)

	if runCLI("migrate") != 0 {
		t.Fatal("migrate failed")
		return
	}

	if !isRealDir(".config/.claude") {
		t.Error(".config/.claude should be a real directory")
	}
	if !isLinkTo(".claude", ".config/.claude") {
		t.Error(".claude should be a symlink to .config/.claude")
	}
	if !isLinkTo(".myrc", ".config/.myrc") {
		t.Error(".myrc should be a symlink to .config/.myrc")
	}
	if !isRealDir(".github") {
		t.Error(".github must stay a real directory")
	}
	if readFile(".config/.claude/settings.json") != "{}\n" {
		t.Error(".claude/settings.json content lost in migration")
	}

	ignore := readFile(".gitignore")
	if !strings.Contains(ignore, "node_modules/\n") {
		t.Error("existing .gitignore content lost")
	}
	if !strings.Contains(ignore, "/.claude\n") {
		t.Error(".gitignore missing /.claude")
	}
	if !strings.Contains(ignore, "/.myrc\n") {
		t.Error(".gitignore missing /.myrc")
	}

	pc := readFile(".pre-commit-config.yaml")
	if !strings.Contains(pc, "id: undot") {
		t.Error(".pre-commit-config.yaml missing undot hook")
	}
	if !strings.Contains(pc, "default_install_hook_types:") {
		t.Error(".pre-commit-config.yaml missing default_install_hook_types")
	}
}

func TestMigrateIdempotent(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	os.Mkdir(".claude", 0o755)
	os.WriteFile(".claude/settings.json", []byte("{}\n"), 0o644)

	if runCLI("migrate") != 0 {
		t.Fatal("first migrate failed")
		return
	}
	if runCLI("migrate") != 0 {
		t.Fatal("second migrate failed")
		return
	}

	ignore := readFile(".gitignore")
	if strings.Count(ignore, "/.claude\n") != 1 {
		t.Error(".gitignore entry duplicated by second migrate")
	}
	pc := readFile(".pre-commit-config.yaml")
	if strings.Count(pc, "id: undot") != 1 {
		t.Error("pre-commit hook duplicated by second migrate")
	}
	if !isLinkTo(".claude", ".config/.claude") {
		t.Error(".claude link broken after second migrate")
	}
}

func TestMigrateExplicit(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	// .env is on the default skip list, but naming it explicitly migrates it.
	os.WriteFile(".env", []byte("SECRET=1\n"), 0o600)
	os.Mkdir(".claude", 0o755)

	if runCLI2("migrate", ".env") != 0 {
		t.Fatal("explicit migrate failed")
		return
	}
	if !isLinkTo(".env", ".config/.env") {
		t.Error(".env should be a symlink after explicit migrate")
	}
	if !isRealDir(".claude") {
		t.Error(".claude should be untouched by explicit migrate of .env")
	}
}

func TestMigrateSkipsDefaults(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	os.WriteFile(".env", []byte("SECRET=1\n"), 0o600)
	os.Mkdir(".mypy_cache", 0o755)
	os.Mkdir(".claude", 0o755)

	if runCLI("migrate") != 0 {
		t.Fatal("migrate failed")
		return
	}
	if isLinkTo(".env", ".config/.env") || exists(".config/.env") {
		t.Error(".env should be skipped by the automatic scan")
	}
	if exists(".config/.mypy_cache") {
		t.Error(".mypy_cache should be skipped by the automatic scan")
	}
	if !isLinkTo(".claude", ".config/.claude") {
		t.Error(".claude should be migrated")
	}
}

func TestMigrateDryRun(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	os.Mkdir(".claude", 0o755)

	if runCLI2("migrate", "--dry-run") != 0 {
		t.Fatal("dry-run migrate failed")
		return
	}
	if !isRealDir(".claude") {
		t.Error("dry run must not move .claude")
	}
	if exists(".config/.claude") {
		t.Error("dry run must not create .config/.claude")
	}
	if exists(".pre-commit-config.yaml") {
		t.Error("dry run must not create .pre-commit-config.yaml")
	}
	if exists(".gitignore") {
		t.Error("dry run must not create .gitignore")
	}
}

func TestGitignoreCompanionPatterns(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	os.Mkdir(".yarn", 0o755)
	os.Mkdir(".claude", 0o755)
	existing := ".yarn/cache\n" +
		"!.yarn/patches\n" +
		"**/.claude/tmp\n" +
		"node_modules/\n"
	os.WriteFile(".gitignore", []byte(existing), 0o644)

	if runCLI("migrate") != 0 {
		t.Fatal("migrate failed")
		return
	}
	ignore := readFile(".gitignore")
	if !strings.Contains(ignore, "/.config/.yarn/cache\n") {
		t.Error("missing companion for .yarn/cache")
	}
	if !strings.Contains(ignore, "!/.config/.yarn/patches\n") {
		t.Error("missing negated companion for !.yarn/patches")
	}
	if strings.Contains(ignore, ".config/**") || strings.Contains(ignore, "/.config/**/.claude") {
		t.Error("**/ patterns match anywhere already; no companion expected")
	}
	if strings.Count(ignore, "node_modules/") != 1 {
		t.Error("unrelated patterns must be left alone")
	}

	// Second run adds nothing new.
	if runCLI("migrate") != 0 {
		t.Fatal("second migrate failed")
		return
	}
	ignore2 := readFile(".gitignore")
	if strings.Count(ignore2, "/.config/.yarn/cache\n") != 1 {
		t.Error("companion pattern duplicated by second migrate")
	}
}

func TestMigrateConflict(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	os.Mkdir(".claude", 0o755)
	os.Mkdir(".config", 0o755)
	os.Mkdir(".config/.claude", 0o755)

	if runCLI("migrate") == 0 {
		t.Error("migrate should fail when both .claude and .config/.claude exist")
	}
	if !isRealDir(".claude") {
		t.Error("conflicting .claude must not be touched")
	}
}
