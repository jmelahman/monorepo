package image

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"github.com/jmelahman/runtainer/internal/paths"
)

func ExtractImage(tarPath string) (string, error) {
	file, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()

	// Hash image tarball contents to get a stable ID
	h := sha256.New()
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	imageID := hex.EncodeToString(h.Sum(nil))[:12]

	destDir := filepath.Join(paths.ImageDir(), imageID)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	// Rewind again to extract
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	var extractReader *tar.Reader
	if filepath.Ext(tarPath) == ".gz" {
		gzr, err := gzip.NewReader(file)
		if err != nil {
			return "", err
		}
		defer func() {
			_ = gzr.Close()
		}()
		extractReader = tar.NewReader(gzr)
	} else {
		extractReader = tar.NewReader(file)
	}

	for {
		hdr, err := extractReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		target := filepath.Join(destDir, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", err
			}
			out, err := os.Create(target)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(out, extractReader); err != nil {
				_ = out.Close()
				return "", err
			}
			if err := out.Close(); err != nil {
				return "", err
			}
		}
	}

	// TODO: flatten to rootfsDir/imageID using layer tarballs
	// e.g., apply layer blobs in manifest to create a rootfs

	return imageID, nil
}
