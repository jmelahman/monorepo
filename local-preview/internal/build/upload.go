package build

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/store"
)

// Upload sides — which content-addressed slot a prebuilt upload primes.
const (
	SideFrontend = "frontend"
	SideBackend  = "backend"
	SideArtifact = "artifact"
)

// ErrNoSuchArtifact marks an upload targeting a name the deployed commit's
// manifest doesn't declare as an artifact; the API maps it to 404.
var ErrNoSuchArtifact = errors.New("no such artifact")

// UploadResult reports what an upload published (or would have): the resolved
// commit, the content-address it targeted, and whether bytes actually landed
// (false when the artifact was already present and overwrite wasn't set).
type UploadResult struct {
	SHA       string       `json:"sha"`
	ShortSHA  string       `json:"short_sha"`
	Side      string       `json:"side"`
	Name      string       `json:"name,omitempty"`
	Hash      string       `json:"hash"`
	Published bool         `json:"published"`
	Files     []UploadFile `json:"files,omitempty"`
}

// UploadFile is one published file of an uploaded downloadable artifact.
type UploadFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// Upload publishes a CI-built side (frontend bundle, backend tree, or a named
// downloadable artifact) into the content-addressed store for a commit, so a
// deploy of that commit — or any commit sharing the hash — serves it without
// building. The server is the authority on the hash: it resolves ref → sha,
// reads the manifest at that commit, and computes the side's content-address
// exactly like a build (resolveHashes), then lands the uploaded tar in the same
// slot buildDeploy would target. body is a tar stream, optionally gzip-compressed.
//
// It never touches deploy rows: an upload is a store-priming operation, ordered
// independently of the deploy that eventually references it. Already-present and
// not overwrite is a no-op (Published=false).
func (q *Queue) Upload(ctx context.Context, repoName, ref, side, name string, body io.Reader, overwrite bool) (UploadResult, error) {
	if side == SideArtifact && name == "" {
		return UploadResult{}, fmt.Errorf("an artifact name is required")
	}
	_, gr, sha, err := q.resolveRepoRef(ctx, repoName, ref)
	if err != nil {
		return UploadResult{}, err
	}
	row := db.DeployRow{Deploy: db.Deploy{SHA: sha, ShortSHA: shortSHA(sha)}, RepoName: repoName}
	m, err := q.loadManifest(ctx, gr, row)
	if err != nil {
		return UploadResult{}, err
	}
	// Only the uploaded side is hashed — an upload of one side must not depend
	// on the others' partitions being valid.
	env, entries, err := q.hashInputs(ctx, gr, sha, m)
	if err != nil {
		return UploadResult{}, err
	}

	res := UploadResult{SHA: sha, ShortSHA: row.ShortSHA, Side: side, Name: name}
	switch side {
	case SideFrontend:
		if res.Hash, err = feHashOf(m.Frontend, env, entries); err != nil {
			return UploadResult{}, err
		}
		if !overwrite && q.files.HasFrontend(repoName, res.Hash) {
			return res, nil
		}
		dir, cleanup, err := q.stageUpload(body)
		if err != nil {
			return UploadResult{}, err
		}
		defer cleanup()
		if err := q.files.PublishFrontend(repoName, res.Hash, dir, overwrite); err != nil {
			return UploadResult{}, err
		}
		res.Published = true
		// Persist to the durable tier now, exactly like a build's publish —
		// a worker serving this deploy hydrates from the tier, and waiting
		// for the next reconcile pass would 502 every cold start until then.
		q.enqueuePersist(repoName, "fe", res.Hash)
	case SideBackend:
		if res.Hash, err = beHashOf(m, env, entries); err != nil {
			return UploadResult{}, err
		}
		if !overwrite && q.files.HasBackend(repoName, res.Hash) {
			return res, nil
		}
		// Payload symlink policy: a backend venv legitimately links to paths
		// that only resolve inside its run container (see ExtractTarPayload).
		dir, cleanup, err := q.stagePayloadUpload(body)
		if err != nil {
			return UploadResult{}, err
		}
		defer cleanup()
		if err := q.files.PublishBackend(repoName, res.Hash, dir, overwrite); err != nil {
			return UploadResult{}, err
		}
		res.Published = true
		q.enqueuePersist(repoName, "be", res.Hash)
	case SideArtifact:
		spec, ok := m.Artifacts[name]
		if !ok {
			return UploadResult{}, fmt.Errorf("%w %q at %s", ErrNoSuchArtifact, name, res.ShortSHA)
		}
		if res.Hash, err = artHashOf(m, spec, env, entries); err != nil {
			return UploadResult{}, err
		}
		if !overwrite && q.files.HasArtifact(repoName, res.Hash) {
			res.Files = toUploadFiles(q.files.ListArtifactFiles(repoName, res.Hash))
			return res, nil
		}
		dir, cleanup, err := q.stageUpload(body)
		if err != nil {
			return UploadResult{}, err
		}
		defer cleanup()
		// Reuses the build's publish: each declared file is copied out flat by
		// base name, and a missing one fails the upload ("was not produced by
		// the build") — so the tar must carry the declared files at their
		// path-relative locations, exactly as the build produces them.
		if err := q.files.PublishArtifactFiles(repoName, res.Hash, dir, spec.Files, overwrite); err != nil {
			return UploadResult{}, err
		}
		res.Published = true
		res.Files = toUploadFiles(q.files.ListArtifactFiles(repoName, res.Hash))
		q.enqueuePersist(repoName, "dl", res.Hash)
	default:
		return UploadResult{}, fmt.Errorf("unknown upload side %q", side)
	}
	return res, nil
}

// stageUpload extracts the uploaded tar(.gz) into a fresh scratch dir whose
// contents become the published artifact (frontend/backend) or the tree the
// declared files are copied out of (artifact). The cleanup is safe after a
// successful publish, whose rename moves the dir out from under it.
func (q *Queue) stageUpload(body io.Reader) (string, func(), error) {
	return q.stage(body, q.files.ExtractTar)
}

// stagePayloadUpload is stageUpload under the payload symlink policy —
// backend uploads only.
func (q *Queue) stagePayloadUpload(body io.Reader) (string, func(), error) {
	return q.stage(body, q.files.ExtractTarPayload)
}

func (q *Queue) stage(body io.Reader, extract func(io.Reader, string) (int64, error)) (string, func(), error) {
	dir, cleanup, err := q.files.NewScratchDir("upload")
	if err != nil {
		return "", nil, err
	}
	if _, err := extract(body, dir); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("extract upload: %w", err)
	}
	return dir, cleanup, nil
}

func toUploadFiles(fs []store.ArtifactFile) []UploadFile {
	out := make([]UploadFile, len(fs))
	for i, f := range fs {
		out[i] = UploadFile{Name: f.Name, Size: f.Size}
	}
	return out
}

func shortSHA(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}
