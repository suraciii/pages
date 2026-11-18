package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/suraciii/pages/internal/filesystem"
)

// validateListenAddress accepts one host:port address. The boolean reports
// whether the host is a loopback address.
func validateListenAddress(address string) (bool, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return false, fmt.Errorf("listen address %q must be one host:port address", address)
	}
	if port == "" {
		return false, fmt.Errorf("listen address %q must include a port", address)
	}
	for index := range len(port) {
		if port[index] < '0' || port[index] > '9' {
			return false, fmt.Errorf("listen port %q must be a decimal number from 1 to 65535", port)
		}
	}
	number, err := strconv.ParseUint(port, 10, 16)
	if err != nil || number == 0 {
		return false, fmt.Errorf("listen port %q must be a decimal number from 1 to 65535", port)
	}
	if strings.ContainsAny(host, " \t\r\n/\\") {
		return false, fmt.Errorf("listen host %q is invalid", host)
	}
	if host == "" {
		return false, nil
	}
	if strings.EqualFold(host, "localhost") {
		return true, nil
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback(), nil
	}
	return false, nil
}

func validateContentSecurityPolicy(value string) error {
	if value == "" {
		return errors.New("content security policy must not be empty")
	}
	if strings.ContainsAny(value, "\r\n") {
		return errors.New("content security policy must be one line")
	}
	return nil
}

// resolveAbsolutePath returns an absolute cleaned path with the longest
// existing prefix resolved through symbolic links.
func resolveAbsolutePath(fileSystem filesystem.FS, path string) (string, error) {
	absolute, err := fileSystem.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path %s: %w", path, err)
	}
	resolved, err := resolveExistingPrefix(fileSystem, filepath.Clean(absolute))
	if err != nil {
		return "", fmt.Errorf("resolve path %s: %w", path, err)
	}
	return resolved, nil
}

func resolveExistingPrefix(fileSystem filesystem.FS, path string) (string, error) {
	missing := make([]string, 0)
	current := path
	for {
		resolved, err := fileSystem.EvalSymlinks(current)
		if err == nil {
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

// pathWithin reports whether target is root or is below root.
func pathWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if relative == "." {
		return true
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
