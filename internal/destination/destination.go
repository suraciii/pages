// Package destination parses the target of a publish operation.
package destination

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Value is either a local path or an absolute HTTP(S) URL.
type Value struct {
	localPath string
	remoteURL *url.URL
}

// Parse resolves an empty value to the current directory and classifies every
// other value as a local OS path or remote HTTP(S) URL.
func Parse(raw string) (Value, error) {
	return ParseWithCurrentDirectory(raw, os.Getwd)
}

// ParseWithCurrentDirectory parses raw and uses currentDirectory only for an
// empty Destination.
func ParseWithCurrentDirectory(raw string, currentDirectory func() (string, error)) (Value, error) {
	if raw == "" {
		path, err := currentDirectory()
		if err != nil {
			return Value{}, fmt.Errorf("resolve current directory: %w", err)
		}
		return Value{localPath: path}, nil
	}

	if !hasURIScheme(raw) {
		return Value{localPath: raw}, nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return Value{}, fmt.Errorf("destination URL is invalid: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return Value{}, fmt.Errorf("destination URL must use HTTP or HTTPS")
	}
	if parsed.Host == "" {
		return Value{}, fmt.Errorf("destination URL must include a host")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return Value{}, fmt.Errorf("destination URL must not contain a query or fragment")
	}
	return Value{remoteURL: parsed}, nil
}

func hasURIScheme(value string) bool {
	scheme, _, found := strings.Cut(value, "://")
	if !found || scheme == "" {
		return false
	}
	for index, character := range scheme {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' {
			continue
		}
		if index > 0 && (character >= '0' && character <= '9' || character == '+' || character == '-' || character == '.') {
			continue
		}
		return false
	}
	return true
}

// IsRemote reports whether the Destination is an HTTP(S) URL.
func (value Value) IsRemote() bool {
	return value.remoteURL != nil
}

// LocalPath returns the local path, or an empty string for a remote
// Destination.
func (value Value) LocalPath() string {
	return value.localPath
}

// RemoteURL returns a copy of the remote URL, or nil for a local Destination.
func (value Value) RemoteURL() *url.URL {
	if value.remoteURL == nil {
		return nil
	}
	copy := *value.remoteURL
	return &copy
}

// String returns the Destination in its input form.
func (value Value) String() string {
	if value.remoteURL != nil {
		return value.remoteURL.String()
	}
	return value.localPath
}
