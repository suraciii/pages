package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/suraciii/pages/internal/pages"
)

const (
	defaultUploadLimit = 10485760
)

func runServe(args []string, runtime commandRuntime) int {
	flags := flag.NewFlagSet("pages serve", flag.ContinueOnError)
	flags.Usage = subcommandUsage(flags, "Usage: pages serve [flags]")
	listenAddress := flags.String("listen", envOr(runtime.environment, "PAGES_LISTEN_ADDR", "127.0.0.1:3103"), "loopback listen address")
	destinationValue := destinationFlags(flags)
	configDir := flags.String("config-dir", runtime.environment("PAGES_CONFIG_DIR"), "configuration directory")
	tokensFile := flags.String("tokens-file", runtime.environment("PAGES_TOKENS_FILE"), "identity token JSON file")
	maxUploadBytes := flags.Int64("max-upload-bytes", envInt64(runtime.environment, runtime.stderr, "PAGES_MAX_UPLOAD_BYTES", defaultUploadLimit), "maximum upload size in bytes")
	if status, ok := parseFlags(flags, args, runtime.stdout, runtime.stderr); !ok {
		return status
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(runtime.stderr, "pages serve: positional arguments are not allowed")
		flags.Usage()
		return 2
	}
	if legacy, found := legacyDestinationEnvironment(runtime.environment); found {
		fmt.Fprintf(runtime.stderr, "pages serve: %s is not supported; use PAGES_DESTINATION\n", legacy)
		return 2
	}
	destinationInput := *destinationValue
	if !flagWasSet(flags, "destination") && !flagWasSet(flags, "dest") {
		destinationInput = runtime.environment("PAGES_DESTINATION")
	}
	resolvedDestination, err := parseDestinationFlags(flags, destinationInput, true, runtime.currentDirectory)
	if err != nil {
		fmt.Fprintf(runtime.stderr, "pages serve: %v\n", err)
		flags.Usage()
		return 2
	}
	if *tokensFile == "" {
		resolvedConfigDir, err := resolveConfigDir(runtime, *configDir)
		if err != nil {
			fmt.Fprintf(runtime.stderr, "pages serve: %v; use --config-dir or --tokens-file\n", err)
			return 1
		}
		*tokensFile = configFilePathNamed(resolvedConfigDir, "tokens.json")
	}

	if *maxUploadBytes <= 0 {
		fmt.Fprintln(runtime.stderr, "pages serve: --max-upload-bytes must be positive")
		return 2
	}

	tokens, err := pages.LoadTokens(runtime.fileSystem, *tokensFile)
	if err != nil {
		fmt.Fprintf(runtime.stderr, "pages serve: %v\n", err)
		return 1
	}
	server, err := pages.NewServer(pages.ServerConfig{
		PublicRoot:     resolvedDestination.LocalPath(),
		Tokens:         tokens,
		MaxUploadBytes: *maxUploadBytes,
		FileSystem:     runtime.fileSystem,
	})
	if err != nil {
		fmt.Fprintf(runtime.stderr, "pages serve: %v\n", err)
		return 1
	}

	httpServer := &http.Server{
		Addr:              *listenAddress,
		Handler:           server,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("pages serve: listen=%s destination=%s tokens-file=%s max-upload-bytes=%d", *listenAddress, resolvedDestination.String(), *tokensFile, *maxUploadBytes)

	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for range hup {
			if err := server.ReloadTokens(*tokensFile); err != nil {
				log.Printf("pages serve: reload tokens: %v", err)
				continue
			}
			log.Printf("pages serve: reloaded tokens from %s", *tokensFile)
		}
	}()

	go func() {
		<-shutdownContext.Done()
		shutdownTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownTimeout)
	}()

	if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(runtime.stderr, "pages serve: %v\n", err)
		return 1
	}
	return 0
}

func envOr(environment func(string) string, name, fallback string) string {
	if value := environment(name); value != "" {
		return value
	}
	return fallback
}

func envInt64(environment func(string) string, stderr io.Writer, name string, fallback int64) int64 {
	value := environment(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		fmt.Fprintf(stderr, "pages serve: invalid %s; using default %d\n", name, fallback)
		return fallback
	}
	return parsed
}
