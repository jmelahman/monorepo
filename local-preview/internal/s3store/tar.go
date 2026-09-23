package s3store

import (
	"archive/tar"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// tarDir writes srcDir's contents to w as a tar stream, with entry names
// relative to srcDir (so extraction into a fresh dir reproduces the tree
// directly). It returns the total uncompressed byte size of regular-file
// contents and the number of regular files — the integrity metadata Save
// records. Regular files, directories, and symlinks are archived; any other
// file type is an error. A source path that vanishes mid-walk (retention's GC
// racing a fresh build) returns ErrSourceGone.
func tarDir(w io.Writer, srcDir string) (size, count int64, err error) {
	root, err := filepath.Abs(srcDir)
	if err != nil {
		return 0, 0, err
	}
	tw := tar.NewWriter(w)
	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return ErrSourceGone
			}
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		name := filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			if os.IsNotExist(err) {
				return ErrSourceGone
			}
			return err
		}
		switch {
		case d.IsDir():
			return tw.WriteHeader(&tar.Header{
				Name:     name + "/",
				Mode:     int64(info.Mode().Perm()),
				Typeflag: tar.TypeDir,
			})
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				if os.IsNotExist(err) {
					return ErrSourceGone
				}
				return err
			}
			return tw.WriteHeader(&tar.Header{
				Name:     name,
				Mode:     int64(info.Mode().Perm()),
				Typeflag: tar.TypeSymlink,
				Linkname: link,
			})
		case info.Mode().IsRegular():
			if err := tw.WriteHeader(&tar.Header{
				Name:     name,
				Mode:     int64(info.Mode().Perm()),
				Size:     info.Size(),
				Typeflag: tar.TypeReg,
			}); err != nil {
				return err
			}
			f, err := os.Open(p)
			if err != nil {
				if os.IsNotExist(err) {
					return ErrSourceGone
				}
				return err
			}
			n, err := io.Copy(tw, f)
			f.Close()
			if err != nil {
				return err
			}
			size += n
			count++
			return nil
		default:
			return fmt.Errorf("unsupported file type %s in artifact: %s", info.Mode(), p)
		}
	})
	if walkErr != nil {
		return 0, 0, walkErr
	}
	if err := tw.Close(); err != nil {
		return 0, 0, err
	}
	return size, count, nil
}
