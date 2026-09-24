package cmd

import "testing"

func TestRemoteTemplate(t *testing.T) {
	for url, want := range map[string]string{
		"git@github.com:jmelahman/monorepo.git":      "git@github.com:jmelahman/{name}.git",
		"https://github.com/jmelahman/monorepo":      "https://github.com/jmelahman/{name}",
		"ssh://git@example.com/srv/git/monorepo.git": "ssh://git@example.com/srv/git/{name}.git",
	} {
		if got := RemoteTemplate(url); got != want {
			t.Errorf("RemoteTemplate(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestParseGitHubRepo(t *testing.T) {
	for url, want := range map[string]string{
		"git@github.com:jmelahman/monorepo.git":                  "jmelahman/monorepo",
		"https://github.com/jmelahman/monorepo":                  "jmelahman/monorepo",
		"https://github.com/jmelahman/monorepo.git/":             "jmelahman/monorepo",
		"ssh://git@github.com/jmelahman/jmelahman.github.io.git": "jmelahman/jmelahman.github.io",
		"git@gitlab.com:jmelahman/monorepo.git":                  "",
	} {
		owner, repo, ok := ParseGitHubRepo(url)
		got := ""
		if ok {
			got = owner + "/" + repo
		}
		if got != want {
			t.Errorf("ParseGitHubRepo(%q) = %q, want %q", url, got, want)
		}
	}
}
