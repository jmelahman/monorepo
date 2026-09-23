package rules

import (
	"fmt"
	"strings"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"mvdan.cc/sh/v3/syntax"
)

// PB5xx: install scriptlets. These run as root on the user's machine at
// install time, so network access and persistence mechanisms are judged much
// more harshly than in the build itself. The exec/obfuscation rules (PB3xx)
// and filesystem rules (PB4xx) also run over scriptlets; the rules here are
// scriptlet-specific.
var scriptletRules = []Rule{
	{
		ID:       "PB501",
		Name:     "network-in-scriptlet",
		Severity: Critical,
		Doc: "Install scriptlets run as root during pacman transactions. A scriptlet that talks " +
			"to the network executes unreviewable remote input at the worst possible moment — " +
			"this is how the 2018 acroread AUR compromise delivered its payload.",
		Check: checkScriptletNetwork,
	},
	{
		ID:       "PB502",
		Name:     "scriptlet-persistence",
		Severity: Warn, MaxSeverity: Critical, // critical for crontabs and other persistence writes
		Doc: "Files a package ships belong in package(), tracked and removed by pacman. A " +
			"scriptlet that edits crontabs, systemd units, shell profiles or autostart entries, or " +
			"creates login-capable users, is installing untracked persistence — a classic backdoor " +
			"pattern.",
		Check: checkScriptletPersistence,
	},
	{
		ID:       "PB503",
		Name:     "unparseable-scriptlet",
		Severity: Error,
		Doc: "An install scriptlet pkglint cannot parse is analyzed by no rule, yet its code " +
			"still runs as root at install time. A parse failure usually means the file is malformed " +
			"or deliberately obfuscated to defeat static analysis; either way it must be reviewed by hand.",
		Check: checkScriptletParseError,
	},
	{
		ID:       "PB504",
		Name:     "hook-covered-command",
		Severity: Info,
		Doc: "pacman has run update-desktop-database, gtk-update-icon-cache, fc-cache and friends " +
			"automatically via alpm hooks since version 5.0. Calling them from a scriptlet is dead " +
			"code at best and shadows the hook's transaction-wide batching at worst; usually the " +
			"whole scriptlet can be deleted.",
		Check: checkHookCoveredCommands,
	},
}

// hookCoveredCommands are commands pacman's stock alpm hooks (or hooks shipped
// by the owning package) run automatically at the end of every transaction, so
// a scriptlet calling them by hand is redundant. Mirrors namcap's
// externalhooks list.
var hookCoveredCommands = map[string]bool{
	"update-desktop-database": true, "update-mime-database": true, "install-info": true,
	"glib-compile-schemas": true, "gtk-update-icon-cache": true, "xdg-icon-resource": true,
	"gconfpkg": true, "gio-querymodules": true, "fc-cache": true, "mkfontscale": true,
	"mkfontdir": true, "systemd-sysusers": true, "systemd-tmpfiles": true, "vlc-cache-gen": true,
}

func checkHookCoveredCommands(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.Commands() {
		if !c.Unit.Scriptlet || !hookCoveredCommands[c.Name] {
			continue
		}
		out = append(out, c.finding("PB504", Info,
			"%s is run automatically by a pacman hook since pacman 5.0; this scriptlet call is redundant", c.Name))
	}
	return out
}

func checkScriptletParseError(ctx *Context) []Finding {
	var out []Finding
	for _, se := range ctx.Pkg.ScriptletErrors {
		out = append(out, Finding{
			RuleID: "PB503",
			// Error (not Warn) drops the package to grade D: a root-executed
			// file that no rule could analyze is a blind spot, not a nit.
			Severity: Error,
			Path:     se.Path,
			Line:     1,
			Col:      1,
			Message:  fmt.Sprintf("install scriptlet could not be parsed and was not analyzed: %s", se.Err),
		})
	}
	return out
}

func checkScriptletNetwork(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.Commands() {
		if !c.Unit.Scriptlet {
			continue
		}
		fetches, ok := networkCommands[c.Name]
		if !ok || !fetches(c) {
			continue
		}
		out = append(out, c.finding("PB501", Critical,
			"%s accesses the network from an install scriptlet running as root", c.Name))
	}
	return out
}

var persistencePathHints = []string{
	"/etc/cron", "/var/spool/cron", "/etc/systemd/system", "/usr/lib/systemd/system",
	"/etc/profile", "/etc/bash.bashrc", "/etc/zsh", ".bashrc", ".zshrc", ".profile",
	".config/autostart", "/etc/xdg/autostart", "/etc/ld.so.preload", "/etc/rc.local",
	"/etc/sudoers", "authorized_keys",
}

// persistenceWriters are commands whose arguments name a file they create or
// modify. PB502 is about a scriptlet installing persistence pacman will not
// track, so only a command that actually writes can do it.
var persistenceWriters = map[string]bool{
	"cp": true, "mv": true, "install": true, "ln": true, "tee": true,
	"mkdir": true, "touch": true, "dd": true, "rsync": true,
	"chmod": true, "chown": true, "chgrp": true, "setfacl": true,
}

// sedInPlace reports whether this sed edits its input rather than filtering it.
// The suffix form (-i.bak) counts, and so does a cluster like -Ei — but not the
// script attached to -e or -f (`-es/i/x/`), which those options swallow.
func sedInPlace(c Command) bool {
	for _, a := range c.Args {
		if strings.HasPrefix(a, "--") {
			if strings.HasPrefix(a, "--in-place") {
				return true
			}
			continue
		}
		if !strings.HasPrefix(a, "-") {
			continue
		}
		for _, r := range a[1:] {
			if r == 'i' {
				return true
			}
			if r == 'e' || r == 'f' {
				break // the rest of the cluster is this option's own argument
			}
		}
	}
	return false
}

func checkScriptletPersistence(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.Commands() {
		if !c.Unit.Scriptlet {
			continue
		}
		switch c.Name {
		case "crontab":
			out = append(out, c.finding("PB502", Critical, "scriptlet installs a crontab"))
			continue
		case "useradd", "usermod":
			for i, a := range c.Args {
				if a == "-s" && i+1 < len(c.Args) {
					shell := c.Args[i+1]
					if !strings.Contains(shell, "nologin") && !strings.Contains(shell, "false") {
						out = append(out, c.finding("PB502", Error,
							"scriptlet creates a user with login shell %q; system users should get nologin", shell))
					}
				}
			}
			continue
		case "systemctl":
			if c.HasArg("enable") || c.HasArg("--now") {
				out = append(out, c.finding("PB502", Warn,
					"scriptlet enables a systemd unit; prefer documenting this or shipping a preset"))
			}
			continue
		}
		sev, verb := Error, "touches"
		switch {
		case persistenceWriters[c.Name]:
		case c.Name == "sed" && sedInPlace(c):
		case (c.Name == "rm" || c.Name == "rmdir") && (c.Fn == "pre_remove" || c.Fn == "post_remove"):
			// Cleaning up on removal is what a correct scriptlet does
			// (sunloginclient's post_remove deletes its autostart entry).
			continue
		case c.Name == "rm" || c.Name == "rmdir":
			// Deleting outside removal can be sabotage — clearing a hardening
			// file before installing — and PB405 leaves scriptlet deletions to
			// this rule, so name it, though not at Error.
			sev, verb = Warn, "removes"
		default:
			// A path that is only a test operand (`[ -f /etc/cron.d/foo ]`), a
			// message to the user, or something being sourced is not a write.
			// Real modifications go through one of the commands above, or a
			// redirect, which the walk below catches.
			continue
		}
		for _, a := range c.Args {
			// The hints overlap (/etc/zsh and .zshrc both match /etc/zsh/.zshrc),
			// so report the first match only.
			for _, hint := range persistencePathHints {
				if strings.Contains(a, hint) {
					out = append(out, c.finding("PB502", sev,
						"scriptlet %s %q, a persistence location outside pacman's tracking", verb, a))
					break
				}
			}
		}
	}

	// Redirection targets (echo ... >> /var/spool/cron/root) count too.
	for _, u := range ctx.Pkg.Scriptlets {
		syntax.Walk(u.File, func(node syntax.Node) bool {
			r, ok := node.(*syntax.Redirect)
			if !ok || r.Word == nil {
				return true
			}
			target, _ := pkgbuild.RenderWord(r.Word, ctx.vars)
			for _, hint := range persistencePathHints {
				if strings.Contains(target, hint) {
					out = append(out, findingAt("PB502", Critical, u.Path, r.Pos(),
						"scriptlet writes to %q, a persistence location outside pacman's tracking", target))
					break
				}
			}
			return true
		})
	}
	return out
}
