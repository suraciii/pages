// Package publish resolves the publish inputs and runs the publish.
package publish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/suraciii/pages/internal/client"
	"github.com/suraciii/pages/internal/destination"
	"github.com/suraciii/pages/internal/filesystem"
	"github.com/suraciii/pages/internal/pages"
)

// ErrUsage marks a resolved value that fails validation. Callers report it
// as a usage error.
var ErrUsage = errors.New("usage")

// Runtime supplies process resources to Resolve and Run.
type Runtime struct {
	FileSystem       filesystem.FS
	Environment      func(string) string
	CurrentDirectory func() (string, error)
	HTTPClient       interface {
		Do(*http.Request) (*http.Response, error)
	}
}

func productionRuntime() Runtime {
	return Runtime{
		FileSystem:       filesystem.OS,
		Environment:      os.Getenv,
		CurrentDirectory: os.Getwd,
	}
}

// Config is the publish config file. Every field is optional.
type Config struct {
	Destination string            `json:"destination"`
	Token       string            `json:"token"`
	Identities  map[string]string `json:"identities"`
	Identity    string            `json:"identity"`
}

// LoadConfig reads a config file. An empty path means the default file:
// a missing default file is fine. A named file must exist.
func LoadConfig(path string) (*Config, error) {
	return loadConfig(filesystem.OS, path)
}

func loadConfig(fileSystem filesystem.FS, path string) (*Config, error) {
	config := &Config{}
	if path == "" {
		return config, nil
	}
	file, err := fileSystem.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("parse config file: trailing data")
	}
	if config.Token != "" && !pages.ValidSecret(config.Token) {
		return nil, fmt.Errorf("default token secret must use base64url characters")
	}
	for identity, secret := range config.Identities {
		if !pages.ValidName(identity) || !pages.ValidSecret(secret) {
			return nil, fmt.Errorf("invalid token entry for identity %q", identity)
		}
	}
	if config.Identity != "" && !pages.ValidName(config.Identity) {
		return nil, fmt.Errorf("invalid identity %q", config.Identity)
	}
	return config, nil
}

// Input is the publish command line before resolution.
type Input struct {
	File        string
	Slug        string
	Destination string
	Identity    string
	Timeout     time.Duration
	ConfigPath  string
}

// Resolved is the publish command line after resolution.
type Resolved struct {
	File        string
	Slug        string
	Destination destination.Value
	Identity    string
	Token       string
	Timeout     time.Duration
}

// Resolve derives every input in the spec order. A resolved value that
// fails validation is a usage error.
func Resolve(input Input) (*Resolved, error) {
	return ResolveWithRuntime(input, productionRuntime())
}

// ResolveWithRuntime resolves publish inputs through explicit process
// resources.
func ResolveWithRuntime(input Input, runtime Runtime) (*Resolved, error) {
	config, err := loadConfig(runtime.FileSystem, input.ConfigPath)
	if err != nil {
		return nil, err
	}
	if input.File == "" || input.Slug == "" {
		return nil, fmt.Errorf("%w: --file and --slug are required", ErrUsage)
	}
	if input.Timeout <= 0 {
		return nil, fmt.Errorf("%w: --timeout must be positive", ErrUsage)
	}
	if !pages.ValidName(input.Slug) {
		return nil, fmt.Errorf("%w: invalid slug %q", ErrUsage, input.Slug)
	}
	extension := strings.ToLower(filepath.Ext(input.File))
	if extension != ".html" && extension != ".zip" {
		return nil, fmt.Errorf("%w: --file must end in .html or .zip", ErrUsage)
	}
	if info, statError := runtime.FileSystem.Stat(input.File); statError == nil && info.IsDir() {
		return nil, fmt.Errorf("%w: --file must not be a directory; package it as a zip", ErrUsage)
	}

	resolvedDestination, err := destination.ParseWithCurrentDirectory(firstNonEmpty(input.Destination, runtime.Environment("PAGES_DESTINATION"), config.Destination), runtime.CurrentDirectory)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUsage, err)
	}
	remote := resolvedDestination.IsRemote()

	identity := ""
	token := ""
	if remote {
		token = runtime.Environment("PAGES_UPLOAD_TOKEN")
		if token != "" {
			prefix, err := pages.TokenIdentity(token)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrUsage, err)
			}
			identity = prefix
		} else {
			identity = firstNonEmpty(input.Identity, config.Identity)
			if identity != "" && !pages.ValidName(identity) {
				return nil, fmt.Errorf("%w: invalid identity %q", ErrUsage, identity)
			}
			if identity == "" {
				token = config.Token
			} else if secret := config.Identities[identity]; secret != "" {
				token = identity + "." + secret
			}
		}
		if token == "" {
			if identity == "" {
				return nil, fmt.Errorf("%w: no upload token for default identity", ErrUsage)
			}
			return nil, fmt.Errorf("%w: no upload token for identity %q", ErrUsage, identity)
		}
	} else {
		identity = firstNonEmpty(input.Identity, config.Identity)
		if identity != "" && !pages.ValidName(identity) {
			return nil, fmt.Errorf("%w: invalid identity %q", ErrUsage, identity)
		}
	}

	resolved := &Resolved{
		File:        input.File,
		Slug:        input.Slug,
		Destination: resolvedDestination,
		Identity:    identity,
		Token:       token,
		Timeout:     input.Timeout,
	}
	return resolved, nil
}

// Run executes the resolved publish and returns the one line to print.
func Run(resolved *Resolved) (string, error) {
	return RunWithRuntime(resolved, productionRuntime())
}

// RunWithRuntime executes a resolved publish through explicit process
// resources.
func RunWithRuntime(resolved *Resolved, runtime Runtime) (string, error) {
	if !resolved.Destination.IsRemote() {
		return runLocal(resolved, runtime.FileSystem)
	}
	ctx, cancel := context.WithTimeout(context.Background(), resolved.Timeout)
	defer cancel()
	publisher := client.Publisher{BaseURL: resolved.Destination.String(), Token: resolved.Token, HTTPClient: runtime.HTTPClient, FileSystem: runtime.FileSystem}
	return publisher.Publish(ctx, resolved.File, resolved.Slug)
}

// runLocal writes the page directly into the local public root with the
// same validation and swap rules as the server. Only the zip safety
// checks run: at most 512 entries and an uncompressed total of at most
// four times the compressed size.
func runLocal(resolved *Resolved, fileSystem filesystem.FS) (result string, resultErr error) {
	stager, err := pages.NewStagerWithFS(resolved.Destination.LocalPath(), fileSystem)
	if err != nil {
		return "", err
	}
	stagedDir, err := stager.StageDir(resolved.Identity)
	if err != nil {
		return "", err
	}
	defer func() {
		if resultErr != nil {
			_ = fileSystem.RemoveAll(stagedDir)
		}
	}()

	if strings.EqualFold(filepath.Ext(resolved.File), ".zip") {
		info, err := fileSystem.Stat(resolved.File)
		if err != nil {
			return "", fmt.Errorf("stat zip file: %w", err)
		}
		if err := pages.StageZipWithFS(fileSystem, resolved.File, stagedDir, info.Size()); err != nil {
			return "", err
		}
		if err := stager.SwapZip(resolved.Identity, resolved.Slug, stagedDir); err != nil {
			return "", err
		}
	} else {
		source, err := fileSystem.Open(resolved.File)
		if err != nil {
			return "", fmt.Errorf("open page file: %w", err)
		}
		target, err := fileSystem.OpenFile(filepath.Join(stagedDir, "index.html"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			source.Close()
			return "", fmt.Errorf("create staged page: %w", err)
		}
		_, copyError := io.Copy(target, source)
		source.Close()
		closeError := target.Close()
		if copyError != nil {
			return "", fmt.Errorf("write page: %w", copyError)
		}
		if closeError != nil {
			return "", fmt.Errorf("close page: %w", closeError)
		}
		if err := stager.SwapHTML(resolved.Identity, resolved.Slug, stagedDir); err != nil {
			return "", err
		}
	}

	absoluteTarget, err := fileSystem.Abs(resolved.Destination.LocalPath())
	if err != nil {
		return "", fmt.Errorf("resolve public root: %w", err)
	}
	segments := []string{absoluteTarget}
	if scope := pages.IdentityScope(resolved.Identity); scope != "" {
		segments = append(segments, scope)
	}
	segments = append(segments, resolved.Slug)
	return filepath.Join(segments...) + string(filepath.Separator), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
