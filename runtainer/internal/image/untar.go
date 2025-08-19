package image

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func untarInto(r io.Reader, target string) error {
	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // done
		}
		if err != nil {
			return err
		}

		path := filepath.Join(target, hdr.Name)
		// Prevent directory traversal
		if !strings.HasPrefix(path, filepath.Clean(target)+string(os.PathSeparator)) {
			return os.ErrPermission
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, path); err != nil {
				return err
			}
		default:
			// ignore other types for now
		}
	}

	return nil
}
