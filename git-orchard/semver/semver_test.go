package semver

import (
	"slices"
	"testing"
)

func version(major, minor, patch uint64) Version {
	return Version{Major: major, Minor: minor, Patch: patch}
}

func preRelease(major, minor, patch uint64, name string, num uint64) Version {
	return Version{Major: major, Minor: minor, Patch: patch, PreRelease: name, PreReleaseNum: num}
}

func prefixed(prefix string, v Version) Version {
	v.Prefix = prefix
	return v
}

// TestParse locks in tag's parsing, quirks included.
func TestParse(t *testing.T) {
	for _, c := range []struct {
		tag  string
		want Version
	}{
		{"v1.2.3", version(1, 2, 3)},
		{"org/v1.2.3", prefixed("org/", version(1, 2, 3))},
		// The prefix is greedy, so it swallows every leading path segment.
		{"a/b/v1.2.3", prefixed("a/b/", version(1, 2, 3))},
		{"prefix/with/slashes/v9.9.9-beta.7", prefixed("prefix/with/slashes/", preRelease(9, 9, 9, "beta", 7))},
		{"//v1.2.3", prefixed("//", version(1, 2, 3))},
		{"v1.2.3/v4.5.6", prefixed("v1.2.3/", version(4, 5, 6))},
		// A prefix needs at least one character before the slash.
		{"/v1.2.3", version(1, 2, 3)},
		// The search is unanchored, so leading junk is skipped...
		{"xxv1.2.3", version(1, 2, 3)},
		{"-v1.2.3", version(1, 2, 3)},
		// ...and trailing junk, including build metadata, is ignored.
		{"v1.2.3+21AF26D3", version(1, 2, 3)},
		{"v1.2.3.4", version(1, 2, 3)},
		{"v1.2.3-rc.1-extra", preRelease(1, 2, 3, "rc", 1)},
		// A pre-release that doesn't match in full is dropped, not fatal.
		{"v1.2.3-rc", preRelease(1, 2, 3, "rc", 0)},
		{"v1.2.3-rc.x", preRelease(1, 2, 3, "rc", 0)},
		{"v1.2.3-1", version(1, 2, 3)},
		{"v1.2.3-RC.2", preRelease(1, 2, 3, "RC", 2)},
		// Leading zeroes are accepted and normalized away.
		{"v01.02.03", version(1, 2, 3)},
		{"v1.2.3-rc.01", preRelease(1, 2, 3, "rc", 1)},
		{"v99999999999999999999.0.0", version(1<<64-1, 0, 0)},
	} {
		got, err := Parse(c.tag)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.tag, err)
		} else if got != c.want {
			t.Errorf("Parse(%q) = %#v, want %#v", c.tag, got, c.want)
		}
	}
	for _, tag := range []string{"invalid-tag", "not-a-version", "v1.2", "v-1.2.3", ""} {
		if v, err := Parse(tag); err == nil {
			t.Errorf("Parse(%q) = %v, want an error", tag, v)
		}
	}
}

func TestStringRoundTrips(t *testing.T) {
	for _, tag := range []string{"v1.2.3", "org/v1.2.3", "v1.2.3-rc", "org/v1.2.3-rc.1"} {
		v, err := Parse(tag)
		if err != nil {
			t.Fatal(err)
		}
		if got := v.String(); got != tag {
			t.Errorf("%q round-trips as %q", tag, got)
		}
	}
}

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		name string
		a, b Version
		want int
	}{
		{"major higher", version(2, 0, 0), version(1, 9, 9), 1},
		{"minor higher", version(1, 2, 0), version(1, 1, 9), 1},
		{"patch higher", version(1, 1, 2), version(1, 1, 1), 1},
		{"pre-release number higher", preRelease(1, 1, 1, "rc", 2), preRelease(1, 1, 1, "rc", 1), 1},
		{"identical", version(1, 1, 1), version(1, 1, 1), 0},
		{"major lower", version(1, 9, 9), version(2, 0, 0), -1},
		{"stable outranks pre-release", version(1, 1, 1), preRelease(1, 1, 1, "rc", 1), 1},
		{"pre-release under stable", preRelease(1, 1, 1, "rc", 1), version(1, 1, 1), -1},
		// Unlike tag, where these are incomparable, pre-releases order by name.
		{"pre-releases order by name", preRelease(1, 1, 1, "rc", 1), preRelease(1, 1, 1, "alpha", 9), 1},
		{"unnumbered pre-release first", preRelease(1, 1, 1, "rc", 0), preRelease(1, 1, 1, "rc", 1), -1},
		{"prefix is ignored", prefixed("a/", version(1, 0, 0)), prefixed("b/", version(1, 0, 0)), 0},
	} {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("%s: Compare(%v, %v) = %d, want %d", c.name, c.a, c.b, got, c.want)
		}
	}
}

func TestPredecessor(t *testing.T) {
	for _, c := range []struct {
		name, tag, want string
	}{
		{"patch decrement", "v1.1.2", "v1.1.1"},
		{"minor decrement", "v1.1.0", "v1.0.0"},
		{"major decrement", "v1.0.0", "v0.0.0"},
		{"no predecessor for v0.0.0", "v0.0.0", ""},
		{"pre-release number > 1 decrements it", "v1.2.3-rc.2", "v1.2.3-rc.1"},
		{"pre-release number 1 drops to the bare pre-release", "v1.2.3-rc.1", "v1.2.3-rc"},
		{"unnumbered pre-release falls back to stable", "v1.2.3-rc", "v1.2.2"},
		{"unnumbered pre-release on a minor boundary", "v1.2.0-alpha", "v1.1.0"},
		{"unnumbered pre-release on a major boundary", "v2.0.0-beta", "v1.0.0"},
		{"prefixed pre-release number decrement", "org/v1.0.0-rc.3", "org/v1.0.0-rc.2"},
		{"pre-release v0.0.0 has no predecessor", "v0.0.0-alpha", ""},
	} {
		v, err := Parse(c.tag)
		if err != nil {
			t.Fatal(err)
		}
		got := ""
		if pred, ok := v.Predecessor(); ok {
			got = pred.String()
		}
		if got != c.want {
			t.Errorf("%s: %s's predecessor is %q, want %q", c.name, c.tag, got, c.want)
		}
	}
}

func parseAll(t *testing.T, tags ...string) []Version {
	t.Helper()
	vs := make([]Version, len(tags))
	for i, tag := range tags {
		v, err := Parse(tag)
		if err != nil {
			t.Fatal(err)
		}
		vs[i] = v
	}
	return vs
}

func tagsOf(vs []Version) []string {
	s := make([]string, len(vs))
	for i, v := range vs {
		s[i] = v.String()
	}
	return s
}

func TestLarger(t *testing.T) {
	all := parseAll(t, "v1.0.0", "v1.2.3", "v1.3.0", "v2.0.0", "v1.3.0-rc.1")
	if got, want := tagsOf(parseAll(t, "v1.2.3")[0].Larger(all)), []string{"v1.3.0", "v2.0.0"}; !slices.Equal(got, want) {
		t.Errorf("larger than v1.2.3: %v, want %v", got, want)
	}
	if got := parseAll(t, "v2.0.0")[0].Larger(all); len(got) != 0 {
		t.Errorf("larger than v2.0.0: %v", got)
	}
	// Only versions with the same prefix count.
	all = parseAll(t, "org/v2.0.0", "v3.0.0")
	if got, want := tagsOf(parseAll(t, "org/v1.0.0")[0].Larger(all)), []string{"org/v2.0.0"}; !slices.Equal(got, want) {
		t.Errorf("larger than org/v1.0.0: %v, want %v", got, want)
	}
}

func TestNext(t *testing.T) {
	for _, c := range []struct {
		name     string
		current  string
		inc      Increment
		suffix   string
		existing []string
		want     string
	}{
		{"patch increment", "v1.2.3", Auto, "", nil, "v1.2.4"},
		{"patch increment with Patch", "v1.2.3", Patch, "", nil, "v1.2.4"},
		{"next tag already exists", "v1.2.3", Auto, "", []string{"v1.2.3", "v1.3.0"}, "v1.2.4"},
		{"patch increment with suffix", "v1.2.3", Patch, "alpha", nil, "v1.2.4-alpha"},
		{"minor increment", "v1.2.3", Minor, "", nil, "v1.3.0"},
		{"minor increment with suffix", "v1.2.3", Minor, "alpha", nil, "v1.3.0-alpha"},
		{"major increment", "v1.2.3", Major, "", nil, "v2.0.0"},
		{"major increment with suffix", "v1.2.3", Major, "alpha", nil, "v2.0.0-alpha"},
		{"pre-release increment", "v1.2.3-rc.1", Auto, "rc", nil, "v1.2.3-rc.2"},
		{"pre-release with patch increment", "v1.2.3-rc.1", Patch, "rc", nil, "v1.2.4-rc"},
		{"pre-release with minor increment", "v1.2.3-rc.1", Minor, "rc", nil, "v1.3.0-rc"},
		{"pre-release with major increment", "v1.2.3-rc.1", Major, "rc", nil, "v2.0.0-rc"},
		{"add suffix to version", "v1.2.3", Auto, "beta", nil, "v1.2.3-beta"},
		{"add suffix to pre-release version", "v1.1.1-beta", Auto, "beta", nil, "v1.1.1-beta.1"},
		{"switching pre-release keeps the current number", "v1.2.3-rc.1", Auto, "beta", nil, "v1.2.3-beta.1"},
		{"switching pre-release continues numbering", "v1.2.3-rc.1", Auto, "beta", []string{"v1.2.3-rc.1", "v1.2.3-beta.2"}, "v1.2.3-beta.3"},
		{"same pre-release ignores others", "v1.2.3-rc.1", Auto, "rc", []string{"v1.2.3-rc.1", "v1.2.3-beta.2"}, "v1.2.3-rc.2"},
		{"no suffix leaves the pre-release", "v1.2.3-rc.1", Auto, "", []string{"v1.2.3-rc.1", "v1.2.3-beta.2"}, "v1.2.4"},
		{"suffix on a stable base", "v1.0.1", Auto, "alpha", []string{"v1.0.0", "v1.0.0-alpha", "v1.0.1"}, "v1.0.1-alpha"},
		{"suffix on a stable base ignores older pre-releases", "v1.0.1", Auto, "alpha", []string{"v1.0.0", "v1.0.0-alpha.1", "v1.0.1"}, "v1.0.1-alpha"},
		{"prefix is preserved", "org/v1.2.3", Auto, "", nil, "org/v1.2.4"},
	} {
		current := parseAll(t, c.current)[0]
		if got := current.Next(c.inc, c.suffix, parseAll(t, c.existing...)).String(); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}
