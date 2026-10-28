//go:build windows

package cli

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// tryLock takes an exclusive lock on fh without waiting. The OS drops it
// when fh closes or its process ends, so a check that dies holds nothing.
func tryLock(fh *os.File) (bool, error) {
	err := windows.LockFileEx(windows.Handle(fh.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, new(windows.Overlapped))
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return false, nil
	}
	return err == nil, err
}
