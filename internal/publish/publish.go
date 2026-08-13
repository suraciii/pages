// Package publish resolves the publish inputs and runs the publish.
package publish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/suraciii/pages/internal/client"
	"github.com/suraciii/pages/internal/pages"
)

// ErrUsage marks a resolved value that fails validation. Callers report it
// as a usage error.
var ErrUsage = errors.New("usage")

// Config is the publish config file. Every field is optional.
type Config struct {
	Remote     string            `json:"remote"`
	PublicRoot string            `json:"public-root"`
	Tokens     map[string]string `json:"tokens"`
	Identity   string            `json:"identity"`
}

// LoadConfig reads a config file. An empty path means the default file:
// a missing default file is fine. A named file must exist.
func LoadConfig(path string) (*Config, error) {
	config := &Config{}
	if path == "" {
		return config, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	for identity, secret := range config.Tokens {
		if !pages.ValidName(identity) || secret == "" || strings.Contains(secret, ".") {
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
	File       string
	Slug       string
	Remote     string
	Identity   string
	Timeout    time.Duration
	ConfigPath string
}

// Resolved is the publish command line after resolution.
type Resolved struct {
	File          string
	Slug          string
	Remote        bool
	RemoteAddress string
	LocalTarget   string
	Identity      string
	Token         string
	Timeout       time.Duration
}

// Resolve derives every input in the spec order. A resolved value that
// fails validation is a usage error.
func Resolve(input Input) (*Resolved, error) {
	config, err := LoadConfig(input.ConfigPath)
	if err != nil {
		return nil, err
	}
	if input.File == "" || input.Slug == "" {
		return nil, fmt.Errorf("%w: -file and -slug are required", ErrUsage)
	}
	if input.Timeout <= 0 {
		return nil, fmt.Errorf("%w: -timeout must be positive", ErrUsage)
	}
	if !pages.ValidName(input.Slug) {
		return nil, fmt.Errorf("%w: invalid slug %q", ErrUsage, input.Slug)
	}
	extension := strings.ToLower(filepath.Ext(input.File))
	if extension != ".html" && extension != ".zip" {
		return nil, fmt.Errorf("%w: -file must end in .html or .zip", ErrUsage)
	}
	if info, statError := os.Stat(input.File); statError == nil && info.IsDir() {
		return nil, fmt.Errorf("%w: -file must not be a directory; package it as a zip", ErrUsage)
	}

	remoteAddress := firstNonEmpty(input.Remote, os.Getenv("PAGES_REMOTE"), config.Remote)
	remote := remoteAddress != ""

	environmentToken := os.Getenv("PAGES_UPLOAD_TOKEN")
	identity := ""
	switch {
	case environmentToken != "":
		prefix, err := pages.TokenIdentity(environmentToken)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUsage, err)
		}
		identity = prefix
	case input.Identity != "":
		identity = input.Identity
	case config.Identity != "":
		identity = config.Identity
	case len(config.Tokens) == 1:
		for key := range config.Tokens {
			identity = key
		}
	default:
		return nil, fmt.Errorf("%w: no identity; set -identity or config.identity", ErrUsage)
	}
	if !pages.ValidName(identity) {
		return nil, fmt.Errorf("%w: invalid identity %q", ErrUsage, identity)
	}

	token := environmentToken
	if token == "" {
		if secret, ok := config.Tokens[identity]; ok {
			token = identity + "." + secret
		}
	}
	if remote && token == "" {
		return nil, fmt.Errorf("%w: no upload token for identity %q", ErrUsage, identity)
	}

	resolved := &Resolved{
		File:          input.File,
		Slug:          input.Slug,
		Remote:        remote,
		RemoteAddress: remoteAddress,
		Identity:      identity,
		Token:         token,
		Timeout:       input.Timeout,
	}
	if !remote {
		resolved.LocalTarget = firstNonEmpty(os.Getenv("PAGES_PUBLIC_ROOT"), config.PublicRoot)
		if resolved.LocalTarget == "" {
			return nil, fmt.Errorf("%w: no public root; set PAGES_PUBLIC_ROOT or config.public-root", ErrUsage)
		}
	}
	return resolved, nil
}

// Run executes the resolved publish and returns the one line to print.
func Run(resolved *Resolved) (string, error) {
	if !resolved.Remote {
		return runLocal(resolved)
	}
	ctx, cancel := context.WithTimeout(context.Background(), resolved.Timeout)
	defer cancel()
	publisher := client.Publisher{BaseURL: resolved.RemoteAddress, Token: resolved.Token}
	return publisher.Publish(ctx, resolved.File, resolved.Slug)
}

// runLocal writes the page directly into the local public root with the
// same validation and swap rules as the server. Only the zip safety
// checks run: at most 512 entries and an uncompressed total of at most
// four times the compressed size.
func runLocal(resolved *Resolved) (result string, resultErr error) {
	stager, err := pages.NewStager(resolved.LocalTarget)
	if err != nil {
		return "", err
	}
	stagedDir, err := stager.StageDir(resolved.Identity)
	if err != nil {
		return "", err
	}
	defer func() {
		if resultErr != nil {
			_ = os.RemoveAll(stagedDir)
		}
	}()

	if strings.EqualFold(filepath.Ext(resolved.File), ".zip") {
		info, err := os.Stat(resolved.File)
		if err != nil {
			return "", fmt.Errorf("stat zip file: %w", err)
		}
		if err := pages.StageZip(resolved.File, stagedDir, info.Size()); err != nil {
			return "", err
		}
		if err := stager.SwapZip(resolved.Identity, resolved.Slug, stagedDir); err != nil {
			return "", err
		}
	} else {
		source, err := os.Open(resolved.File)
		if err != nil {
			return "", fmt.Errorf("open page file: %w", err)
		}
		target, err := os.Create(filepath.Join(stagedDir, "index.html"))
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

	absoluteTarget, err := filepath.Abs(resolved.LocalTarget)
	if err != nil {
		return "", fmt.Errorf("resolve public root: %w", err)
	}
	return filepath.Join(absoluteTarget, resolved.Identity, resolved.Slug) + "/", nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
