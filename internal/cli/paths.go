package cli

import (
	"fmt"
	"path/filepath"
)

func resolveConfigDir(runtime commandRuntime, configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	userConfigDir, err := runtime.userConfigDir()
	if err != nil {
		return "", fmt.Errorf("determine user config directory: %w", err)
	}
	if userConfigDir == "" {
		return "", fmt.Errorf("determine user config directory: operating system returned an empty path")
	}
	return filepath.Join(userConfigDir, "pages"), nil
}

func configFilePath(configDir string) string {
	return configFilePathNamed(configDir, "config.json")
}

func configFilePathNamed(configDir, name string) string {
	return filepath.Join(configDir, name)
}
