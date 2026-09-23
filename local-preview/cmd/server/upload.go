package server

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmelahman/local-preview/internal/client"
)

func uploadCmd() *cobra.Command {
	var serverURL, repoFlag, oidcAudience string
	var overwrite, deploy, oidc bool

	parent := &cobra.Command{
		Use:   "upload",
		Short: "Upload a CI-built frontend, backend, or artifact for a commit",
		Long: "Publish prebuilt outputs into the content-addressed store so a deploy of\n" +
			"the commit serves them without rebuilding. The server resolves the ref\n" +
			"and computes the same hash a build would target, then lands the upload in\n" +
			"that slot. Each <path> may be a directory (its contents are tarred) or an\n" +
			"existing .tar/.tar.gz file. The ref defaults to HEAD of the current repo.\n\n" +
			"When the server requires GitHub Actions OIDC, pass --oidc to authenticate\n" +
			"with a token minted by the runner (needs `permissions: id-token: write`).",
	}
	addServerFlag(parent, &serverURL)
	parent.PersistentFlags().StringVar(&repoFlag, "repo", "", "Registered repo name (default: auto-detect from cwd)")
	parent.PersistentFlags().BoolVar(&overwrite, "overwrite", false, "Replace the artifact if it is already present")
	parent.PersistentFlags().BoolVar(&deploy, "deploy", false, "Deploy the commit after uploading and print its preview URL")
	parent.PersistentFlags().BoolVar(&oidc, "oidc", false, "Authenticate with a GitHub Actions OIDC token fetched from the runner")
	parent.PersistentFlags().StringVar(&oidcAudience, "oidc-audience", "", "Audience to request for the OIDC token (default: $PREVIEW_GITHUB_OIDC_AUDIENCE, else the server URL)")

	run := func(cmd *cobra.Command, side, name, path, ref string) error {
		url, err := resolveURL(cmd, serverURL)
		if err != nil {
			return err
		}
		token, err := uploadToken(cmd.Context(), oidc, oidcAudience, url)
		if err != nil {
			return err
		}
		return runUpload(cmd.Context(), url, cmd.OutOrStdout(),
			repoFlag, side, name, path, ref, overwrite, deploy, token)
	}

	frontend := &cobra.Command{
		Use:   "frontend <path> [ref]",
		Short: "Upload a prebuilt frontend bundle (a dist directory or tarball)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, "frontend", "", args[0], argRef(args, 1))
		},
	}
	backend := &cobra.Command{
		Use:   "backend <path> [ref]",
		Short: "Upload a prebuilt backend tree (a built directory or tarball)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, "backend", "", args[0], argRef(args, 1))
		},
	}
	artifact := &cobra.Command{
		Use:   "artifact <name> <path> [ref]",
		Short: "Upload a named downloadable artifact's files",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, "artifact", args[0], args[1], argRef(args, 2))
		},
	}
	parent.AddCommand(frontend, backend, artifact)
	return parent
}

// argRef returns the ref positional at index i, or "" when it was omitted.
func argRef(args []string, i int) string {
	if len(args) > i {
		return args[i]
	}
	return ""
}

func runUpload(ctx context.Context, url string, out io.Writer, repoName, side, name, path, ref string, overwrite, deploy bool, token string) error {
	c := client.New(url, nil)
	if token != "" {
		c.SetToken(token)
	}

	if ref == "" {
		sha, err := localHeadSHA(".")
		if err != nil {
			return fmt.Errorf("no ref given and no commit found in the current directory: %w", err)
		}
		ref = sha
	}
	if repoName == "" {
		var err error
		if repoName, err = detectRepo(ctx, c); err != nil {
			return err
		}
	}

	body, err := openUploadBody(path)
	if err != nil {
		return err
	}
	defer body.Close()

	res, err := c.Upload(ctx, repoName, side, name, ref, overwrite, body)
	if err != nil {
		return err
	}

	target := res.Side
	if res.Name != "" {
		target += " " + res.Name
	}
	if res.Published {
		fmt.Fprintf(out, "uploaded %s for %s@%s (hash %s)\n", target, repoName, res.ShortSHA, res.Hash)
	} else {
		fmt.Fprintf(out, "%s for %s@%s already present (hash %s); pass --overwrite to replace\n",
			target, repoName, res.ShortSHA, res.Hash)
	}
	for _, f := range res.Files {
		fmt.Fprintf(out, "  %s (%s)\n", f.Name, formatBytes(uint64(f.Size)))
	}

	if deploy {
		return triggerDeploy(ctx, c, out, repoName, ref)
	}
	return nil
}

// triggerDeploy deploys the just-uploaded commit and waits for it to settle —
// the one-command CI flow behind `preview upload --deploy`.
func triggerDeploy(ctx context.Context, c *client.Client, out io.Writer, repoName, ref string) error {
	d, err := c.CreateDeploy(ctx, repoName, ref, false)
	if err != nil {
		return err
	}
	for d.Status == "queued" || d.Status == "building" {
		time.Sleep(500 * time.Millisecond)
		if d, err = c.GetDeploy(ctx, d.ID); err != nil {
			return err
		}
	}
	if d.Status == "ready" {
		fmt.Fprintf(out, "ready: %s\n", d.PreviewURL)
		return nil
	}
	fmt.Fprintf(out, "deploy %d %s: %s\n", d.ID, d.Status, d.Error)
	fmt.Fprintf(out, "full logs: preview deploy logs %d\n", d.ID)
	return fmt.Errorf("deploy did not become ready")
}

// openUploadBody streams the upload body: a directory is tarred (its contents,
// no wrapper dir, so the tar root maps to the published root); a regular file
// (a prebuilt .tar/.tar.gz) is streamed as-is.
func openUploadBody(path string) (io.ReadCloser, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return tarGzDir(path), nil
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is neither a directory nor a regular file", path)
	}
	return os.Open(path)
}

// tarGzDir returns a reader over a gzip-compressed tar of dir's contents,
// produced concurrently so nothing is buffered whole in memory. Entry names are
// relative to dir (no wrapping directory).
func tarGzDir(dir string) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		gz := gzip.NewWriter(pw)
		tw := tar.NewWriter(gz)
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			link := ""
			if info.Mode()&os.ModeSymlink != 0 {
				if link, err = os.Readlink(p); err != nil {
					return err
				}
			}
			hdr, err := tar.FileInfoHeader(info, link)
			if err != nil {
				return err
			}
			hdr.Name = filepath.ToSlash(rel)
			if d.IsDir() {
				hdr.Name += "/"
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			f, err := os.Open(p)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(tw, f)
			return err
		})
		if err == nil {
			if cerr := tw.Close(); cerr != nil {
				err = cerr
			} else {
				err = gz.Close()
			}
		}
		pw.CloseWithError(err)
	}()
	return pr
}
