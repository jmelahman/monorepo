package rules

import (
	"strings"
	"testing"
)

func TestGoGuidelineFlagRules(t *testing.T) {
	bare := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  go build -o demo .
}`)}
	t.Run("bare go build trips all four rules", func(t *testing.T) {
		ids := ruleIDs(lint(t, bare))
		for _, id := range []string{"PB914", "PB915", "PB916", "PB917"} {
			if ids[id] == 0 {
				t.Errorf("expected %s on a bare go build, got %v", id, ids)
			}
		}
	})

	t.Run("guideline GOFLAGS export inside build() satisfies the flag rules", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  export CGO_CFLAGS="$CFLAGS" CGO_LDFLAGS="$LDFLAGS"
  export GOFLAGS="-buildmode=pie -trimpath -mod=readonly -modcacherw"
  go build -o demo .
}`)}
		ids := ruleIDs(lint(t, files))
		for _, id := range []string{"PB914", "PB915", "PB916", "PB917"} {
			if ids[id] != 0 {
				t.Errorf("%s still fires with the guideline exports: %v", id, ids)
			}
		}
	})

	t.Run("flags on the command itself count", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  CGO_ENABLED=0 go build -buildmode=pie -trimpath -modcacherw -o demo .
}`)}
		ids := ruleIDs(lint(t, files))
		for _, id := range []string{"PB914", "PB915", "PB916", "PB917"} {
			if ids[id] != 0 {
				t.Errorf("%s still fires with per-command flags: %v", id, ids)
			}
		}
	})

	t.Run("an explicit other buildmode is a decision, not a finding", func(t *testing.T) {
		expectNoRule(t, "PB914", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  go build -buildmode=c-shared -o libdemo.so .
}`)})
	})

	t.Run("appending to GOFLAGS is seen through the expansion", func(t *testing.T) {
		expectNoRule(t, "PB915", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  export GOFLAGS="$GOFLAGS -trimpath"
  go build -buildmode=pie -o demo .
}`)})
	})

	t.Run("bare GOFLAGS statements count once GOFLAGS is exported", func(t *testing.T) {
		// seafile-server builds GOFLAGS up line by line and exports it once.
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  GOFLAGS='-buildmode=pie'
  GOFLAGS+=' -trimpath'
  GOFLAGS+=' -modcacherw'
  export GOFLAGS
  for d in a b; do
    pushd "$d"
    go build .
    popd
  done
}`)}
		ids := ruleIDs(lint(t, files))
		for _, id := range []string{"PB914", "PB915", "PB916"} {
			if ids[id] != 0 {
				t.Errorf("%s fires on an exported GOFLAGS: %v", id, ids)
			}
		}
	})

	t.Run("a bare GOFLAGS that is never exported stays a shell variable", func(t *testing.T) {
		expectRule(t, "PB915", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  GOFLAGS='-buildmode=pie -trimpath'
  go build .
}`)})
	})

	t.Run("go mod download in prepare needs -modcacherw", func(t *testing.T) {
		expectRule(t, "PB916", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  go mod download
}`)})
		expectNoRule(t, "PB916", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  go mod download -modcacherw
}`)})
	})

	t.Run("GOFLAGS is scoped to the phases that can still see it", func(t *testing.T) {
		// build() exports GOFLAGS long after prepare() has finished, so the
		// download ran against a read-only cache regardless.
		expectRule(t, "PB916", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  go mod download
}
build() {
  export GOFLAGS="-buildmode=pie -trimpath -mod=readonly -modcacherw"
  go build -o demo .
}`)})
		// Set at the top level, it is sourced before every phase.
		expectNoRule(t, "PB916", map[string]string{"PKGBUILD": pkgbuildWith("", `
export GOFLAGS="-trimpath -modcacherw"

prepare() {
  go mod download
}`)})
		// Set in prepare(), it also reaches the later phases.
		expectNoRule(t, "PB916", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  export GOFLAGS="-trimpath -modcacherw"
  go mod download
}
build() {
  go build -o demo .
}`)})
	})

	t.Run("GOFLAGS assembled from an array is read through the reference", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  local goflags=(
    -buildmode=pie
    -trimpath
    -mod=readonly
    -modcacherw
  )
  export CGO_CFLAGS="$CFLAGS" CGO_LDFLAGS="$LDFLAGS"
  export GOFLAGS="${goflags[*]}"
  go build -o demo .
}`)}
		ids := ruleIDs(lint(t, files))
		for _, id := range []string{"PB914", "PB915", "PB916", "PB917"} {
			if ids[id] != 0 {
				t.Errorf("%s still fires with GOFLAGS built from an array: %v", id, ids)
			}
		}
	})

	t.Run("a flag array spread onto the command counts, and what it lacks is still reported", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  local common=(
    -v
    -buildmode=pie
    -mod=readonly
    -modcacherw
  )
  go build "${common[@]}" -o build ./cmd/...
  go build "${common[@]}" -tags oss -o build-oss ./cmd/...
}`)}
		ids := ruleIDs(lint(t, files))
		if ids["PB914"] != 0 || ids["PB916"] != 0 {
			t.Errorf("flags in the spread array not seen: %v", ids)
		}
		if ids["PB915"] != 2 {
			t.Errorf("PB915 fired %d times, want 2 (the array has no -trimpath)", ids["PB915"])
		}
	})

	t.Run("an export below the command it would cover does not count", func(t *testing.T) {
		expectRule(t, "PB915", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  go build -buildmode=pie -o demo .
  export GOFLAGS="-trimpath"
}`)})
	})

	t.Run("an environment prefix covers only its own command", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  GOFLAGS=-trimpath go build -buildmode=pie -o demo ./cmd/demo
  go build -buildmode=pie -o democtl ./cmd/democtl
}`)}
		if n := ruleIDs(lint(t, files))["PB915"]; n != 1 {
			t.Errorf("PB915 fired %d times, want 1 (only the unprefixed build)", n)
		}
	})

	t.Run("CGO_ENABLED=0 silences the cgo-forwarding rule", func(t *testing.T) {
		expectNoRule(t, "PB917", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  export CGO_ENABLED=0
  go build -o demo .
}`)})
	})

	t.Run("cgo rule reports once per PKGBUILD", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  go build -o demo ./cmd/demo
  go build -o democtl ./cmd/democtl
}`)}
		if n := ruleIDs(lint(t, files))["PB917"]; n != 1 {
			t.Errorf("PB917 fired %d times, want 1", n)
		}
	})

	t.Run("go in prepare is not held to artifact flags", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  go generate ./...
}`)}
		ids := ruleIDs(lint(t, files))
		for _, id := range []string{"PB914", "PB915", "PB917"} {
			if ids[id] != 0 {
				t.Errorf("%s fired on prepare()-phase go, got %v", id, ids)
			}
		}
	})
}

func TestFixGoGuidelineFlags(t *testing.T) {
	t.Run("flags are inserted after the verb, not appended", func(t *testing.T) {
		// go's flag parsing stops at the first non-flag argument, so a
		// trailing flag would be read as a package pattern.
		got := fixPKGBUILD(t, `
build() {
  go build -o demo .
}`, FixUnsafe, nil)
		mustContain(t, got, "go build -buildmode=pie -trimpath -modcacherw -o demo .")
	})

	t.Run("modcacherw alone is a safe fix, the rest are not", func(t *testing.T) {
		got := fixPKGBUILD(t, `
prepare() {
  go mod download
}
build() {
  export GOFLAGS="-buildmode=pie -trimpath"
  go build -o demo .
}`, FixSafe, nil)
		mustContain(t, got, "go mod download -modcacherw")
		mustNotContain(t, got, "build -buildmode")
	})

	t.Run("GOFLAGS coverage suppresses the insertion", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  export GOFLAGS="-buildmode=pie -trimpath -mod=readonly -modcacherw"
  go build -o demo .
}`)}, FixUnsafe, nil)["PKGBUILD"]
		if strings.Contains(got, "go build -buildmode") {
			t.Errorf("flags were inserted despite GOFLAGS carrying them:\n%s", got)
		}
	})

	t.Run("the download PB204 writes carries -modcacherw of its own", func(t *testing.T) {
		// GOFLAGS is exported in build(), a phase after the prepare() the fix
		// creates, so the inserted download cannot inherit the flag.
		got := fixPKGBUILD(t, `
build() {
  cd "$pkgname" || exit
  export GOFLAGS="-buildmode=pie -trimpath -mod=readonly -modcacherw"
  go build -o demo .
}`, FixUnsafe, nil)
		mustContain(t, got, "prepare() {\n  cd \"$pkgname\" || exit\n  go mod download -modcacherw\n}")
	})

	t.Run("a top-level GOFLAGS does cover the inserted download", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
export GOFLAGS="-buildmode=pie -trimpath -modcacherw"

build() {
  go build -o demo .
}`)}, FixUnsafe, nil)["PKGBUILD"]
		mustContain(t, got, "prepare() {\n  go mod download\n}")
		mustNotContain(t, got, "go mod download -modcacherw")
	})

	t.Run("fix is idempotent", func(t *testing.T) {
		first := fixPKGBUILD(t, `
build() {
  go build -o demo .
}`, FixUnsafe, nil)
		again := fixAll(t, map[string]string{"PKGBUILD": first}, FixUnsafe, nil)
		if out, ok := again["PKGBUILD"]; ok {
			t.Errorf("second pass should be a no-op, got:\n%s", out)
		}
	})
}
