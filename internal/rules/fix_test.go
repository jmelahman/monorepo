package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
)

// fixAll writes files to a temp dir, runs Fix at the given level, and returns
// the fixed content of each changed unit keyed by base filename.
func fixAll(t *testing.T, files map[string]string, level FixLevel, env *FixEnv) map[string]string {
	t.Helper()
	return fixAllIgnoring(t, files, level, env, nil)
}

func fixAllIgnoring(t *testing.T, files map[string]string, level FixLevel, env *FixEnv, ignore map[string]bool) map[string]string {
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
	for _, r := range Fix(pkg, ignore, level, env) {
		if r.Changed() {
			out[filepath.Base(r.Path)] = string(r.Fixed)
		}
	}
	return out
}

// fixOnly runs just the named rules' fixers, by ignoring every other rule.
//
// The fixtures here are minimal PKGBUILDs that trip more than the rule under
// test — a `cargo build` with no makedepends is a PB944 finding as much as a
// PB942 one — so a test asserting that *nothing* was rewritten has to say
// which rule it means. Tests that assert a specific rewrite do not need this;
// they read the text they care about out of whatever else also fired.
func fixOnly(t *testing.T, files map[string]string, level FixLevel, env *FixEnv, ids ...string) map[string]string {
	t.Helper()
	return fixAllIgnoring(t, files, level, env, ignoreAllBut(ids...))
}

func ignoreAllBut(ids ...string) map[string]bool {
	ignore := map[string]bool{}
	for _, r := range registry() {
		if !slices.Contains(ids, r.ID) {
			ignore[r.ID] = true
		}
	}
	return ignore
}

// fixPKGBUILD is a convenience for the common single-file case.
func fixPKGBUILD(t *testing.T, body string, level FixLevel, env *FixEnv) string {
	t.Helper()
	return fixAll(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, level, env)["PKGBUILD"]
}

func fakeResolve(_, _ string) (string, error) {
	return "0123456789abcdef0123456789abcdef01234567", nil
}

// Stand-in digests for one notional source file. Their only requirement is
// that they are well-formed and distinct: the fixer never hashes anything
// itself, it compares what a PKGBUILD declares against what LocalDigest
// reports, so these tests are about that agreement rather than about any real
// file's contents.
const (
	demoMD5    = "d3f8d5d2d1f4cbf4f4f0c1e1e1d5a5d5"
	demoSHA1   = "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"
	demoSHA256 = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
)

// digestTable builds a LocalDigest over a filename -> digests table, so a test
// can say exactly which sources are "already downloaded" and what they hash
// to. Anything absent from the table reports as not fetched, which is the case
// the fixer must decline rather than guess at.
func digestTable(files map[string]Digests) func(string, string) (Digests, error) {
	return func(_, filename string) (Digests, error) {
		d, ok := files[filename]
		if !ok {
			return Digests{}, os.ErrNotExist
		}
		return d, nil
	}
}

// demoDigests is the common single-source case: one fetched tarball.
func demoDigests() *FixEnv {
	return &FixEnv{LocalDigest: digestTable(map[string]Digests{
		"demo-1.0.0.tar.gz": {MD5: demoMD5, SHA1: demoSHA1, SHA256: demoSHA256},
	})}
}

// weakPKGBUILD is a PKGBUILD whose only digest is the weak one given.
func weakPKGBUILD(sums string) string {
	return pkgbuildWith(`pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='https://example.com/demo'
license=('MIT')
source=("https://example.com/demo-$pkgver.tar.gz")
`+sums, "")
}

func TestFixWeakChecksums(t *testing.T) {
	got := fixAll(t, map[string]string{
		"PKGBUILD": weakPKGBUILD("md5sums=('" + demoMD5 + "')"),
	}, FixSafe, demoDigests())["PKGBUILD"]
	mustContain(t, got, "sha256sums=('"+demoSHA256+"')")
	// The weak array stays: makepkg checks every array present, so keeping it
	// costs nothing and the edit is purely additive.
	mustContain(t, got, "md5sums=('"+demoMD5+"')")
}

// The digest the fixer writes is only meaningful because the weak one it
// verified describes the same bytes. When the local file does not match what
// the PKGBUILD declares, the honest move is to write nothing — the tarball was
// re-rolled, or something worse, and either way a maintainer has to look.
func TestFixWeakChecksumsRefusesOnMismatch(t *testing.T) {
	got := fixAll(t, map[string]string{
		"PKGBUILD": weakPKGBUILD("md5sums=('00000000000000000000000000000000')"),
	}, FixSafe, demoDigests())
	if _, ok := got["PKGBUILD"]; ok {
		t.Errorf("a weak digest that does not match the local file must not be fixed, got:\n%s", got["PKGBUILD"])
	}
}

// Nothing on disk means nothing to hash: pkglint does not fetch the source to
// close the finding.
func TestFixWeakChecksumsWithoutLocalSource(t *testing.T) {
	env := &FixEnv{LocalDigest: digestTable(nil)}
	got := fixAll(t, map[string]string{
		"PKGBUILD": weakPKGBUILD("md5sums=('" + demoMD5 + "')"),
	}, FixSafe, env)
	if _, ok := got["PKGBUILD"]; ok {
		t.Errorf("an unfetched source must not be fixed, got:\n%s", got["PKGBUILD"])
	}
}

// Without the capability at all — a caller that supplied no FixEnv — the rule
// behaves as it did before it was fixable.
func TestFixWeakChecksumsNeedsCapability(t *testing.T) {
	got := fixAll(t, map[string]string{
		"PKGBUILD": weakPKGBUILD("md5sums=('" + demoMD5 + "')"),
	}, FixSafe, nil)
	if _, ok := got["PKGBUILD"]; ok {
		t.Errorf("no LocalDigest means no fix, got:\n%s", got["PKGBUILD"])
	}
}

// sha1 is the better witness, so it is what gets verified when both are there.
// Only one strong array is written even though both weak arrays are reported.
func TestFixWeakChecksumsPrefersSHA1(t *testing.T) {
	got := fixAll(t, map[string]string{
		"PKGBUILD": weakPKGBUILD("md5sums=('00000000000000000000000000000000')\nsha1sums=('" + demoSHA1 + "')"),
	}, FixSafe, demoDigests())["PKGBUILD"]
	mustContain(t, got, "sha256sums=('"+demoSHA256+"')")
	if n := strings.Count(got, "sha256sums="); n != 1 {
		t.Errorf("expected exactly one sha256sums array, got %d:\n%s", n, got)
	}
}

// ck is makepkg's CRC and pkglint does not compute it, so there is no witness
// to check the local bytes against and no fix to make.
func TestFixWeakChecksumsSkipsCk(t *testing.T) {
	got := fixAll(t, map[string]string{
		"PKGBUILD": weakPKGBUILD("cksums=('123456789')"),
	}, FixSafe, demoDigests())
	if _, ok := got["PKGBUILD"]; ok {
		t.Errorf("a ck-only package has nothing to verify against, got:\n%s", got["PKGBUILD"])
	}
}

// A SKIP carries across to the strong array rather than becoming a digest:
// what PB101 and PB111 report must not be silently altered by a PB102 fix.
func TestFixWeakChecksumsCarriesSKIP(t *testing.T) {
	body := pkgbuildWith(`pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='https://example.com/demo'
license=('MIT')
source=("https://example.com/demo-$pkgver.tar.gz"
        "https://example.com/demo-$pkgver.tar.gz.sig")
md5sums=('`+demoMD5+`'
         'SKIP')`, "")
	got := fixAll(t, map[string]string{"PKGBUILD": body}, FixSafe, demoDigests())["PKGBUILD"]
	// Values align under the first quote, as updpkgsums writes them.
	mustContain(t, got, "sha256sums=('"+demoSHA256+"'\n            'SKIP')")
}

// An array of nothing but SKIP would satisfy "a strong digest is present" and
// silence PB102 while verifying nothing at all — a regression dressed as a
// fix. Reaching that state takes a real digest written against a VCS source,
// which is what makes the rule fire while leaving the fixer with no file to
// hash.
func TestFixWeakChecksumsRefusesAllSKIP(t *testing.T) {
	body := pkgbuildWith(`pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='https://example.com/demo'
license=('MIT')
source=("git+https://example.com/demo.git#tag=v1.0.0")
md5sums=('`+demoMD5+`')`, "")
	got := fixOnly(t, map[string]string{"PKGBUILD": body}, FixSafe, demoDigests(), "PB102")
	if _, ok := got["PKGBUILD"]; ok {
		t.Errorf("an all-SKIP strong array must not be written, got:\n%s", got["PKGBUILD"])
	}
}

// makepkg pairs sums to sources by index. When the counts already disagree
// (PB110's finding), writing an array here would produce a confidently
// misaligned one rather than a partial fix.
func TestFixWeakChecksumsRefusesOnCountMismatch(t *testing.T) {
	body := pkgbuildWith(`pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='https://example.com/demo'
license=('MIT')
source=("https://example.com/demo-$pkgver.tar.gz"
        "https://example.com/extra-$pkgver.tar.gz")
md5sums=('`+demoMD5+`')`, "")
	got := fixAll(t, map[string]string{"PKGBUILD": body}, FixSafe, demoDigests())
	if _, ok := got["PKGBUILD"]; ok {
		t.Errorf("a sums/source count mismatch must not be fixed, got:\n%s", got["PKGBUILD"])
	}
}

// Arch-specific arrays get their own sibling, named for the same arch.
func TestFixWeakChecksumsPerArch(t *testing.T) {
	body := pkgbuildWith(`pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='https://example.com/demo'
license=('MIT')
source_x86_64=("https://example.com/demo-$pkgver.tar.gz")
md5sums_x86_64=('`+demoMD5+`')`, "")
	got := fixAll(t, map[string]string{"PKGBUILD": body}, FixSafe, demoDigests())["PKGBUILD"]
	mustContain(t, got, "sha256sums_x86_64=('"+demoSHA256+"')")
}

// Applying the fix twice must not append a second array: once sha256sums is
// there, PB102 no longer fires and neither does its fixer.
func TestFixWeakChecksumsIdempotent(t *testing.T) {
	src := weakPKGBUILD("md5sums=('" + demoMD5 + "')")
	once := fixAll(t, map[string]string{"PKGBUILD": src}, FixSafe, demoDigests())["PKGBUILD"]
	twice := fixAll(t, map[string]string{"PKGBUILD": once}, FixSafe, demoDigests())
	if _, ok := twice["PKGBUILD"]; ok {
		t.Errorf("second pass changed the file again:\n%s", twice["PKGBUILD"])
	}
}

func mustContain(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("expected output to contain %q\n--- got ---\n%s", want, got)
	}
}

func mustNotContain(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Errorf("expected output NOT to contain %q\n--- got ---\n%s", want, got)
	}
}

// transportPKGBUILD is a one-source package whose source array and sums array
// the caller supplies, so a test can name exactly the transport under test.
func transportPKGBUILD(source, sums string) string {
	return `pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='https://example.com/demo'
license=('MIT')
source=(` + source + `)
` + sums + "\n"
}

const demoSums = "sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')"

// servedEnv is a FixEnv that answers probes from a fixed list of URLs known to
// be served, and records every URL it was asked about. Both capabilities the
// transport fix can use are wired to the same list: ordinary sources go through
// ProbeHTTPS and git remotes through ResolveRef, and a test should not have to
// care which one a given source reaches for.
func servedEnv(urls ...string) (*FixEnv, *[]string) {
	served := map[string]bool{}
	for _, u := range urls {
		served[u] = true
	}
	asked := &[]string{}
	probe := func(u string) error {
		*asked = append(*asked, u)
		if !served[u] {
			return fmt.Errorf("not served: %s", u)
		}
		return nil
	}
	return &FixEnv{
		ProbeHTTPS: probe,
		ResolveRef: func(u, _ string) (string, error) {
			if err := probe(u); err != nil {
				return "", err
			}
			return "0123456789abcdef0123456789abcdef01234567", nil
		},
	}, asked
}

func TestFixInsecureTransport(t *testing.T) {
	// Whether the host answers on https is a fact about the server, not about
	// the PKGBUILD, so this fix stays behind --unsafe-fix even once verified.
	t.Run("safe level leaves the scheme alone", func(t *testing.T) {
		env, _ := servedEnv("https://example.com/demo-1.0.0.tar.gz")
		got := fixAll(t, map[string]string{
			"PKGBUILD": transportPKGBUILD(`"http://example.com/demo-$pkgver.tar.gz"`, demoSums),
		}, FixSafe, env)
		if _, ok := got["PKGBUILD"]; ok {
			t.Errorf("FixSafe should not apply the unsafe PB104 fix, got:\n%s", got["PKGBUILD"])
		}
	})

	for _, tc := range []struct {
		name, source, probed, want string
	}{
		{
			name:   "http",
			source: `"http://example.com/demo-$pkgver.tar.gz"`,
			probed: "https://example.com/demo-1.0.0.tar.gz",
			want:   `"https://example.com/demo-$pkgver.tar.gz"`,
		},
		{
			name:   "ftp",
			source: `"ftp://ftp.example.com/pub/demo-$pkgver.tar.gz"`,
			probed: "https://ftp.example.com/pub/demo-1.0.0.tar.gz",
			want:   `"https://ftp.example.com/pub/demo-$pkgver.tar.gz"`,
		},
		{
			// A git remote is probed as a git remote: an HTTP status says
			// nothing about whether a repository lives at that path.
			name:   "git+http",
			source: `"git+http://example.com/demo.git#commit=` + gitCommit + `"`,
			probed: "git+https://example.com/demo.git",
			want:   `"git+https://example.com/demo.git#commit=` + gitCommit + `"`,
		},
		{
			// The bare git wire protocol has no encrypted form; git+https
			// clones the same path on every forge that offers git://.
			name:   "bare git",
			source: `"git://example.com/demo.git#commit=` + gitCommit + `"`,
			probed: "git+https://example.com/demo.git",
			want:   `"git+https://example.com/demo.git#commit=` + gitCommit + `"`,
		},
		{
			// A renamed source keeps its filename:: prefix; only the scheme
			// moves, and the prefix is not part of what gets probed.
			name:   "renamed source",
			source: `"demo.tar.gz::http://example.com/d/$pkgver"`,
			probed: "https://example.com/d/1.0.0",
			want:   `"demo.tar.gz::https://example.com/d/$pkgver"`,
		},
		{
			// The query string belongs to the server and often decides what it
			// sends back, so probing without it would ask about a different URL.
			name:   "query string preserved",
			source: `"demo.tar.gz::http://example.com/get?file=demo&v=$pkgver"`,
			probed: "https://example.com/get?file=demo&v=1.0.0",
			want:   `"demo.tar.gz::https://example.com/get?file=demo&v=$pkgver"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, asked := servedEnv(tc.probed)
			got := fixAll(t, map[string]string{
				"PKGBUILD": transportPKGBUILD(tc.source, demoSums),
			}, FixUnsafe, env)["PKGBUILD"]
			mustContain(t, got, "source=("+tc.want+")")
			// The digest describes the artifact, not the channel it came over,
			// so an https fetch of the same file still verifies against it.
			mustContain(t, got, demoSums)
			if len(*asked) != 1 || (*asked)[0] != tc.probed {
				t.Errorf("probed %v, want exactly [%s]", *asked, tc.probed)
			}
			if n := ruleIDs(lint(t, map[string]string{"PKGBUILD": got}))["PB104"]; n != 0 {
				t.Errorf("fixed PKGBUILD still has %d PB104 finding(s):\n%s", n, got)
			}
		})
	}

	// The whole point of probing: a host that does not answer over https keeps
	// its finding rather than having the build broken on its behalf.
	t.Run("an unserved https URL is not rewritten", func(t *testing.T) {
		env, asked := servedEnv() // nothing is served
		got := fixAll(t, map[string]string{
			"PKGBUILD": transportPKGBUILD(`"http://example.com/demo-$pkgver.tar.gz"`, demoSums),
		}, FixUnsafe, env)
		if _, ok := got["PKGBUILD"]; ok {
			t.Errorf("expected no edit when the probe fails, got:\n%s", got["PKGBUILD"])
		}
		if len(*asked) != 1 {
			t.Errorf("probed %v, want exactly one attempt", *asked)
		}
	})

	// Offline there is no way to check the claim, and an unchecked rewrite is
	// the thing this fix exists not to do.
	t.Run("without a probe nothing is rewritten", func(t *testing.T) {
		for name, env := range map[string]*FixEnv{"nil env": nil, "no capabilities": {}} {
			got := fixAll(t, map[string]string{
				"PKGBUILD": transportPKGBUILD(`"http://example.com/demo-$pkgver.tar.gz"`, demoSums),
			}, FixUnsafe, env)
			if _, ok := got["PKGBUILD"]; ok {
				t.Errorf("%s: expected no edit without a probe, got:\n%s", name, got["PKGBUILD"])
			}
		}
	})

	// A git source needs a git probe specifically: with only ProbeHTTPS wired
	// up there is no way to ask whether a repository answers, so no edit.
	t.Run("a git source without ResolveRef is not rewritten", func(t *testing.T) {
		env, _ := servedEnv("git+https://example.com/demo.git")
		env.ResolveRef = nil
		got := fixOnly(t, map[string]string{
			"PKGBUILD": transportPKGBUILD(`"git://example.com/demo.git#commit=`+gitCommit+`"`, "sha256sums=('SKIP')"),
		}, FixUnsafe, env, "PB104")
		if _, ok := got["PKGBUILD"]; ok {
			t.Errorf("expected no edit without ResolveRef, got:\n%s", got["PKGBUILD"])
		}
	})

	// svn:// and rsync:// have no rewrite that is even usually right, so the
	// finding stands rather than being closed with a guess.
	for _, src := range []string{`"svn://example.com/demo/trunk"`, `"rsync://example.com/demo.tar.gz"`} {
		t.Run("unrewritable "+src, func(t *testing.T) {
			env, asked := servedEnv("https://example.com/demo/trunk", "https://example.com/demo.tar.gz")
			got := fixOnly(t, map[string]string{
				"PKGBUILD": transportPKGBUILD(src, "sha256sums=('SKIP')"),
			}, FixUnsafe, env, "PB104")
			if _, ok := got["PKGBUILD"]; ok {
				t.Errorf("expected no edit, got:\n%s", got["PKGBUILD"])
			}
			if len(*asked) != 0 {
				t.Errorf("probed %v for an unrewritable scheme, want none", *asked)
			}
		})
	}

	// The finding is about the expanded URL, but the edit can only address
	// bytes that are written down. With the scheme inside a variable there are
	// none, so PB104 keeps reporting it for a human to rewrite.
	t.Run("a scheme hidden in a variable is left alone", func(t *testing.T) {
		env, _ := servedEnv("https://example.com/demo.tar.gz")
		got := fixAll(t, map[string]string{"PKGBUILD": `pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='https://example.com/demo'
_url=http://example.com/demo.tar.gz
source=("$_url")
` + demoSums + "\n"}, FixUnsafe, env)
		if _, ok := got["PKGBUILD"]; ok {
			t.Errorf("expected no edit for a variable-spelled scheme, got:\n%s", got["PKGBUILD"])
		}
	})

	// A URL the parser could not finish expanding is not an address anything
	// can be asked about, so the fix declines before it probes.
	t.Run("an unresolved variable is never probed", func(t *testing.T) {
		env, asked := servedEnv()
		got := fixAll(t, map[string]string{
			"PKGBUILD": transportPKGBUILD(`"http://example.com/$_mirror/demo-$pkgver.tar.gz"`, demoSums),
		}, FixUnsafe, env)
		if _, ok := got["PKGBUILD"]; ok {
			t.Errorf("expected no edit for an unexpanded URL, got:\n%s", got["PKGBUILD"])
		}
		if len(*asked) != 0 {
			t.Errorf("probed %v for an unexpanded URL, want none", *asked)
		}
	})

	// Two occurrences of the scheme in one element leave no way to tell which
	// one is the transport, so neither is touched.
	t.Run("a URL carrying another URL is left alone", func(t *testing.T) {
		env, _ := servedEnv("https://mirror.example.com/get?u=http://example.com/demo.tar.gz")
		got := fixAll(t, map[string]string{
			"PKGBUILD": transportPKGBUILD(`"http://mirror.example.com/get?u=http://example.com/demo.tar.gz"`, demoSums),
		}, FixUnsafe, env)
		if _, ok := got["PKGBUILD"]; ok {
			t.Errorf("expected no edit for an ambiguous scheme, got:\n%s", got["PKGBUILD"])
		}
	})

	// A brace group is two sources written as one element sharing one scheme:
	// one edit, not two overlapping ones. PB104 owns the tarball and PB112 the
	// signature, and both point at the same bytes.
	t.Run("a brace group gets a single edit", func(t *testing.T) {
		env, _ := servedEnv("https://example.com/demo-1.0.0.tar.gz", "https://example.com/demo-1.0.0.tar.gz.sig")
		got := fixAll(t, map[string]string{
			"PKGBUILD": transportPKGBUILD(`"http://example.com/demo-$pkgver.tar.gz"{,.sig}`,
				demoSums+"\nsha256sums+=('SKIP')\nvalidpgpkeys=('ABAF11C65A2970B130ABE3C479BE3E4300411886')"),
		}, FixUnsafe, env)["PKGBUILD"]
		mustContain(t, got, `source=("https://example.com/demo-$pkgver.tar.gz"{,.sig})`)
		mustNotContain(t, got, "http://")
	})

	// The element is the unit of rewriting, so every URL it expands to must
	// verify before any of it is touched — the signature included, even though
	// PB104 does not report it: the one edit re-addresses both fetches.
	t.Run("a brace group with an unserved signature is left alone", func(t *testing.T) {
		env, _ := servedEnv("https://example.com/demo-1.0.0.tar.gz") // the .sig is not
		got := fixAll(t, map[string]string{
			"PKGBUILD": transportPKGBUILD(`"http://example.com/demo-$pkgver.tar.gz"{,.sig}`,
				demoSums+"\nsha256sums+=('SKIP')\nvalidpgpkeys=('ABAF11C65A2970B130ABE3C479BE3E4300411886')"),
		}, FixUnsafe, env)
		if _, ok := got["PKGBUILD"]; ok {
			t.Errorf("expected no edit while one of the element's URLs is unserved, got:\n%s", got["PKGBUILD"])
		}
	})

	// Same rule within one tier: an alternate that fails its probe vetoes the
	// element even when another alternate passed first.
	t.Run("a brace group with an unserved alternate is left alone", func(t *testing.T) {
		env, asked := servedEnv("https://example.com/demo-1.0.0.tar.gz") // the .patch is not
		got := fixAll(t, map[string]string{
			"PKGBUILD": transportPKGBUILD(`"http://example.com/demo-$pkgver"{.tar.gz,.patch}`,
				"sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef'\n            'deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')"),
		}, FixUnsafe, env)
		if _, ok := got["PKGBUILD"]; ok {
			t.Errorf("expected no edit while one alternate is unserved, got:\n%s", got["PKGBUILD"])
		}
		want := []string{"https://example.com/demo-1.0.0.tar.gz", "https://example.com/demo-1.0.0.patch"}
		if len(*asked) != 2 || (*asked)[0] != want[0] || (*asked)[1] != want[1] {
			t.Errorf("probed %v, want %v", *asked, want)
		}
	})

	// hg+http has an obvious https spelling, but neither FixEnv capability can
	// vouch for it — ProbeHTTPS speaks plain https, ResolveRef speaks git — so
	// the fix declines without asking anything.
	t.Run("hg+http is not rewritten", func(t *testing.T) {
		env, asked := servedEnv("hg+https://example.com/demo", "https://example.com/demo")
		got := fixOnly(t, map[string]string{
			"PKGBUILD": transportPKGBUILD(`"hg+http://example.com/demo"`, "sha256sums=('SKIP')"),
		}, FixUnsafe, env, "PB104")
		if _, ok := got["PKGBUILD"]; ok {
			t.Errorf("expected no edit for hg+http, got:\n%s", got["PKGBUILD"])
		}
		if len(*asked) != 0 {
			t.Errorf("probed %v for an unverifiable scheme, want none", *asked)
		}
	})
}

// PB112 fixes the signature sources PB104 deliberately skips, so between them
// every insecurely fetched source is covered exactly once.
func TestFixInsecureSignatureTransport(t *testing.T) {
	env, _ := servedEnv("https://example.com/demo-1.0.0.tar.gz.sig")
	got := fixAll(t, map[string]string{"PKGBUILD": transportPKGBUILD(
		`"https://example.com/demo-$pkgver.tar.gz"
        "http://example.com/demo-$pkgver.tar.gz.sig"`,
		demoSums+"\nsha256sums+=('SKIP')\nvalidpgpkeys=('ABAF11C65A2970B130ABE3C479BE3E4300411886')"),
	}, FixUnsafe, env)["PKGBUILD"]
	mustContain(t, got, `"https://example.com/demo-$pkgver.tar.gz.sig"`)
	if n := ruleIDs(lint(t, map[string]string{"PKGBUILD": got}))["PB112"]; n != 0 {
		t.Errorf("fixed PKGBUILD still has %d PB112 finding(s):\n%s", n, got)
	}
}

const gitCommit = "3f2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a"

func TestFixCargoLocked(t *testing.T) {
	t.Run("appended", func(t *testing.T) {
		body := `
build() {
  cargo build --release
}`
		// --locked fails outright when the source ships no Cargo.lock, so the
		// rewrite is behavior-changing and must not run at the safe level.
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixSafe, nil, "PB203"); len(got) != 0 {
			t.Errorf("FixSafe should not apply the unsafe PB203 fix, got:\n%s", got["PKGBUILD"])
		}
		got := fixPKGBUILD(t, body, FixUnsafe, nil)
		mustContain(t, got, "cargo build --release --locked")
	})
	t.Run("written in front of the separator", func(t *testing.T) {
		// The end of the line belongs to rustc, which has no --locked and
		// stops the build when handed one.
		got := fixPKGBUILD(t, `
build() {
  cargo rustc --release -- -C target-cpu=native
}`, FixUnsafe, nil)
		mustContain(t, got, "cargo rustc --release --locked -- -C target-cpu=native")
		if n := ruleIDs(lint(t, map[string]string{"PKGBUILD": got}))["PB203"]; n != 0 {
			t.Errorf("fixed PKGBUILD still has %d PB203 finding(s):\n%s", n, got)
		}
	})
	t.Run("a separator behind an expansion declines the fix", func(t *testing.T) {
		// The separator is real but no argument spells it, so there is no
		// place in the text the flag is known to land in front of.
		body := `
_args='-- -C target-cpu=native'
build() {
  cargo rustc --release $_args
}`
		files := map[string]string{"PKGBUILD": pkgbuildWith("", body)}
		if n := ruleIDs(lint(t, files))["PB203"]; n != 1 {
			t.Errorf("PB203 fired %d times on an unlocked build, want exactly 1", n)
		}
		if got := fixOnly(t, files, FixUnsafe, nil, "PB203"); len(got) != 0 {
			t.Errorf("nothing may be written past a separator pkglint cannot point at, got:\n%s", got["PKGBUILD"])
		}
	})
}

func TestFixCargoCheckRelease(t *testing.T) {
	t.Run("removed mid-line", func(t *testing.T) {
		body := `
check() {
  cargo test --release --locked
}`
		// Rebuilding the crate in the dev profile changes what the check run
		// does, so the safe level leaves it alone.
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixSafe, nil, "PB940"); len(got) != 0 {
			t.Errorf("FixSafe should not apply the unsafe PB940 fix, got:\n%s", got["PKGBUILD"])
		}
		got := fixPKGBUILD(t, body, FixUnsafe, nil)
		mustContain(t, got, "cargo test --locked")
		mustNotContain(t, got, "--release")
		if n := ruleIDs(lint(t, map[string]string{"PKGBUILD": got}))["PB940"]; n != 0 {
			t.Errorf("fixed PKGBUILD still has %d PB940 finding(s):\n%s", n, got)
		}
	})
	t.Run("cargo check too", func(t *testing.T) {
		got := fixPKGBUILD(t, `
check() {
  cargo check --release
}`, FixUnsafe, nil)
		mustContain(t, got, "cargo check\n")
	})
	t.Run("continuation line taken whole", func(t *testing.T) {
		got := fixPKGBUILD(t, `
check() {
  cargo test \
    --release \
    --locked
}`, FixUnsafe, nil)
		mustContain(t, got, "cargo test \\\n    --locked\n")
	})
	t.Run("last continuation line takes the backslash above it", func(t *testing.T) {
		got := fixPKGBUILD(t, `
check() {
  cargo test --locked \
    --release
  echo done
}`, FixUnsafe, nil)
		mustContain(t, got, "cargo test --locked\n  echo done\n")
	})
	t.Run("harness argument left alone", func(t *testing.T) {
		// `--release` here is the test binary's own flag, not cargo's; PB940
		// does not fire and nothing may be deleted.
		body := `
check() {
  cargo test --locked -- --release
}`
		if n := ruleIDs(lint(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}))["PB940"]; n != 0 {
			t.Errorf("PB940 fired on a harness argument (%d finding(s))", n)
		}
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixUnsafe, nil, "PB940"); len(got) != 0 {
			t.Errorf("nothing should change, got:\n%s", got["PKGBUILD"])
		}
	})
	t.Run("expansion declined", func(t *testing.T) {
		body := `
_flags='--release'
check() {
  cargo test $_flags --locked
}`
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixUnsafe, nil, "PB940"); len(got) != 0 {
			t.Errorf("a --release reached through a variable must not be rewritten, got:\n%s", got["PKGBUILD"])
		}
	})
	t.Run("short spelling removed", func(t *testing.T) {
		got := fixPKGBUILD(t, `
check() {
  cargo test -r --locked
}`, FixUnsafe, nil)
		mustContain(t, got, "cargo test --locked")
	})
}

func TestFixCargoBuildRelease(t *testing.T) {
	t.Run("inserted after the subcommand", func(t *testing.T) {
		body := `
build() {
  cargo build --locked
}`
		// The release profile is a different compile, so the safe level leaves
		// it alone.
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixSafe, nil, "PB942"); len(got) != 0 {
			t.Errorf("FixSafe should not apply the unsafe PB942 fix, got:\n%s", got["PKGBUILD"])
		}
		got := fixPKGBUILD(t, body, FixUnsafe, nil)
		mustContain(t, got, "cargo build --release --locked")
		if n := ruleIDs(lint(t, map[string]string{"PKGBUILD": got}))["PB942"]; n != 0 {
			t.Errorf("fixed PKGBUILD still has %d PB942 finding(s):\n%s", n, got)
		}
	})
	t.Run("the flag lands ahead of a continuation", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  cargo build --locked \
    --features extras
}`, FixUnsafe, nil)
		mustContain(t, got, "cargo build --release --locked \\\n    --features extras\n")
	})
	t.Run("short spelling is already the release profile", func(t *testing.T) {
		body := `
build() {
  cargo build -r --locked
}`
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixUnsafe, nil, "PB942"); len(got) != 0 {
			t.Errorf("`cargo build -r` already builds --release, got:\n%s", got["PKGBUILD"])
		}
	})
	t.Run("a release flag in the array is read, not rewritten", func(t *testing.T) {
		// The array holds the --release this would otherwise insert, so the
		// rule has nothing to report and the fix has nothing to add.
		body := `
build() {
  _myflags=(--release --features extras)
  cargo build --locked "${_myflags[@]}"
}`
		files := map[string]string{"PKGBUILD": pkgbuildWith("", body)}
		if got := fixOnly(t, files, FixUnsafe, nil, "PB942"); len(got) != 0 {
			t.Errorf("a build that already asks for --release must not be edited, got:\n%s", got["PKGBUILD"])
		}
		if n := ruleIDs(lint(t, files))["PB942"]; n != 0 {
			t.Errorf("a build whose array carries --release got %d PB942 finding(s)", n)
		}
	})
	t.Run("an array without it keeps the finding but declines the fix", func(t *testing.T) {
		// Reading the array is what tells the rule the profile is cargo's
		// default; the fix still refuses to edit an argument list whose words
		// it cannot all see in the command itself.
		body := `
build() {
  _myflags=(--features extras)
  cargo build --locked "${_myflags[@]}"
}`
		files := map[string]string{"PKGBUILD": pkgbuildWith("", body)}
		if n := ruleIDs(lint(t, files))["PB942"]; n != 1 {
			t.Errorf("PB942 fired %d times on a dev-profile build, want exactly 1", n)
		}
		if got := fixOnly(t, files, FixUnsafe, nil, "PB942"); len(got) != 0 {
			t.Errorf("an argument list with an expansion in it must not be edited, got:\n%s", got["PKGBUILD"])
		}
	})
	t.Run("an unreadable value keeps the finding but declines the fix", func(t *testing.T) {
		// A feature array is not a profile flag, so the finding stands; the
		// fix still will not edit an argument list it cannot read in full.
		body := `
_features=(cli gui)
build() {
  cargo build --locked --features "${_features[@]}"
}`
		files := map[string]string{"PKGBUILD": pkgbuildWith("", body)}
		if n := ruleIDs(lint(t, files))["PB942"]; n != 1 {
			t.Errorf("PB942 fired %d times on a dev-profile build, want exactly 1", n)
		}
		if got := fixOnly(t, files, FixUnsafe, nil, "PB942"); len(got) != 0 {
			t.Errorf("an argument list with an expansion in it must not be edited, got:\n%s", got["PKGBUILD"])
		}
	})
	t.Run("a dev-profile artifact path declines the fix", func(t *testing.T) {
		// --release would move the binary to target/release and leave this
		// package() copying a path that no longer exists.
		body := `
build() {
  cargo build --locked
}
package() {
  install -Dm755 target/debug/demo "$pkgdir/usr/bin/demo"
}`
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixUnsafe, nil, "PB942"); len(got) != 0 {
			t.Errorf("a package reading target/debug must keep its finding, got:\n%s", got["PKGBUILD"])
		}
	})
	t.Run("a subcommand behind an expansion declined", func(t *testing.T) {
		// The rule resolves $_sub to `build` and fires; the fix has no literal
		// word to insert after, so it leaves the finding standing.
		body := `
_sub=build
build() {
  cargo $_sub --locked
}`
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixUnsafe, nil, "PB942"); len(got) != 0 {
			t.Errorf("nothing to insert after, got:\n%s", got["PKGBUILD"])
		}
	})
	t.Run("a deliberate pair of profiles is left alone", func(t *testing.T) {
		body := `
build() {
  cargo build --locked
  cargo build --locked --release
}`
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixUnsafe, nil, "PB942"); len(got) != 0 {
			t.Errorf("a build() that already builds --release must keep its dev build, got:\n%s", got["PKGBUILD"])
		}
	})
	t.Run("only the build() build ships the artifact", func(t *testing.T) {
		body := `
prepare() {
  cargo build --locked
}`
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixUnsafe, nil, "PB942"); len(got) != 0 {
			t.Errorf("a cargo build outside build() must not be rewritten, got:\n%s", got["PKGBUILD"])
		}
	})
	t.Run("an explicit profile is left alone", func(t *testing.T) {
		body := `
build() {
  cargo build --profile dist --locked
}`
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixUnsafe, nil, "PB942"); len(got) != 0 {
			t.Errorf("an explicit --profile must not be overridden, got:\n%s", got["PKGBUILD"])
		}
	})
	t.Run("alongside the PB203 lockfile fix", func(t *testing.T) {
		// Both fixes address the same bare `cargo build`, and both insert at
		// the end of it; the result has to be one command carrying both flags.
		got := fixPKGBUILD(t, `
build() {
  cargo build
}`, FixUnsafe, nil)
		mustContain(t, got, "cargo build --locked --release\n")
		if n := ruleIDs(lint(t, map[string]string{"PKGBUILD": got})); n["PB942"] != 0 || n["PB203"] != 0 {
			t.Errorf("fixed PKGBUILD still has PB942=%d PB203=%d:\n%s", n["PB942"], n["PB203"], got)
		}
	})
}

func TestFixGoEnvWeakening(t *testing.T) {
	t.Run("standalone assignment removed", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": `pkgname=demo
pkgver=1
pkgrel=1
arch=('x86_64')
GOSUMDB=off
source=("https://example.com/demo.tar.gz")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')
`}, FixSafe, nil)["PKGBUILD"]
		mustNotContain(t, got, "GOSUMDB")
	})
	t.Run("inline command prefix removed, command kept", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  GOFLAGS=-insecure go build -o demo .
}`, FixSafe, nil)
		mustNotContain(t, got, "-insecure")
		mustContain(t, got, "go build -modcacherw -o demo .") // prefix gone, command kept (PB916 adds its flag)
	})
}

func TestFixVCSPins(t *testing.T) {
	body := `pkgname=demo
pkgver=1
pkgrel=1
arch=('x86_64')
url='https://example.com'
source=("git+https://example.com/demo.git#tag=v1")
sha256sums=('SKIP')
`
	t.Run("resolves tag to commit", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": body}, FixSafe, &FixEnv{ResolveRef: fakeResolve})["PKGBUILD"]
		mustContain(t, got, "#commit=0123456789abcdef0123456789abcdef01234567")
		mustNotContain(t, got, "#tag=v1")
	})
	t.Run("offline leaves the ref alone", func(t *testing.T) {
		got := fixOnly(t, map[string]string{"PKGBUILD": body}, FixSafe, nil, "PB103")
		if _, ok := got["PKGBUILD"]; ok {
			t.Errorf("expected no edit without ResolveRef, got:\n%s", got["PKGBUILD"])
		}
	})
	t.Run("a -git package's branch is not pinned", func(t *testing.T) {
		// Pinning here would freeze a package whose contract is "build the tip",
		// so the fix has to stay out of it exactly as checkVCSPins does.
		vcs := `pkgname=demo-git
pkgver=1
pkgrel=1
arch=('x86_64')
url='https://example.com'
source=("git+https://example.com/demo.git#branch=main")
sha256sums=('SKIP')
`
		got := fixOnly(t, map[string]string{"PKGBUILD": vcs}, FixSafe, &FixEnv{ResolveRef: fakeResolve}, "PB103")
		if _, ok := got["PKGBUILD"]; ok {
			t.Errorf("a -git package's branch should not be pinned, got:\n%s", got["PKGBUILD"])
		}
	})
	t.Run("a -git package's mutable tag is still rewritten", func(t *testing.T) {
		vcs := strings.Replace(body, "pkgname=demo", "pkgname=demo-git", 1)
		got := fixAll(t, map[string]string{"PKGBUILD": vcs}, FixSafe, &FixEnv{ResolveRef: fakeResolve})["PKGBUILD"]
		mustContain(t, got, "#commit=0123456789abcdef0123456789abcdef01234567")
	})
	t.Run("a ref spelled through a variable is still pinned", func(t *testing.T) {
		// The template shape: #tag=v$pkgver. The expanded ref resolves like
		// any other; the whole written fragment value gives way to the commit.
		vcs := `pkgname=demo
pkgver=1.15.0
pkgrel=1
arch=('x86_64')
url='https://example.com'
source=("git+https://example.com/demo.git#tag=v$pkgver")
sha256sums=('SKIP')
`
		got := fixAll(t, map[string]string{"PKGBUILD": vcs}, FixSafe, &FixEnv{ResolveRef: fakeResolve})["PKGBUILD"]
		mustContain(t, got, "git+https://example.com/demo.git#commit=0123456789abcdef0123456789abcdef01234567")
		mustNotContain(t, got, "#tag=")
	})
	t.Run("a fragment hidden inside a variable is left alone", func(t *testing.T) {
		vcs := `pkgname=demo
pkgver=1
pkgrel=1
arch=('x86_64')
url='https://example.com'
_ref='tag=v1'
source=("git+https://example.com/demo.git#$_ref")
sha256sums=('SKIP')
`
		got := fixOnly(t, map[string]string{"PKGBUILD": vcs}, FixSafe, &FixEnv{ResolveRef: fakeResolve}, "PB103")
		if out, ok := got["PKGBUILD"]; ok {
			t.Errorf("a variable-hidden fragment key should not be rewritten, got:\n%s", out)
		}
	})
	t.Run("a brace group sharing one fragment is not rewritten", func(t *testing.T) {
		// Two repositories, one written #tag: resolving either URL's ref and
		// editing the shared text would pin both to the same commit.
		vcs := `pkgname=demo
pkgver=1
pkgrel=1
arch=('x86_64')
url='https://example.com'
source=("git+https://example.com/{demo,extra}.git#tag=v1")
sha256sums=('SKIP' 'SKIP')
`
		got := fixOnly(t, map[string]string{"PKGBUILD": vcs}, FixSafe, &FixEnv{ResolveRef: fakeResolve}, "PB103")
		if _, ok := got["PKGBUILD"]; ok {
			t.Errorf("a shared fragment should not be rewritten, got:\n%s", got["PKGBUILD"])
		}
	})
	t.Run("signed tag is not rewritten", func(t *testing.T) {
		signed := strings.Replace(body,
			`source=("git+https://example.com/demo.git#tag=v1")`,
			`source=("git+https://example.com/demo.git?signed#tag=v1")`, 1)
		got := fixOnly(t, map[string]string{"PKGBUILD": signed}, FixSafe, &FixEnv{ResolveRef: fakeResolve}, "PB103")
		if _, ok := got["PKGBUILD"]; ok {
			t.Errorf("signed tag should not be rewritten, got:\n%s", got["PKGBUILD"])
		}
	})
}

// TestFixVCSPinsElementIndexing pins that PB103 edits the element the finding
// is actually about.
//
// sourceElems keys the AST words by the element's index *within its own
// assignment*, restarting at 0 for every `source+=(...)`. Sources() reports
// merged indices spanning all assignments of the name. The two index spaces
// disagreed as soon as a PKGBUILD used `+=` or a brace group, and the lookup
// silently returned the wrong word (or none).
const vcsPinHeader = `pkgname=demo
pkgver=1
pkgrel=1
arch=('x86_64')
url='https://example.com'
`

func TestFixVCSPinsElementIndexing(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"

	t.Run("appended array elements are pinned", func(t *testing.T) {
		// The lookup for merged index 0 hit the appended assignment's element
		// 0 (b.git), whose text does not contain "#tag=v1", so no edit was
		// emitted; merged index 1 had no key at all. The fix silently stopped
		// firing for every package using source+=.
		got := fixAll(t, map[string]string{"PKGBUILD": vcsPinHeader +
			`source=("git+https://example.com/a.git#tag=v1")
source+=("git+https://example.com/b.git#tag=v2")
sha256sums=('SKIP')
sha256sums+=('SKIP')
`}, FixSafe, &FixEnv{ResolveRef: fakeResolve})["PKGBUILD"]
		mustContain(t, got, "git+https://example.com/a.git#commit="+sha)
		mustContain(t, got, "git+https://example.com/b.git#commit="+sha)
		mustNotContain(t, got, "#tag=")
	})

	t.Run("identical refs do not cross-edit", func(t *testing.T) {
		// The dangerous variant: both entries carry "#tag=v1", so the
		// containment guard passed and the edit computed for the *base*
		// element was written over the *appended* element's byte range —
		// rewriting a URL the finding was not about while leaving the real
		// one unpinned.
		got := fixAll(t, map[string]string{"PKGBUILD": vcsPinHeader +
			`source=("git+https://example.com/a.git#tag=v1")
source+=("git+https://example.com/b.git#tag=v1")
sha256sums=('SKIP')
sha256sums+=('SKIP')
`}, FixSafe, &FixEnv{ResolveRef: fakeResolve})["PKGBUILD"]
		mustContain(t, got, "git+https://example.com/a.git#commit="+sha)
		mustContain(t, got, "git+https://example.com/b.git#commit="+sha)
		mustNotContain(t, got, "#tag=")
	})

	t.Run("brace expansion does not shift later elements", func(t *testing.T) {
		// The brace group expands to two entries from one written element, so
		// the git source is merged index 2 but written element 1.
		got := fixAll(t, map[string]string{"PKGBUILD": vcsPinHeader +
			`source=("https://example.com/x.tar.gz"{,.sig}
        "git+https://example.com/a.git#tag=v1")
sha256sums=('SKIP' 'SKIP' 'SKIP')
`}, FixSafe, &FixEnv{ResolveRef: fakeResolve})["PKGBUILD"]
		mustContain(t, got, "git+https://example.com/a.git#commit="+sha)
		mustNotContain(t, got, "#tag=")
	})

	t.Run("plain reassignment resets the numbering", func(t *testing.T) {
		// A second `source=` (no +=) replaces the array, exactly as in bash,
		// so only the surviving element is a source at all. The shadowed line
		// must be left alone.
		got := fixAll(t, map[string]string{"PKGBUILD": vcsPinHeader +
			`source=("git+https://example.com/old.git#tag=v0")
source=("git+https://example.com/new.git#tag=v1")
sha256sums=('SKIP')
`}, FixSafe, &FixEnv{ResolveRef: fakeResolve})["PKGBUILD"]
		mustContain(t, got, "git+https://example.com/new.git#commit="+sha)
		mustContain(t, got, "git+https://example.com/old.git#tag=v0")
	})

	t.Run("indexed override neither resets numbering nor shifts elements", func(t *testing.T) {
		// `source[0]=` updates one element in place: the override's own text
		// is what the fix must rewrite, and the untouched element 1 must keep
		// its numbering (no reset, no consumed index).
		got := fixAll(t, map[string]string{"PKGBUILD": vcsPinHeader +
			`source=("git+https://example.com/old.git#tag=v0"
        "git+https://example.com/b.git#tag=v2")
source[0]="git+https://example.com/new.git#tag=v1"
sha256sums=('SKIP' 'SKIP')
`}, FixSafe, &FixEnv{ResolveRef: fakeResolve})["PKGBUILD"]
		mustContain(t, got, "git+https://example.com/new.git#commit="+sha)
		mustContain(t, got, "git+https://example.com/b.git#commit="+sha)
		mustContain(t, got, "git+https://example.com/old.git#tag=v0")
	})
}

func TestFixGoDownloads(t *testing.T) {
	relintClean := func(t *testing.T, got string) {
		t.Helper()
		if n := ruleIDs(lint(t, map[string]string{"PKGBUILD": got}))["PB204"]; n != 0 {
			t.Errorf("fixed PKGBUILD still has %d PB204 finding(s):\n%s", n, got)
		}
	}
	t.Run("writes a prepare() mirroring build's cd", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  cd "$pkgname" || exit
  go build -o demo .
}`, FixUnsafe, nil)
		mustContain(t, got, "prepare() {\n  cd \"$pkgname\" || exit\n  go mod download -modcacherw\n}\n\nbuild() {")
		mustContain(t, got, "go build -buildmode=pie -trimpath -modcacherw -o demo .") // PB204 adds no -mod flag here
		relintClean(t, got)
	})
	t.Run("a build without a cd gets a bare download", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  go build -o demo .
}`, FixUnsafe, nil)
		mustContain(t, got, "prepare() {\n  go mod download -modcacherw\n}\n\nbuild() {")
		relintClean(t, got)
	})
	t.Run("an existing prepare() is extended, not shadowed", func(t *testing.T) {
		got := fixPKGBUILD(t, `
prepare() {
  cd "$pkgname" || exit
  patch -p1 < ../fix.patch
}
build() {
  cd "$pkgname" || exit
  go build -o demo .
}`, FixUnsafe, nil)
		mustContain(t, got, "patch -p1 < ../fix.patch\n  go mod download -modcacherw\n}")
		mustNotContain(t, got, "prepare() {\n  go mod download") // no second prepare
		relintClean(t, got)
	})
	t.Run("a prepare() without a cd borrows build's", func(t *testing.T) {
		got := fixPKGBUILD(t, `
prepare() {
  patch -p1 < ../fix.patch
}
build() {
  cd "$pkgname" || exit
  go build -o demo .
}`, FixUnsafe, nil)
		mustContain(t, got, "patch -p1 < ../fix.patch\n  cd \"$pkgname\" || exit\n  go mod download -modcacherw\n}")
		relintClean(t, got)
	})
	t.Run("a vendored build needs no edit", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  go build -mod=vendor -o demo .
}`, FixUnsafe, nil)
		mustNotContain(t, got, "go mod download") // vendored: PB204 stays out
	})
	t.Run("a cd sharing its line with another command is not copied", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  cd "$pkgname" && make generate
  go build -o demo .
}`, FixUnsafe, nil)
		mustContain(t, got, "prepare() {\n  go mod download -modcacherw\n}")
		mustNotContain(t, got, "make generate\n  go mod download")
		relintClean(t, got)
	})
}

func TestFixNpmCI(t *testing.T) {
	got := fixPKGBUILD(t, `
build() {
  npm install
  yarn install
  pnpm install
  bun install
}`, FixUnsafe, nil)
	mustContain(t, got, "npm ci")
	mustContain(t, got, "yarn install --immutable")
	mustContain(t, got, "pnpm install --frozen-lockfile")
	mustContain(t, got, "bun install --frozen-lockfile")
}

func TestFixLockfileManagers(t *testing.T) {
	got := fixPKGBUILD(t, `
build() {
  composer install
  bundle install
  uv sync
}`, FixUnsafe, nil)
	mustContain(t, got, "composer install --no-scripts")
	mustContain(t, got, "bundle install --frozen")
	mustContain(t, got, "uv sync --frozen")
}

// The lockfile fixers append their flag through appendFlagEdit and decline on
// hiddenFlagWords, the same primitives the cmake/meson fixers use. These pin
// what that shares: nothing is written into a command whose flags pkglint
// cannot read, nothing lands past a `--`, and a word that merely *contains* an
// expansion is still fixed.
func TestFixLockfileManagersShareFlagPlacement(t *testing.T) {
	lockfileIDs := []string{"PB206", "PB207", "PB208", "PB209"}

	t.Run("hidden flags decline", func(t *testing.T) {
		body := `
_bundle_flags='--frozen'
build() {
  bundle install $_bundle_flags
  composer install "$_composer_flags"
  uv sync $_uv
  yarn install $_yarn
  pnpm install $_pnpm
}`
		files := map[string]string{"PKGBUILD": pkgbuildWith("", body)}
		// The checkers are untouched: `bundle install $_bundle_flags` reads
		// its --frozen through the variable, so PB208 never fired here.
		if n := ruleIDs(lint(t, files))["PB208"]; n != 0 {
			t.Errorf("PB208 fired %d times, want 0 (the checker must not change)", n)
		}
		if got := fixOnly(t, files, FixUnsafe, nil, lockfileIDs...); len(got) != 0 {
			t.Errorf("a flag may not be inserted beside one pkglint cannot read, got:\n%s", got["PKGBUILD"])
		}
	})

	t.Run("flag lands before a literal separator", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  composer install -- extra
}`, FixUnsafe, nil)
		mustContain(t, got, "composer install --no-scripts -- extra")
	})

	t.Run("a separator behind an expansion declines", func(t *testing.T) {
		// Declined by hiddenFlagWords, since `$_rest` is a bare reference to
		// a value pkglint cannot read — which is the outcome the separator
		// check wants too: no text is written past a `--` it cannot point at.
		body := `
build() {
  composer install $_rest
}`
		files := map[string]string{"PKGBUILD": pkgbuildWith("", body)}
		if got := fixOnly(t, files, FixUnsafe, nil, lockfileIDs...); len(got) != 0 {
			t.Errorf("nothing may be written past a separator pkglint cannot point at, got:\n%s", got["PKGBUILD"])
		}
	})

	t.Run("a separator inside a resolved variable declines", func(t *testing.T) {
		// `$_rest` resolves, so hiddenFlagWords passes it; the separator
		// check is what must catch the `--` inside the value. cmake goes
		// through the same primitive, so it is pinned alongside.
		got := fixPKGBUILD(t, `
_rest='-- extra'
build() {
  composer install $_rest
  cmake -B build $_rest
}`, FixUnsafe, nil)
		mustNotContain(t, got, "--no-scripts")
		mustNotContain(t, got, "-DCMAKE_INSTALL_PREFIX")
	})

	t.Run("a resolved variable without a separator is still fixed", func(t *testing.T) {
		got := fixPKGBUILD(t, `
_flags='--prefer-dist'
build() {
  composer install $_flags
}`, FixUnsafe, nil)
		mustContain(t, got, "composer install $_flags --no-scripts")
	})

	t.Run("value expansions are still fixed", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  uv sync --project "$srcdir/app"
}`, FixUnsafe, nil)
		mustContain(t, got, `uv sync --project "$srcdir/app" --frozen`)
	})
}

// The checker and the fixer of PB206–PB209 read one lockfileFlags table. For
// every entry: each reported subcommand fires the finding, each satisfying
// flag silences it, and each fixable subcommand is fixed to a command the
// checker then accepts — so the two halves cannot drift apart.
func TestLockfileFlagsCheckAndFixAgree(t *testing.T) {
	for _, l := range lockfileFlags {
		for _, name := range l.commands {
			for _, sub := range l.subs {
				cmd := strings.TrimSpace(name + " " + sub)
				t.Run(cmd, func(t *testing.T) {
					files := map[string]string{"PKGBUILD": pkgbuildWith("", "build() {\n  "+cmd+"\n}")}
					if n := ruleIDs(lint(t, files))[l.id]; n != 1 {
						t.Fatalf("%s fired %d times for `%s`, want 1", l.id, n, cmd)
					}
					for _, f := range l.satisfied {
						pinned := map[string]string{"PKGBUILD": pkgbuildWith("", "build() {\n  "+cmd+" "+f+"\n}")}
						if n := ruleIDs(lint(t, pinned))[l.id]; n != 0 {
							t.Errorf("%s still fires for `%s %s`", l.id, cmd, f)
						}
					}
					fixSubs := l.fixSubs
					if fixSubs == nil {
						fixSubs = l.subs
					}
					got := fixOnly(t, files, FixUnsafe, nil, l.id)
					if !slices.Contains(fixSubs, sub) {
						if len(got) != 0 {
							t.Errorf("`%s` is not a fixable subcommand, got:\n%s", cmd, got["PKGBUILD"])
						}
						return
					}
					mustContain(t, got["PKGBUILD"], cmd+" "+l.flag)
					if n := ruleIDs(lint(t, map[string]string{"PKGBUILD": got["PKGBUILD"]}))[l.id]; n != 0 {
						t.Errorf("%s still fires after its own fix:\n%s", l.id, got["PKGBUILD"])
					}
				})
			}
		}
	}

	t.Run("a package argument is PB210's and is not frozen", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", "build() {\n  pnpm install left-pad\n}")}
		ids := ruleIDs(lint(t, files))
		if ids["PB206"] != 0 || ids["PB210"] != 1 {
			t.Fatalf("want PB210 once and no PB206, got %v", ids)
		}
		if got := fixOnly(t, files, FixUnsafe, nil, "PB206"); len(got) != 0 {
			t.Errorf("`pnpm install <pkg>` must not be frozen, got:\n%s", got["PKGBUILD"])
		}
	})
}

// `gem install rails` fetches from RubyGems with nothing pinning what it
// gets, and the remedy — a committed Gemfile.lock, or local .gem files to
// install from — is not a flag: PB210 reports it, and the bundler fixer
// leaves it alone.
func TestFixBundlerLeavesGemInstall(t *testing.T) {
	body := `
build() {
  gem install rails
}`
	got := fixPKGBUILD(t, body, FixUnsafe, nil)
	mustNotContain(t, got, "gem install rails --")
	files := map[string]string{"PKGBUILD": pkgbuildWith("", body)}
	ids := ruleIDs(lint(t, files))
	if ids["PB208"] != 0 || ids["PB210"] != 1 {
		t.Errorf("want PB210 once and no PB208 for the gem install, got %v", ids)
	}
}

func TestFixUvRunNotTouched(t *testing.T) {
	got := fixPKGBUILD(t, `
build() {
  uv run make
}`, FixUnsafe, nil)
	mustNotContain(t, got, "uv run make --frozen")
}

func TestFixSetuid(t *testing.T) {
	got := fixPKGBUILD(t, `
package() {
  chmod 4755 "$pkgdir/usr/bin/demo"
}`, FixUnsafe, nil)
	mustContain(t, got, "chmod 0755")
	mustNotContain(t, got, "chmod 4755")
}

func TestFixSetuidInstallMode(t *testing.T) {
	got := fixPKGBUILD(t, `
package() {
  install -Dm4755 demo "$pkgdir/usr/bin/demo"
}`, FixUnsafe, nil)
	mustContain(t, got, "install -Dm0755")
	mustNotContain(t, got, "4755")
}

// Modes whose leading digit is not 4/2/6 still carry the setuid/setgid bit, so
// the fixer must strip it there too.
func TestFixSetuidOctalModes(t *testing.T) {
	t.Run("leading-zero chmod mode", func(t *testing.T) {
		got := fixPKGBUILD(t, `
package() {
  chmod 04755 "$pkgdir/usr/bin/demo"
}`, FixUnsafe, nil)
		mustContain(t, got, `chmod 0755 "$pkgdir/usr/bin/demo"`)
		mustNotContain(t, got, "04755")
	})
	t.Run("setuid plus sticky keeps the sticky bit", func(t *testing.T) {
		got := fixPKGBUILD(t, `
package() {
  chmod 7755 "$pkgdir/usr/bin/demo"
}`, FixUnsafe, nil)
		mustContain(t, got, `chmod 1755 "$pkgdir/usr/bin/demo"`)
	})
	t.Run("leading-zero install mode", func(t *testing.T) {
		got := fixPKGBUILD(t, `
package() {
  install -m 02755 demo "$pkgdir/usr/bin/demo"
}`, FixUnsafe, nil)
		mustContain(t, got, `install -m 0755 demo "$pkgdir/usr/bin/demo"`)
		mustNotContain(t, got, "02755")
	})
	t.Run("ordinary mode is left alone", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  chmod 0755 "$pkgdir/usr/bin/demo"
}`)}, FixUnsafe, nil)
		if _, ok := got["PKGBUILD"]; ok {
			t.Errorf("expected no edit for a non-setuid mode, got:\n%s", got["PKGBUILD"])
		}
	})
}

func TestFixBackupSlash(t *testing.T) {
	got := fixAll(t, map[string]string{"PKGBUILD": `pkgname=demo
pkgver=1
pkgrel=1
arch=('x86_64')
backup=('/etc/foo.conf' 'etc/bar.conf')
`}, FixSafe, nil)["PKGBUILD"]
	mustContain(t, got, "'etc/foo.conf'")
	mustContain(t, got, "'etc/bar.conf'")
	mustNotContain(t, got, "'/etc/foo.conf'")
}

func TestFixVariableType(t *testing.T) {
	t.Run("bare word wrapped in array", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": `pkgname=demo
pkgver=1
pkgrel=1
arch=('x86_64')
depends=gtk3
`}, FixSafe, nil)["PKGBUILD"]
		mustContain(t, got, "depends=(gtk3)")
	})
	t.Run("quoted scalar wrapped preserving the single element", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": `pkgname=demo
pkgver=1
pkgrel=1
arch=('x86_64')
license="MIT"
`}, FixSafe, nil)["PKGBUILD"]
		mustContain(t, got, `license=("MIT")`)
	})
	t.Run("dynamic scalar is left alone (wrapping would change word splitting)", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": `pkgname=demo
pkgver=1
pkgrel=1
arch=('x86_64')
depends=$_deps
`}, FixSafe, nil)
		if _, ok := got["PKGBUILD"]; ok {
			t.Errorf("dynamic scalar should not be wrapped, got:\n%s", got["PKGBUILD"])
		}
	})
	// Indexed element writes are valid array updates; wrapping one produced
	// `sha512sums[6]=('SKIP')`, which bash rejects ("cannot assign list to
	// array member") — and only the last write was touched, since earlier
	// ones were clobbered in Vars.
	t.Run("indexed element writes are left alone", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": `pkgname=demo
pkgver=1
pkgrel=1
arch=('x86_64')
sha512sums=('a' 'b' 'c' 'd' 'e' 'f' 'g')
sha512sums[4]='SKIP'
sha512sums[6]='SKIP'
`}, FixSafe, nil)
		if out, ok := got["PKGBUILD"]; ok {
			t.Errorf("indexed writes should not be rewritten, got:\n%s", out)
		}
	})
}

// Unsafe fixes must not run under the safe level. (The safe PB916 fix may
// still touch the same line, so the assertion is about PB204's edit, not
// about the file being untouched.)
func TestFixLevelGating(t *testing.T) {
	body := `
build() {
  go build -o demo .
}`
	if got := fixPKGBUILD(t, body, FixSafe, nil); strings.Contains(got, "go mod download") {
		t.Errorf("FixSafe should not apply the unsafe PB204 fix, got:\n%s", got)
	}
	if got := fixPKGBUILD(t, body, FixUnsafe, nil); !strings.Contains(got, "go mod download") {
		t.Errorf("FixUnsafe should apply PB204, got:\n%s", got)
	}
}

// An inline suppression on the finding's line must also suppress its fix.
// Run at FixUnsafe, where the PB203 fix is otherwise eligible, so the
// directive is what blocks it.
func TestFixSuppression(t *testing.T) {
	got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  # pkglint: ignore=PB203
  cargo build --release
}`)}, FixUnsafe, nil, "PB203")
	if _, ok := got["PKGBUILD"]; ok {
		t.Errorf("suppressed rule should not be fixed, got:\n%s", got["PKGBUILD"])
	}
}

// Applying every fix twice must be a no-op the second time.
func TestFixIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "PKGBUILD")
	src := pkgbuildWith("", `
build() {
  cargo build --release
  go build -o demo .
  npm install
}
package() {
  chmod 4755 "$pkgdir/usr/bin/demo"
}`)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	apply := func() bool {
		pkg, err := pkgbuild.Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		results := Fix(pkg, nil, FixUnsafe, nil)
		changed := false
		for _, r := range results {
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
}

// contractFixEnv is the stub the example contract runs fixes under. Every
// capability answers, so the test is about whether a fixer clears its own
// finding rather than about what a network happened to say: ResolveRef hands
// back a fixed hash, ProbeHTTPS agrees the URL is served, and LocalDigest
// reports the demo digests for anything asked about. None of it touches the
// network or the disk.
func contractFixEnv() *FixEnv {
	return &FixEnv{
		ResolveRef: fakeResolve,
		ProbeHTTPS: func(string) error { return nil },
		LocalDigest: func(string, string) (Digests, error) {
			return Digests{MD5: demoMD5, SHA1: demoSHA1, SHA256: demoSHA256}, nil
		},
	}
}

// fixExampleGaps lists fixable rules whose Bad example this contract cannot
// round-trip, with the reason. Keep it SMALL and justified: every entry is a
// fix whose "does it actually clear the finding" question goes unasked here,
// and must therefore be answered by a test of its own.
var fixExampleGaps = map[string]string{
	"PB102": "the fix writes the digest LocalDigest reports for the fetched source, and the stub " +
		"cannot know which of the example's several sources it is being asked about; " +
		"TestFixWeakChecksums covers the agreement between declared and reported digests.",
}

// TestFixesClearTheirExample is the contract every auto-fix owes its rule: the
// Bad example trips the rule, the rule's own fix is applied to it, and the
// result no longer trips. A fix that leaves its finding standing is a diff for
// nothing, and one that trips a *different* rule it did not before has traded
// one complaint for another.
//
// Applying the fix a second time must change nothing. TestFixIdempotent asks
// this of one hand-written PKGBUILD; asking it of every example is what makes
// the guarantee hold for fixes nobody thought to add to that fixture.
func TestFixesClearTheirExample(t *testing.T) {
	for _, r := range Registry() {
		r := r
		if !r.FixLevel.Fixable() || r.Scope == ScopePackage {
			continue
		}
		if reason, skip := fixExampleGaps[r.ID]; skip {
			t.Logf("SKIP %s (known gap): %s", r.ID, reason)
			continue
		}
		if reason, skip := knownGaps[r.ID]; skip {
			t.Logf("SKIP %s (example gap): %s", r.ID, reason)
			continue
		}
		t.Run(r.ID, func(t *testing.T) {
			files := packageFor(r.ID, "bad", r.Bad)
			if n := ruleIDs(lint(t, files))[r.ID]; n == 0 {
				t.Fatalf("%s: Bad example does not trip the rule, so there is nothing to fix", r.ID)
			}
			env := contractFixEnv()
			fixed := fixAll(t, files, r.FixLevel, env)
			if len(fixed) == 0 {
				t.Fatalf("%s: %s left the Bad example unchanged", r.ID, r.FixLevel.Flag())
			}
			for name, content := range fixed {
				files[name] = content
			}
			if n := ruleIDs(lint(t, files))[r.ID]; n != 0 {
				t.Errorf("%s: %d finding(s) survive the fix:\n%s", r.ID, n, files["PKGBUILD"])
			}
			if again := fixAll(t, files, r.FixLevel, env); len(again) != 0 {
				t.Errorf("%s: fixing the fixed example changed it again:\n%s", r.ID, again["PKGBUILD"])
			}
		})
	}
}

// TestFixExampleGapsAreStillGaps keeps that allowlist honest: an entry whose
// example does round-trip is no longer a gap and must be removed.
func TestFixExampleGapsAreStillGaps(t *testing.T) {
	for id := range fixExampleGaps {
		r, ok := RuleByID(id)
		if !ok {
			t.Errorf("fixExampleGaps lists unknown rule %s", id)
			continue
		}
		if !r.FixLevel.Fixable() {
			t.Errorf("fixExampleGaps lists %s, which has no fix; remove it", id)
			continue
		}
		files := packageFor(id, "bad", r.Bad)
		for name, content := range fixAll(t, files, r.FixLevel, contractFixEnv()) {
			files[name] = content
		}
		if ruleIDs(lint(t, files))[id] == 0 {
			t.Errorf("%s is in fixExampleGaps but its fix now clears the example; remove it", id)
		}
	}
}

func TestApplyEdits(t *testing.T) {
	t.Run("non-overlapping edits apply back to front", func(t *testing.T) {
		raw := []byte("abcdefgh")
		out, applied := ApplyEdits(raw, []Edit{
			{Start: 2, End: 4, New: "YY"},
			{Start: 0, End: 1, New: "X"},
		})
		if string(out) != "XbYYefgh" {
			t.Errorf("got %q", out)
		}
		if len(applied) != 2 {
			t.Errorf("expected 2 applied, got %d", len(applied))
		}
	})
	t.Run("insertion", func(t *testing.T) {
		out, _ := ApplyEdits([]byte("ab"), []Edit{{Start: 1, End: 1, New: "-"}})
		if string(out) != "a-b" {
			t.Errorf("got %q", out)
		}
	})
	t.Run("overlapping edits keep the earlier one", func(t *testing.T) {
		out, applied := ApplyEdits([]byte("abcdef"), []Edit{
			{Start: 0, End: 3, New: "X"},
			{Start: 1, End: 2, New: "Y"},
		})
		if string(out) != "Xdef" {
			t.Errorf("got %q", out)
		}
		if len(applied) != 1 {
			t.Errorf("expected 1 applied (overlap dropped), got %d", len(applied))
		}
	})
	t.Run("out-of-range edits are dropped", func(t *testing.T) {
		out, applied := ApplyEdits([]byte("ab"), []Edit{{Start: 5, End: 9, New: "X"}})
		if string(out) != "ab" || len(applied) != 0 {
			t.Errorf("got %q applied=%d", out, len(applied))
		}
	})
}

// TestFixLevelContract pins the FixLevel accessors the report-card site
// invokes from its templates (the fixpill badge); reflection there means the
// compiler cannot catch a rename or behavior change.
func TestFixLevelContract(t *testing.T) {
	for _, tc := range []struct {
		level   FixLevel
		fixable bool
		safe    bool
		flag    string
	}{
		{FixNone, false, false, ""},
		{FixSafe, true, true, "--fix"},
		{FixUnsafe, true, false, "--unsafe-fix"},
	} {
		if got := tc.level.Fixable(); got != tc.fixable {
			t.Errorf("FixLevel(%d).Fixable() = %v, want %v", tc.level, got, tc.fixable)
		}
		if got := tc.level.Safe(); got != tc.safe {
			t.Errorf("FixLevel(%d).Safe() = %v, want %v", tc.level, got, tc.safe)
		}
		if got := tc.level.Flag(); got != tc.flag {
			t.Errorf("FixLevel(%d).Flag() = %q, want %q", tc.level, got, tc.flag)
		}
	}
}

// customVarPKGBUILD is a package whose only irregularity is the custom
// variable spelling under test, so what the fixer does with it is unambiguous.
func customVarPKGBUILD(body string) string {
	return `pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='https://example.com/demo'
license=('MIT')
source=("https://example.com/demo-$pkgver.tar.gz")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')
` + body + "\n"
}

func fixCustomVar(t *testing.T, body string) (string, bool) {
	t.Helper()
	got, ok := fixAll(t, map[string]string{"PKGBUILD": customVarPKGBUILD(body)}, FixUnsafe, nil)["PKGBUILD"]
	return got, ok
}

func TestFixUnprefixedCustomVars(t *testing.T) {
	t.Run("declaration and every reference are renamed together", func(t *testing.T) {
		got, _ := fixCustomVar(t, `pyname=demolib
commit=abc123
build() {
  cd "$srcdir/${pyname}-$pkgver"
  echo "$commit" > rev
  make PY=$pyname
}`)
		mustContain(t, got, "_pyname=demolib")
		mustContain(t, got, "_commit=abc123")
		mustContain(t, got, `cd "$srcdir/${_pyname}-$pkgver"`)
		mustContain(t, got, `echo "$_commit" > rev`)
		mustContain(t, got, "make PY=$_pyname")
		mustNotContain(t, got, "$pyname")
	})

	// Arithmetic names a variable without a $, and the rename has to follow it
	// there or the file starts reading an empty one.
	t.Run("arithmetic operands are renamed", func(t *testing.T) {
		got, _ := fixCustomVar(t, `count=1
build() {
  ((count++))
  echo $((count + 1))
}`)
		mustContain(t, got, "_count=1")
		mustContain(t, got, "((_count++))")
		mustContain(t, got, "$((_count + 1))")
	})

	t.Run("unset and for-loop iterators are renamed", func(t *testing.T) {
		got, _ := fixCustomVar(t, `mod=a
build() {
  for mod in a b; do echo "$mod"; done
  unset mod
}`)
		mustContain(t, got, "for _mod in a b; do echo \"$_mod\"; done")
		mustContain(t, got, "unset _mod")
	})

	t.Run("the underscored name being taken blocks the rename", func(t *testing.T) {
		if got, ok := fixCustomVar(t, `pyname=demolib
_pyname=other`); ok {
			t.Errorf("rename onto an existing name should be declined, got:\n%s", got)
		}
	})

	// An exported variable is read by whatever the build runs, and no walk of
	// the PKGBUILD can see that side of it.
	t.Run("exported variables are left alone", func(t *testing.T) {
		for _, body := range []string{
			"prefix=/usr\nexport prefix",
			"export prefix=/usr",
			"declare -x prefix=/usr",
		} {
			if got, ok := fixCustomVar(t, body); ok {
				t.Errorf("exported variable should not be renamed, got:\n%s", got)
			}
		}
	})

	t.Run("an environment prefix on a command is left alone", func(t *testing.T) {
		if got, ok := fixCustomVar(t, `flags=-O2
build() {
  flags=-O2 make
}`); ok {
			t.Errorf("command-environment assignment should not be renamed, got:\n%s", got)
		}
	})

	// A mention the walk cannot classify might be a reference; the rename is
	// only offered when every occurrence is accounted for.
	t.Run("an unaccountable mention blocks the rename", func(t *testing.T) {
		for _, body := range []string{
			"target=all\nbuild() {\n  make target\n}",              // bare word argument
			"target=all\nbuild() {\n  make 'target'\n}",            // quoted the same way
			"target=all\nbuild() {\n  [[ -v target ]] && :\n}",     // a test on the name
			"target=all\nbuild() {\n  read target < in\n}",         // an assignment this walk does not model
			"target=all\nbuild() {\n  unset -f target\n}",          // names a function, not the variable
			"target=all\nbuild() {\n  unset -fv target\n}",         // bundled spelling of the same flag
			"target=all\nbuild() {\n  unset \"$w\" target\n}",      // $w could be -f, or any name
			"target=all\nbuild() {\n  \"un\"${w}\"set\" target\n}", // maybe-unset built at run time
			"target=all\nbuild() {\n  trap 'echo $target' EXIT\n}", // reference in deferred shell text
			"target=all\nbuild() {\n  sh -c \"echo \\$target\"\n}", // reference behind an escape
			"target=all\nbuild() {\n  sh <<'EOT'\n$target\nEOT\n}", // reference in a quoted heredoc
		} {
			if got, ok := fixCustomVar(t, body); ok {
				t.Errorf("unaccountable mention should block the rename, got:\n%s", got)
			}
		}
	})

	// Run-time naming can reach any variable by a name no walk can predict —
	// and a sourced file can read any variable with no trace here — so either
	// stands every rename in the file down, not just the one it names.
	t.Run("dynamic naming or sourcing blocks every rename in the file", func(t *testing.T) {
		for _, hazard := range []string{
			`eval "echo $pyname"`,
			`echo "${!pyname}"`,
			`echo "${!py@}"`, // prefix listing: a rename changes what it returns
			`declare -n ref=pyname`,
			`source helpers.sh`,
			`. "$srcdir/helpers.sh"`,
		} {
			body := "pyname=demolib\ncommit=abc123\nbuild() {\n  " + hazard + "\n  echo \"$commit\"\n}"
			if got, ok := fixCustomVar(t, body); ok {
				t.Errorf("%s should block renames, got:\n%s", hazard, got)
			}
		}
	})

	// `${!arr[@]}` lists arr's indices: unlike every other ${!...} it names
	// arr itself, so it neither blocks the file nor escapes the rename.
	t.Run("array-indices expansion renames like any reference", func(t *testing.T) {
		got, _ := fixCustomVar(t, `mods=(a b)
build() {
  for i in "${!mods[@]}"; do echo "${mods[$i]}"; done
}`)
		mustContain(t, got, `"${!_mods[@]}"`)
		mustContain(t, got, `"${_mods[$i]}"`)
	})

	// The literal-text veto wants a whole reference, not a prefix of a longer
	// name: text mentioning $python must not pin down a variable named py.
	t.Run("a longer name in literal text does not block", func(t *testing.T) {
		got, _ := fixCustomVar(t, `py=demolib
build() {
  sed 's/$python/x/' f
  echo "$py"
}`)
		mustContain(t, got, "_py=demolib")
		mustContain(t, got, `echo "$_py"`)
		mustContain(t, got, "sed 's/$python/x/' f")
	})

	t.Run("known makepkg fields are never renamed", func(t *testing.T) {
		got, ok := fixCustomVar(t, `build() {
  echo "$pkgver"
}`)
		if ok {
			t.Errorf("schema fields should not be renamed, got:\n%s", got)
		}
	})

	// A directive on any line of the rename waives all of it: renaming the
	// references while the declaration keeps its name would break the build.
	t.Run("an inline directive waives the whole rename", func(t *testing.T) {
		got, ok := fixCustomVar(t, `# pkglint: ignore=PB902
pyname=demolib
build() {
  echo "$pyname"
}`)
		if ok {
			t.Errorf("suppressed rename should not be applied, got:\n%s", got)
		}
	})

	t.Run("the renamed file is clean of PB902", func(t *testing.T) {
		got, _ := fixCustomVar(t, `pyname=demolib
build() {
  echo "${pyname}"
}`)
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		pkg, err := pkgbuild.Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range Run(pkg, nil) {
			if f.RuleID == "PB902" {
				t.Errorf("PB902 still reported after the fix: %s", f.Message)
			}
		}
	})
}

// A grouped rewrite must not be applied in part: if one of its edits is
// dropped for overlapping another, the rest go with it.
func TestApplyEditsGroupsAreAllOrNothing(t *testing.T) {
	raw := []byte("abcdefgh")
	out, applied := ApplyEdits(raw, []Edit{
		{Start: 0, End: 4, New: "X"},                  // wins the overlap
		{Start: 2, End: 3, New: "Y", Group: "rename"}, // dropped, taking its group
		{Start: 6, End: 7, New: "Z", Group: "rename"},
	})
	if string(out) != "Xefgh" {
		t.Errorf("got %q, want %q", out, "Xefgh")
	}
	if len(applied) != 1 || applied[0].New != "X" {
		t.Errorf("expected only the ungrouped edit to apply, got %+v", applied)
	}
}

// Dropping a broken group frees the range its members held, so an edit they
// displaced gets its turn on the next round.
func TestApplyEditsGroupDropFreesOverlap(t *testing.T) {
	out, applied := ApplyEdits([]byte("abcdefgh"), []Edit{
		{Start: 0, End: 2, New: "X", Group: "rename"}, // group breaks below
		{Start: 0, End: 4, New: "Y"},                  // displaced by it at first
		{Start: 5, End: 6, New: "Z"},                  // breaks the group by overlapping
		{Start: 5, End: 7, New: "W", Group: "rename"},
	})
	if string(out) != "YeZgh" {
		t.Errorf("got %q, want %q", out, "YeZgh")
	}
	if len(applied) != 2 {
		t.Errorf("expected 2 applied, got %+v", applied)
	}
}

// PB103 rewrites a whole source element, PB902 rewrites a variable reference
// inside one. When they collide the rename must lose as a unit: applying its
// other edits would leave the element referring to a variable the file no
// longer sets.
func TestFixRenameYieldsToOverlappingEdit(t *testing.T) {
	got := fixAll(t, map[string]string{"PKGBUILD": `pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='https://example.com/demo'
license=('MIT')
tagname=v$pkgver
source=("git+https://example.com/demo.git#tag=$tagname")
sha256sums=('SKIP')
build() {
  echo "$tagname"
}
`}, FixUnsafe, &FixEnv{ResolveRef: fakeResolve})["PKGBUILD"]
	mustContain(t, got, "#commit=0123456789abcdef")
	mustContain(t, got, "tagname=v$pkgver")
	mustContain(t, got, `echo "$tagname"`)
	mustNotContain(t, got, "_tagname")
}

// Arrays reach their name through more spellings than a scalar does — an
// append, an element write, a length, a whole-array expansion, and expansions
// nested in a heredoc or a command substitution. All of them are the same
// variable, so all of them move together.
func TestFixUnprefixedCustomVarsArrayForms(t *testing.T) {
	got, _ := fixCustomVar(t, `mods=(a b)
mods+=(c)
mods[1]=z
build() {
  local n=${#mods[@]}
  cat <<EOT
${mods[0]} and $n
EOT
  echo "$(printf '%s\n' "${mods[@]}")"
}`)
	for _, want := range []string{
		"_mods=(a b)", "_mods+=(c)", "_mods[1]=z",
		"${#_mods[@]}", "${_mods[0]}", "${_mods[@]}",
	} {
		mustContain(t, got, want)
	}
	mustNotContain(t, got, "${mods")
}

// The Go flag fixes put the flag where the PKGBUILD keeps its flags, in the
// guidelines' order, rather than always after the verb.
func TestFixGoFlagPlacement(t *testing.T) {
	t.Run("into the GOFLAGS export the command inherits, once for every command", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  export GOFLAGS="-buildmode=pie -mod=readonly -modcacherw"
  go build -o demo ./cmd/demo
  go build -o democtl ./cmd/democtl
}`)}, FixUnsafe, nil)["PKGBUILD"]
		mustContain(t, got, `export GOFLAGS="-buildmode=pie -trimpath -mod=readonly -modcacherw"`)
		mustContain(t, got, "  go build -o demo ./cmd/demo\n  go build -o democtl ./cmd/democtl\n")
	})

	t.Run("into the array the command spreads, one element per line", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  local common=(
    -v
    -buildmode=pie
    -mod=readonly
    -modcacherw
  )
  go build "${common[@]}" -o build ./cmd/...
  go build "${common[@]}" -tags oss -o build-oss ./cmd/...
}`)}, FixUnsafe, nil)["PKGBUILD"]
		mustContain(t, got, "    -buildmode=pie\n    -trimpath\n    -mod=readonly\n")
		if strings.Count(got, "-trimpath") != 1 {
			t.Errorf("flag inserted %d times, want once in the array:\n%s", strings.Count(got, "-trimpath"), got)
		}
	})

	t.Run("into the array GOFLAGS is assembled from", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  local goflags=(-buildmode=pie -mod=readonly -modcacherw)
  export GOFLAGS="${goflags[*]}"
  go build -o demo .
}`)}, FixUnsafe, nil)["PKGBUILD"]
		mustContain(t, got, "local goflags=(-buildmode=pie -trimpath -mod=readonly -modcacherw)")
	})

	t.Run("as a continuation line when the command lays its flags out that way", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  go build -v \
    -buildmode=pie \
    -mod=readonly \
    -modcacherw \
    -o build ./...
}`)}, FixUnsafe, nil)["PKGBUILD"]
		mustContain(t, got, "    -buildmode=pie \\\n    -trimpath \\\n    -mod=readonly \\\n")
	})

	t.Run("after the verb when the command has no flags to slot among", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  go build -o demo .
}`)}, FixUnsafe, nil)["PKGBUILD"]
		mustContain(t, got, "go build -buildmode=pie -trimpath -modcacherw -o demo .")
	})

	t.Run("a GOFLAGS export below the command does not draw the edit", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  go build -buildmode=pie -modcacherw -o demo .
  export GOFLAGS="-buildmode=pie"
}`)}, FixUnsafe, nil)["PKGBUILD"]
		mustContain(t, got, "go build -buildmode=pie -trimpath -modcacherw -o demo .")
		mustContain(t, got, `export GOFLAGS="-buildmode=pie"`)
	})
}
