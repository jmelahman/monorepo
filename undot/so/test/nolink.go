package main

import (
	"solod.dev/so/os"
	"solod.dev/so/strings"
	"solod.dev/so/testing"
)

func TestNolinkSkipsAndCleansUp(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	os.Mkdir(".claude", 0o755)
	os.WriteFile(".myrc", []byte("x=1\n"), 0o644)

	if runCLI("migrate") != 0 {
		t.Fatal("initial migrate failed")
		return
	}
	if !isLinkTo(".myrc", ".config/.myrc") {
		t.Fatal(".myrc should be linked before nolink exists")
		return
	}

	// Marking the migrated entry nolink: link removes the stale link and
	// stops creating it.
	os.WriteFile(".config/undot.conf", []byte("nolink .myrc\n"), 0o644)
	if runCLI("link") != 0 {
		t.Fatal("link failed")
		return
	}
	if exists(".myrc") {
		t.Error("link should remove the previously created .myrc link")
	}
	if !exists(".config/.myrc") {
		t.Error(".config/.myrc must stay")
	}
	if !isLinkTo(".claude", ".config/.claude") {
		t.Error("other links must be unaffected")
	}
	if runCLI("link") != 0 {
		t.Error("link should stay clean with nolink in place")
	}

	// A real file at the root path is never touched.
	os.WriteFile(".myrc", []byte("local override\n"), 0o644)
	if runCLI("link") != 0 {
		t.Error("link should ignore a real file at a nolink path")
	}
	if readFile(".myrc") != "local override\n" {
		t.Error("real file at nolink path must be left alone")
	}
}

func TestNolinkMigrate(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	os.Mkdir(".config", 0o755)
	os.WriteFile(".config/undot.conf", []byte("nolink .goreleaser.yaml\n"), 0o644)
	os.WriteFile(".goreleaser.yaml", []byte("version: 2\n"), 0o644)
	os.Mkdir(".claude", 0o755)

	// Explicitly migrating a nolink entry moves it without linking back.
	if runCLI2("migrate", ".goreleaser.yaml") != 0 {
		t.Fatal("explicit migrate failed")
		return
	}
	if exists(".goreleaser.yaml") {
		t.Error("nolink entry must not get a root symlink")
	}
	if !exists(".config/.goreleaser.yaml") {
		t.Error(".goreleaser.yaml should be moved into .config/")
	}
	ignore := readFile(".gitignore")
	if strings.Contains(ignore, "/.goreleaser.yaml") {
		t.Error("no .gitignore entry expected for a nolink entry")
	}
}

func TestNolinkUserLevelRejected(t *testing.T) {
	if !enterTempRepo() {
		t.Fatal("cannot set up temp repo")
		return
	}
	// enterTempRepo points XDG_CONFIG_HOME at the repo dir; a user-level
	// nolink must not be honored.
	os.Mkdir("undot", 0o755)
	os.WriteFile("undot/undot.conf", []byte("nolink .claude\n"), 0o644)
	os.Mkdir(".claude", 0o755)

	if runCLI("migrate") != 0 {
		t.Fatal("migrate failed")
		return
	}
	if !isLinkTo(".claude", ".config/.claude") {
		t.Error("user-level nolink must be ignored; .claude should be linked")
	}
}
