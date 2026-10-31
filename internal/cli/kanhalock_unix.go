//go:build !windows

package cli

import (
	"errors"
	"os"
	"syscall"
)

// tryLock takes an exclusive lock on fh without waiting. The OS drops it
// when fh closes or its process ends, so a check that dies holds nothing.
func tryLock(fh *os.File) (bool, error) {
	err := syscall.Flock(int(fh.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return false, nil
	}
	return err == nil, err
}

// noFollow keeps OpenFile from following a symlink at the path's last element.
const noFollow = syscall.O_NOFOLLOW

// privateDir reports whether dir is a real directory that only this user owns
// and can write: another user cannot plant records or a symlink in it.
func privateDir(dir string) bool {
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() || fi.Mode().Perm()&0o077 != 0 {
		return false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}
