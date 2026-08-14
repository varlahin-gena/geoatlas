//go:build windows

package datalock

import (
	"os"

	"golang.org/x/sys/windows"
)

func lockExclusive(f *os.File) error {
	var ol windows.Overlapped
	// LOCKFILE_EXCLUSIVE_LOCK | LOCKFILE_FAIL_IMMEDIATELY
	const flags = windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY
	return windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, &ol)
}

func unlock(f *os.File) error {
	var ol windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}
