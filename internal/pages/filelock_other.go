//go:build plan9 || js || wasip1

package pages

import (
	"fmt"
	"os"
)

func lockFile(_ *os.File) (func() error, error) {
	return nil, fmt.Errorf("token file locking is not supported on this platform")
}
