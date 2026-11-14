package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// TempPath returns the temporary file used while replacing path.
func TempPath(path string) string {
	return filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".optiimg-tmp")
}

// ReplaceAtomic overwrites path with data via a temp file in the same
// directory: write, fsync, chmod, rename. On any error the temp file is
// removed and the original is left untouched.
func ReplaceAtomic(path string, data []byte, mode fs.FileMode, mtime time.Time, keepMtime bool) (err error) {
	tmp := TempPath(path)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp, mode); err != nil {
		return err
	}
	if keepMtime {
		if err = os.Chtimes(tmp, mtime, mtime); err != nil {
			return err
		}
	}
	attempts := 1
	if runtime.GOOS == "windows" {
		attempts = 3 // renaming over a file that is briefly open can fail
	}
	for i := 0; i < attempts; i++ {
		if err = os.Rename(tmp, path); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}
