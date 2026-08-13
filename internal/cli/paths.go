package cli

import (
	"os"
	"path/filepath"
)

func defaultConfigDir() string {
	if value := os.Getenv("PAGES_CONFIG_DIR"); value != "" {
		return value
	}
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(userConfigDir, "pages")
}

func defaultConfigPath() string {
	return configFilePath(defaultConfigDir())
}

func configFilePath(configDir string) string {
	return configFilePathNamed(configDir, "config.json")
}

func configFilePathNamed(configDir, name string) string {
	return filepath.Join(configDir, name)
}
