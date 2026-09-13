package executor

import (
	"errors"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/store"
)

// WriteArtifact uses the standard-library descriptor-backed Root API. The
// configured root inode is checked before use; all subsequent filesystem
// operations remain confined even if an intermediate path becomes a symlink.
func WriteArtifact(root, relative, content string) error {
	if _, e := store.SafeArtifactPath(root, relative); e != nil {
		return e
	}
	if e := os.MkdirAll(root, 0700); e != nil {
		return e
	}
	before, e := os.Lstat(root)
	if e != nil {
		return e
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return errors.New("ROOT_PATH_DENIED")
	}
	confined, e := os.OpenRoot(root)
	if e != nil {
		return e
	}
	defer confined.Close()
	opened, e := confined.Stat(".")
	if e != nil {
		return e
	}
	if !os.SameFile(before, opened) {
		return errors.New("ROOT_CHANGED")
	}
	clean := filepath.Clean(relative)
	dir := filepath.Dir(clean)
	if e = confined.MkdirAll(dir, 0700); e != nil {
		return e
	}
	tmp := filepath.Join(dir, ".artifact-"+contract.NewID())
	f, e := confined.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	defer confined.Remove(tmp)
	_, e = f.WriteString(content)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	if e = confined.Rename(tmp, clean); e != nil {
		return e
	}
	parent, e := confined.Open(dir)
	if e != nil {
		return e
	}
	defer parent.Close()
	return parent.Sync()
}
