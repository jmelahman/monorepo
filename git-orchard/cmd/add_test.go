package cmd

import "testing"

func TestRepoName(t *testing.T) {
	for url, want := range map[string]string{
		"git@github.com:owner/foo.git":      "foo",
		"https://github.com/owner/foo":      "foo",
		"https://github.com/owner/foo.git/": "foo",
		"ssh://git@example.com/srv/foo.git": "foo",
		"/srv/git/foo/.git":                 "foo",
		"../foo":                            "foo",
		"host:foo.git":                      "foo",
		"https://example.com/":              "example.com",
	} {
		if got := RepoName(url); got != want {
			t.Errorf("RepoName(%q) = %q, want %q", url, got, want)
		}
	}
}
