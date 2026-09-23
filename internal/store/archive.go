package store

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrArchiveTooLarge marks an extraction aborted because the archive's
// cumulative decompressed size exceeded the cap passed to ExtractTar — the
// guard against a gzip bomb or an unbounded upload filling the disk. Callers
// surface it as 413 (upload) / a hydrate failure.
var ErrArchiveTooLarge = errors.New("archive exceeds decompression cap")

// ExtractTar extracts a tar stream — gzip-compressed or raw — into destDir. It
// is hardened against archives crafted to escape destDir: entry names that are
// absolute or contain ".." are rejected, and symlink/hardlink targets that
// resolve outside destDir are refused. Only regular files, directories, and
// in-tree symlinks/hardlinks are extracted; other entry types are skipped.
//
// maxBytes caps the total decompressed size written across all entries: once
// the running total would exceed it, extraction aborts with ErrArchiveTooLarge
// rather than decompressing an unbounded (or bomb) stream to disk. A value <= 0
// means no cap. This bounds decompressed output regardless of the compressed
// wire size, so it also protects the durable-tier hydrate path.
//
// It backs both CI uploads (internal/build) and durable-tier hydration, so it
// lives here in store — the one package both callers already import — and is
// the single hardened extractor neither may duplicate.
// Both extractors return the number of regular-file content bytes written —
// the same quantity the durable tier records at Save time, so hydration can
// verify integrity by comparing like with like.
func ExtractTar(r io.Reader, destDir string, maxBytes int64) (int64, error) {
	return extractTar(r, destDir, maxBytes, false)
}

// ExtractTarPayload is ExtractTar with the symlink-target restriction lifted:
// symlinks are recreated verbatim, wherever they point. It exists for exactly
// one class of tree — a *backend artifact*, which is an executed payload, not
// served data. A backend venv legitimately carries absolute symlinks that
// resolve inside its run container (onyx: .venv/bin/python → the devcontainer's
// /opt/uv/...), so the strict rule would refuse every hydrate and CI upload of
// it.
//
// Why this does not reopen the escaping-symlink hole the strict rule closed
// (see REGRESSIONS.md): the danger of a hostile symlink is a *trusted host-side
// reader following it* — the public frontend file server and the artifact
// publisher, both of which serve only frontend/artifact trees, which stay
// strict. Nothing on the host follows a backend tree's links: run containers
// resolve them in their own filesystem, the persist tar-writer Readlinks
// without following, eviction removes without following, and a host-run
// preview is the repo's own code executing anyway. Entry *names* are still
// confined to destDir, and hardlinks stay strict on every path — os.Link
// resolves host-side at extract time, so an out-of-root hardlink would BE the
// host file.
func ExtractTarPayload(r io.Reader, destDir string, maxBytes int64) (int64, error) {
	return extractTar(r, destDir, maxBytes, true)
}

func extractTar(r io.Reader, destDir string, maxBytes int64, payloadLinks bool) (int64, error) {
	br := bufio.NewReader(r)
	// Sniff the gzip magic so raw .tar and .tar.gz both work.
	if magic, err := br.Peek(2); err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return 0, fmt.Errorf("gzip: %w", err)
		}
		defer gz.Close()
		return extractTarStream(gz, destDir, maxBytes, payloadLinks)
	}
	return extractTarStream(br, destDir, maxBytes, payloadLinks)
}

func extractTarStream(r io.Reader, destDir string, maxBytes int64, payloadLinks bool) (int64, error) {
	root, err := filepath.Abs(destDir)
	if err != nil {
		return 0, err
	}
	tr := tar.NewReader(r)
	seen := false
	var total int64
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return total, err
		}
		target, err := safeJoin(root, hdr.Name)
		if err != nil {
			return total, err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return total, err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return total, err
			}
			n, err := writeFileFrom(tr, target, hdr.FileInfo().Mode().Perm(), maxBytes, total)
			total += n
			if err != nil {
				return total, err
			}
			seen = true
		case tar.TypeSymlink, tar.TypeLink:
			// Symlink targets are relative to the link's own directory;
			// hardlink targets are archive paths relative to root. Either way
			// the resolved target must stay inside destDir — and an absolute
			// target always escapes, so reject it before it becomes a symlink
			// literally pointing outside the tree. The one exception is a
			// payload tree's symlinks (never a hardlink) — see
			// ExtractTarPayload for the argument.
			payloadSymlink := payloadLinks && hdr.Typeflag == tar.TypeSymlink
			if !payloadSymlink {
				if filepath.IsAbs(hdr.Linkname) || strings.HasPrefix(hdr.Linkname, "/") {
					return total, fmt.Errorf("absolute link target %q in archive", hdr.Name)
				}
			}
			var resolved string
			if hdr.Typeflag == tar.TypeSymlink {
				resolved = filepath.Join(filepath.Dir(target), filepath.FromSlash(hdr.Linkname))
			} else if resolved, err = safeJoin(root, hdr.Linkname); err != nil {
				return total, err
			}
			if !payloadSymlink && !withinRoot(root, resolved) {
				return total, fmt.Errorf("unsafe link target %q in archive", hdr.Name)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return total, err
			}
			os.Remove(target) // tolerate a re-extraction over an existing entry
			if hdr.Typeflag == tar.TypeSymlink {
				err = os.Symlink(hdr.Linkname, target)
			} else {
				err = os.Link(resolved, target)
			}
			if err != nil {
				return total, err
			}
			seen = true
		default:
			// Skip devices, fifos, and other unsupported entry types.
		}
	}
	if !seen {
		return total, fmt.Errorf("archive contained no files")
	}
	return total, nil
}

// safeJoin joins a tar entry name onto root, rejecting absolute paths and ".."
// escapes so a crafted archive can't write outside the extraction root.
func safeJoin(root, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty path in archive")
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("absolute path %q in archive", name)
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes the archive root", name)
	}
	joined := filepath.Join(root, clean)
	if !withinRoot(root, joined) {
		return "", fmt.Errorf("unsafe path %q in archive", name)
	}
	return joined, nil
}

func withinRoot(root, p string) bool {
	return p == root || strings.HasPrefix(p, root+string(os.PathSeparator))
}

// writeFileFrom writes r into dst and returns the number of bytes written. When
// maxBytes > 0 it bounds the archive's cumulative decompressed size: it copies
// at most one byte past the remaining budget (maxBytes-written) and, if that
// byte materializes, removes the partial file and returns ErrArchiveTooLarge —
// so a bomb neither streams to completion nor leaves the over-cap bytes behind.
func writeFileFrom(r io.Reader, dst string, perm os.FileMode, maxBytes, written int64) (int64, error) {
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return 0, err
	}
	src := r
	if maxBytes > 0 {
		// remaining+1 lets an exact-fit archive pass but trips on the first
		// over-cap byte.
		src = io.LimitReader(r, maxBytes-written+1)
	}
	n, err := io.Copy(f, src)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return n, err
	}
	if maxBytes > 0 && written+n > maxBytes {
		os.Remove(dst)
		return n, fmt.Errorf("%w of %d bytes", ErrArchiveTooLarge, maxBytes)
	}
	return n, nil
}
