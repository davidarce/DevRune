package backup

import (
	"io/fs"
	"os"
)

// WriteFileAtomic writes data to path atomically via a .tmp sibling + os.Rename.
// It creates path+".tmp", writes, syncs, closes, and renames to path.
// Both files reside in the same directory, guaranteeing same-filesystem rename (POSIX atomic).
// The .tmp file is removed on any error path to avoid leaving garbage on disk.
func WriteFileAtomic(path string, data []byte, perm fs.FileMode) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
