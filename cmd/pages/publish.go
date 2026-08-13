package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/suraciii/pages/internal/publish"
)

func runPublish(args []string) int {
	flags := flag.NewFlagSet("pages publish", flag.ExitOnError)
	filePath := flags.String("file", "", "HTML or zip file to publish (required)")
	slug := flags.String("slug", "", "page slug (required)")
	baseURL := flags.String("base-url", "", "remote upload address; selects remote mode")
	identity := flags.String("identity", "", "identity; derived when empty")
	timeout := flags.Duration("timeout", 90*time.Second, "upload and verification timeout")
	configFlag := flags.String("config", defaultConfigPath(), "config file")
	flags.Parse(args)

	configPath := *configFlag
	explicitConfig := false
	flags.Visit(func(set *flag.Flag) {
		if set.Name == "config" {
			explicitConfig = true
		}
	})
	if !explicitConfig {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			configPath = ""
		}
	}

	resolved, err := publish.Resolve(publish.Input{
		File:       *filePath,
		Slug:       *slug,
		BaseURL:    *baseURL,
		Identity:   *identity,
		Timeout:    *timeout,
		ConfigPath: configPath,
	})
	if err != nil {
		if errors.Is(err, publish.ErrUsage) {
			fmt.Fprintf(os.Stderr, "pages publish: %v\n", err)
			flags.Usage()
			return 2
		}
		fmt.Fprintf(os.Stderr, "pages publish: %v\n", err)
		return 1
	}

	result, err := publish.Run(resolved)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pages publish: %v\n", err)
		return 1
	}
	fmt.Println(result)
	return 0
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "pages", "config.json")
}
