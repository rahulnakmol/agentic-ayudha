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
