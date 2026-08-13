package cli

import (
	"io"
	"net/http"
	"os"

	"github.com/suraciii/pages/internal/filesystem"
	"github.com/suraciii/pages/internal/publish"
)

type commandRuntime struct {
	fileSystem       filesystem.FS
	environment      func(string) string
	userConfigDir    func() (string, error)
	currentDirectory func() (string, error)
	httpClient       interface {
		Do(*http.Request) (*http.Response, error)
	}
	stdout io.Writer
	stderr io.Writer
}

func productionCommandRuntime() commandRuntime {
	return commandRuntime{
		fileSystem:       filesystem.OS,
		environment:      os.Getenv,
		userConfigDir:    os.UserConfigDir,
		currentDirectory: os.Getwd,
		stdout:           os.Stdout,
		stderr:           os.Stderr,
	}
}

func (runtime commandRuntime) publishRuntime() publish.Runtime {
	return publish.Runtime{
		FileSystem:       runtime.fileSystem,
		Environment:      runtime.environment,
		CurrentDirectory: runtime.currentDirectory,
		HTTPClient:       runtime.httpClient,
	}
}
