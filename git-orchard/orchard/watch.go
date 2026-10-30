package orchard

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jmelahman/git-orchard/config"
	"github.com/jmelahman/git-orchard/ghrun"
)

// WatchOptions configure WatchRelease.
type WatchOptions struct {
	// Remote is the monorepo remote the tag was pushed to, whose runs are the
	// mirror's. Empty means the mirror has nothing to publish.
	Remote string
	// Upstream means the tag was pushed upstream directly, so the mirror has
	// nothing left to publish.
	Upstream bool
	// List lists a repository's runs; it defaults to ghrun.GH.
	List ghrun.Lister
	// Output is where progress goes; it defaults to os.Stderr.
	Output io.Writer
	// Watch configures the polling of each repository's runs.
	Watch ghrun.WatchOptions
}

// WatchRelease follows the GitHub Actions runs publishing tag, the release of
// version of s: the mirror's on opts.Remote, then the upstream's. It returns
// an error if any of them fails. ctx bounds the watch; the release carries on
// publishing without it.
func (o *Orchard) WatchRelease(ctx context.Context, s config.Subtree, version, tag string, opts WatchOptions) error {
	if opts.List == nil {
		opts.List = ghrun.GH
	}
	if opts.Output == nil {
		opts.Output = os.Stderr
	}
	if opts.Watch.Interval == 0 {
		opts.Watch.Interval = 5 * time.Second
	}
	if opts.Watch.Grace == 0 {
		opts.Watch.Grace = time.Minute
	}
	if opts.Watch.Sleep == nil {
		opts.Watch.Sleep = ghrun.Sleep
	}
	if opts.Watch.Now == nil {
		opts.Watch.Now = time.Now
	}

	// With Upstream, the tag is already upstream, so the mirror has nothing
	// left to publish.
	if opts.Remote != "" && !opts.Upstream {
		commit, err := o.Repo.Output("rev-parse", "--verify", "refs/tags/"+tag+"^{commit}")
		if err != nil {
			return err
		}
		if owner, name, ok := ghrun.ParseGitHubRepo(o.RemoteURL(opts.Remote)); ok {
			if err := watchRepo(ctx, owner+"/"+name, tag, commit, opts); err != nil {
				return fmt.Errorf("mirroring %s failed: %w", tag, err)
			}
		} else {
			_, _ = fmt.Fprintf(opts.Output, "%s isn't on GitHub; not watching the mirror.\n", opts.Remote)
		}
	}

	// The mirror's runs finishing doesn't guarantee the tag is upstream, e.g.
	// where none ran, so wait for it.
	deadline := opts.Watch.Now().Add(opts.Watch.Grace)
	var commit string
	for {
		var err error
		if commit, err = o.UpstreamTag(s, version); err != nil {
			return err
		}
		if commit != "" {
			break
		}
		if opts.Watch.Now().After(deadline) {
			return fmt.Errorf("%s's upstream has no tag %s; is the mirror action publishing tags?", s.Prefix, version)
		}
		if err := opts.Watch.Sleep(ctx, opts.Watch.Interval); err != nil {
			return err
		}
	}

	owner, name, ok := ghrun.ParseGitHubRepo(s.Remote)
	if !ok {
		_, _ = fmt.Fprintf(opts.Output, "%s's upstream isn't on GitHub; not watching it.\n", s.Prefix)
		return nil
	}
	if err := watchRepo(ctx, owner+"/"+name, version, commit, opts); err != nil {
		return fmt.Errorf("releasing %s failed: %w", version, err)
	}
	_, _ = fmt.Fprintf(opts.Output, "Published %s.\n", tag)
	return nil
}

// watchRepo follows the runs the push of tag at commit started in repo,
// reporting each as its state changes.
func watchRepo(ctx context.Context, repo, tag, commit string, opts WatchOptions) error {
	_, _ = fmt.Fprintf(opts.Output, "Watching %s's runs for %s...\n", repo, tag)
	seen := map[int64]bool{}
	runs, err := ghrun.Watch(ctx, opts.List, repo, tag, commit, opts.Watch, func(r ghrun.Run) {
		if !seen[r.ID] {
			seen[r.ID] = true
			_, _ = fmt.Fprintf(opts.Output, "  %s: %s (%s)\n", r.Workflow, r.State(), r.URL)
			return
		}
		_, _ = fmt.Fprintf(opts.Output, "  %s: %s\n", r.Workflow, r.State())
	})
	if err == nil && len(runs) == 0 {
		_, _ = fmt.Fprintf(opts.Output, "  No runs started for %s.\n", tag)
	}
	return err
}
