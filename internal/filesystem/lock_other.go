//go:build plan9 || js || wasip1

package filesystem

import (
	"os"
)

func lockOSFile(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	return func() { _ = file.Close() }, nil
}
