//go:build aix || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris

package filesystem

import (
	"fmt"
	"os"
	"syscall"
)

func lockOSFile(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return nil, err
	}
	lock := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: 0, Len: 1}
	for {
		err = syscall.FcntlFlock(file.Fd(), syscall.F_SETLKW, &lock)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("lock file: %w", err)
		}
		break
	}
	return func() {
		lock.Type = syscall.F_UNLCK
		_ = syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock)
		_ = file.Close()
	}, nil
}
