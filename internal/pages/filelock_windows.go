//go:build windows

package pages

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

const lockFileExclusiveLock = 0x2

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	lockFileExProc   = kernel32.NewProc("LockFileEx")
	unlockFileExProc = kernel32.NewProc("UnlockFileEx")
)

func lockFile(file *os.File) (func() error, error) {
	overlapped := &syscall.Overlapped{}
	result, _, callErr := lockFileExProc.Call(
		file.Fd(), lockFileExclusiveLock, 0, 1, 0,
		uintptr(unsafe.Pointer(overlapped)),
	)
	runtime.KeepAlive(file)
	if result == 0 {
		return nil, callErr
	}
	return func() error {
		result, _, callErr := unlockFileExProc.Call(
			file.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(overlapped)),
		)
		runtime.KeepAlive(file)
		if result == 0 {
			return callErr
		}
		return nil
	}, nil
}
