package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/suraciii/pages/internal/pages"
)

const defaultUploadLimit = 10485760

func runServe(args []string) int {
	flags := flag.NewFlagSet("pages serve", flag.ExitOnError)
	listenAddress := flags.String("listen", envOr("PAGES_LISTEN_ADDR", "127.0.0.1:3103"), "loopback listen address")
	publicRoot := flags.String("public-root", os.Getenv("PAGES_PUBLIC_ROOT"), "static-resources directory (required)")
	tokensFile := flags.String("tokens-file", os.Getenv("PAGES_TOKENS_FILE"), "identity token JSON file (required)")
	maxUploadBytes := flags.Int64("max-upload-bytes", envInt64("PAGES_MAX_UPLOAD_BYTES", defaultUploadLimit), "maximum upload size in bytes")
	flags.Parse(args)

	if *publicRoot == "" || *tokensFile == "" {
		fmt.Fprintln(os.Stderr, "pages serve: -public-root and -tokens-file are required")
		flags.Usage()
		return 2
	}
	if *maxUploadBytes <= 0 {
		fmt.Fprintln(os.Stderr, "pages serve: -max-upload-bytes must be positive")
		return 2
	}

	tokens, err := pages.LoadTokens(*tokensFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pages serve: %v\n", err)
		return 1
	}
	server, err := pages.NewServer(pages.ServerConfig{
		PublicRoot:     *publicRoot,
		Tokens:         tokens,
		MaxUploadBytes: *maxUploadBytes,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "pages serve: %v\n", err)
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

	log.Printf("pages serve: listen=%s public-root=%s tokens-file=%s max-upload-bytes=%d", *listenAddress, *publicRoot, *tokensFile, *maxUploadBytes)

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
		fmt.Fprintf(os.Stderr, "pages serve: %v\n", err)
		return 1
	}
	return 0
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt64(name string, fallback int64) int64 {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		fmt.Fprintf(os.Stderr, "pages serve: invalid %s; using default %d\n", name, fallback)
		return fallback
	}
	return parsed
}
