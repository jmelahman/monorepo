package image

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"github.com/jmelahman/porter/internal/paths"
)

func ExtractImage(tarPath string) (string, error) {
	file, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var tarReader *tar.Reader
	if filepath.Ext(tarPath) == ".gz" {
		gzr, err := gzip.NewReader(file)
		if err != nil {
			return "", err
		}
		defer gzr.Close()
		tarReader = tar.NewReader(gzr)
	} else {
		tarReader = tar.NewReader(file)
	}

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
	if filepath.Ext(tarPath) == ".gz" {
		gzr, _ := gzip.NewReader(file)
		defer gzr.Close()
		tarReader = tar.NewReader(gzr)
	} else {
		tarReader = tar.NewReader(file)
	}

	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		target := filepath.Join(destDir, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", err
			}
			out, err := os.Create(target)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(out, tarReader); err != nil {
				out.Close()
				return "", err
			}
			out.Close()
		}
	}

	// TODO: flatten to rootfsDir/imageID using layer tarballs
	// e.g., apply layer blobs in manifest to create a rootfs

	return imageID, nil
}
