package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	getter "github.com/hashicorp/go-getter"
)

// ErrDigestMismatch is returned by [Fetch] when a download does not hash to
// the digest its [Spec] pins.
var ErrDigestMismatch = errors.New("downloaded file does not match its pinned SHA-256")

// Spec pins a GGUF file on the Hugging Face Hub. Revision is a commit, not a
// branch, so the URL always names the same bytes, and SHA256 is checked
// before the file is used.
type Spec struct {
	Repo     string // e.g. "ggml-org/Qwen3-0.6B-GGUF"
	Revision string // commit hash
	File     string // path within the repository
	SHA256   string // hex digest of the file
	// Endpoint is the Hub to download from, "https://huggingface.co" if
	// empty; a mirror serves the same paths.
	Endpoint string
	Size     int64 // bytes
}

// URL is where the file is downloaded from.
func (s Spec) URL() string {
	endpoint := s.Endpoint
	if endpoint == "" {
		endpoint = "https://huggingface.co"
	}
	return endpoint + "/" + s.Repo + "/resolve/" + s.Revision + "/" + s.File
}

// Path is where [Fetch] keeps the file under cache.
func (s Spec) Path(cache string) string {
	return filepath.Join(cache, "models", s.Repo, s.Revision, s.File)
}

// Fetch downloads the file s pins into cache and returns its path. A file is
// moved into place only once its digest has been checked, so one that is
// already there, at the pinned size, is used without hashing it again.
func Fetch(ctx context.Context, s Spec, cache string, progress io.Writer) (string, error) {
	dest := s.Path(cache)
	if fi, err := os.Stat(dest); err == nil && fi.Size() == s.Size {
		return dest, nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL(), nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch %s: %s", s.URL(), resp.Status)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".fetch-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	// A body past the pinned size can only fail the digest check, so stop
	// one byte after it rather than fill the disk.
	body := tracker{progress}.TrackProgress(s.File, 0, s.Size, resp.Body)
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(body, s.Size+1))
	err = errors.Join(err, body.Close(), tmp.Close())
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", s.URL(), err)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != s.SHA256 {
		return "", fmt.Errorf("%w: %s is %s, want %s", ErrDigestMismatch, s.File, got, s.SHA256)
	}
	return dest, os.Rename(tmp.Name(), dest)
}

// tracker reports download progress, for both [Fetch] and yzma's library
// installer: as a single rewritten line on a terminal, and otherwise as one
// line per download, once it ends. A nil writer reports nothing.
type tracker struct {
	w io.Writer
}

var _ getter.ProgressTracker = tracker{}

func (t tracker) TrackProgress(src string, current, total int64, stream io.ReadCloser) io.ReadCloser {
	if t.w == nil {
		return stream
	}
	return &progressReader{ReadCloser: stream, w: t.w, name: filepath.Base(src), done: current, total: total, tty: isTerminal(t.w)}
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

type progressReader struct {
	io.ReadCloser
	w     io.Writer
	last  time.Time
	name  string
	done  int64
	total int64
	tty   bool
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.done += int64(n)
	if now := time.Now(); r.tty && (now.Sub(r.last) > 200*time.Millisecond || errors.Is(err, io.EOF)) {
		r.last = now
		r.report()
	}
	return n, err
}

func (r *progressReader) Close() error {
	r.report()
	_, _ = fmt.Fprintln(r.w)
	return r.ReadCloser.Close()
}

func (r *progressReader) report() {
	const mib = 1 << 20
	line := fmt.Sprintf("%s: %d MiB", r.name, r.done/mib)
	if r.total > 0 {
		line = fmt.Sprintf("%s: %d/%d MiB (%.0f%%)", r.name, r.done/mib, r.total/mib, 100*float64(r.done)/float64(r.total))
	}
	if r.tty {
		line = "\r" + line + "\033[K"
	}
	_, _ = io.WriteString(r.w, line)
}
