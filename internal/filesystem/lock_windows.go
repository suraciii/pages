//go:build windows

package filesystem

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32Lock          = syscall.NewLazyDLL("kernel32.dll")
	lockFileExProcedure   = kernel32Lock.NewProc("LockFileEx")
	unlockFileExProcedure = kernel32Lock.NewProc("UnlockFileEx")
)

func lockOSFile(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	overlapped := &syscall.Overlapped{}
	result, _, callError := lockFileExProcedure.Call(file.Fd(), 0x00000002, 0, 1, 0, uintptr(unsafe.Pointer(overlapped)))
	if result == 0 {
		file.Close()
		return nil, fmt.Errorf("lock file: %w", callError)
	}
	return func() {
		_, _, _ = unlockFileExProcedure.Call(file.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(overlapped)))
		_ = file.Close()
	}, nil
}
