//go:build aix || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris

package pages

import (
	"os"
	"syscall"
)

func lockFile(file *os.File) (func() error, error) {
	lock := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: 0, Len: 1}
	for {
		err := syscall.FcntlFlock(file.Fd(), syscall.F_SETLKW, &lock)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return nil, err
		}
		break
	}
	return func() error {
		lock.Type = syscall.F_UNLCK
		return syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock)
	}, nil
}
