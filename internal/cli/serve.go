package cli

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/suraciii/pages/internal/destination"
	"github.com/suraciii/pages/internal/filesystem"
	"github.com/suraciii/pages/internal/pages"
)

const (
	defaultUploadLimit = 10485760
)

const serveUsageNotes = `
Service
  One listener serves page reads and authenticated token uploads. The default
  listener is loopback. A public deployment sets --listen and either direct
  TLS (--tls-cert and --tls-key) or a proxy that forwards every method and
  path to the loopback listener.

Inputs
  The Public Root defaults to pages-public/ under the current directory.
  serve reads no configuration file; flags and environment values are the
  only inputs. The tokens file, certificate chain, and private key must stay
  outside the Public Root or the Service refuses to start.

Policy
  The default Content Security Policy blocks scripts and external resources.
  --content-security-policy replaces the whole policy. Health is
  GET /__pages/healthz. A missing tokens file starts read-only; run
  pages generate-token to enable remote Uploads.

Reload
  SIGHUP reloads the tokens file and the certificate pair independently.
  Changing the listener, Destination, upload limit, CSP, configuration
  directory, tokens path, or certificate path needs a restart.

Contract: design/cli.md is the authoritative command contract, revision ` + commandContractRevision + `.
This help was checked against it.
`

func runServe(args []string, runtime commandRuntime) int {
	flags := flag.NewFlagSet("pages serve", flag.ContinueOnError)
	flags.Usage = subcommandUsage(flags, "Usage: pages serve [flags]", serveUsageNotes)
	listenAddress := flags.String("listen", envOr(runtime.environment, "PAGES_LISTEN_ADDR", "127.0.0.1:3103"), "listen address (host:port)")
	destinationValue := destinationFlags(flags, true)
	configDir := flags.String("config-dir", runtime.environment("PAGES_CONFIG_DIR"), "configuration directory")
	tokensFile := flags.String("tokens-file", runtime.environment("PAGES_TOKENS_FILE"), "identity token JSON file")
	maxUploadBytes := flags.Int64("max-upload-bytes", defaultUploadLimit, "maximum upload size in bytes")
	tlsCert := flags.String("tls-cert", runtime.environment("PAGES_TLS_CERT"), "PEM certificate chain; requires --tls-key")
	tlsKey := flags.String("tls-key", runtime.environment("PAGES_TLS_KEY"), "PEM private key; requires --tls-cert")
	contentSecurityPolicy := flags.String("content-security-policy", runtime.environment("PAGES_CSP"), "replace the reference Content Security Policy")
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
	if err := rejectDestinationAliasConflict(flags); err != nil {
		fmt.Fprintf(runtime.stderr, "pages serve: %v\n", err)
		flags.Usage()
		return 2
	}

	destinationInput := *destinationValue
	if !flagWasSet(flags, "destination") && !flagWasSet(flags, "dest") {
		destinationInput = runtime.environment("PAGES_DESTINATION")
	}
	if destinationInput == "" {
		destinationInput = destination.DefaultPublicRoot
	}
	resolvedDestination, err := parseDestinationFlags(flags, destinationInput, true, runtime.currentDirectory)
	if err != nil {
		fmt.Fprintf(runtime.stderr, "pages serve: %v\n", err)
		flags.Usage()
		return 2
	}

	uploadLimit := *maxUploadBytes
	if !flagWasSet(flags, "max-upload-bytes") {
		if value := runtime.environment("PAGES_MAX_UPLOAD_BYTES"); value != "" {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil || parsed <= 0 {
				fmt.Fprintf(runtime.stderr, "pages serve: invalid PAGES_MAX_UPLOAD_BYTES %q: must be a positive integer\n", value)
				return 2
			}
			uploadLimit = parsed
		}
	}
	if uploadLimit <= 0 {
		fmt.Fprintln(runtime.stderr, "pages serve: --max-upload-bytes must be positive")
		return 2
	}

	listenerLoopback, err := validateListenAddress(*listenAddress)
	if err != nil {
		fmt.Fprintf(runtime.stderr, "pages serve: %v\n", err)
		return 2
	}

	certificateInput := *tlsCert
	keyInput := *tlsKey
	if flagWasSet(flags, "tls-cert") && certificateInput == "" {
		fmt.Fprintln(runtime.stderr, "pages serve: --tls-cert must not be empty")
		return 2
	}
	if flagWasSet(flags, "tls-key") && keyInput == "" {
		fmt.Fprintln(runtime.stderr, "pages serve: --tls-key must not be empty")
		return 2
	}
	if (certificateInput == "") != (keyInput == "") {
		fmt.Fprintln(runtime.stderr, "pages serve: --tls-cert and --tls-key must be set together")
		return 2
	}
	tlsEnabled := certificateInput != ""

	cspInput := *contentSecurityPolicy
	if flagWasSet(flags, "content-security-policy") && cspInput == "" {
		fmt.Fprintln(runtime.stderr, "pages serve: --content-security-policy must not be empty")
		return 2
	}
	if cspInput != "" {
		if err := validateContentSecurityPolicy(cspInput); err != nil {
			fmt.Fprintf(runtime.stderr, "pages serve: %v\n", err)
			return 2
		}
	}
	cspState := "default"
	if cspInput != "" {
		cspState = "custom"
	} else {
		cspInput = pages.DefaultContentSecurityPolicy
	}

	if *tokensFile == "" {
		resolvedConfigDir, err := resolveConfigDir(runtime, *configDir)
		if err != nil {
			fmt.Fprintf(runtime.stderr, "pages serve: %v; use --config-dir or --tokens-file\n", err)
			return 1
		}
		*tokensFile = configFilePathNamed(resolvedConfigDir, "tokens.json")
	}

	publicRoot, err := resolveAbsolutePath(runtime.fileSystem, resolvedDestination.LocalPath())
	if err != nil {
		fmt.Fprintf(runtime.stderr, "pages serve: %v\n", err)
		return 1
	}
	tokensPath, err := resolveAbsolutePath(runtime.fileSystem, *tokensFile)
	if err != nil {
		fmt.Fprintf(runtime.stderr, "pages serve: %v\n", err)
		return 1
	}
	if pathWithin(publicRoot, tokensPath) {
		fmt.Fprintf(runtime.stderr, "pages serve: tokens file %s is inside the Public Root %s\n", tokensPath, publicRoot)
		return 1
	}
	certificatePath := ""
	keyPath := ""
	if tlsEnabled {
		certificatePath, err = resolveAbsolutePath(runtime.fileSystem, certificateInput)
		if err != nil {
			fmt.Fprintf(runtime.stderr, "pages serve: %v\n", err)
			return 1
		}
		keyPath, err = resolveAbsolutePath(runtime.fileSystem, keyInput)
		if err != nil {
			fmt.Fprintf(runtime.stderr, "pages serve: %v\n", err)
			return 1
		}
		for _, credentialPath := range []string{certificatePath, keyPath} {
			if pathWithin(publicRoot, credentialPath) {
				fmt.Fprintf(runtime.stderr, "pages serve: TLS file %s is inside the Public Root %s\n", credentialPath, publicRoot)
				return 1
			}
		}
	}

	tokens, err := pages.LoadTokens(runtime.fileSystem, tokensPath)
	if err != nil {
		fmt.Fprintf(runtime.stderr, "pages serve: %v\n", err)
		return 1
	}

	var tlsConfig *tls.Config
	var certificates *certificateStore
	if tlsEnabled {
		certificatePEM, err := runtime.fileSystem.ReadFile(certificatePath)
		if err != nil {
			fmt.Fprintf(runtime.stderr, "pages serve: read certificate: %v\n", err)
			return 1
		}
		keyPEM, err := runtime.fileSystem.ReadFile(keyPath)
		if err != nil {
			fmt.Fprintf(runtime.stderr, "pages serve: read private key: %v\n", err)
			return 1
		}
		certificates, err = newCertificateStore(certificatePEM, keyPEM)
		if err != nil {
			fmt.Fprintf(runtime.stderr, "pages serve: load certificate pair: %v\n", err)
			return 1
		}
		tlsConfig = &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: certificates.getCertificate,
		}
	}

	logger := log.New(runtime.stderr, "pages serve: ", 0)
	pageServer, err := pages.NewServer(pages.ServerConfig{
		PublicRoot:            publicRoot,
		Tokens:                tokens,
		MaxUploadBytes:        uploadLimit,
		ContentSecurityPolicy: cspInput,
		FileSystem:            runtime.fileSystem,
		Logger:                logger,
	})
	if err != nil {
		fmt.Fprintf(runtime.stderr, "pages serve: %v\n", err)
		return 1
	}

	if !tlsEnabled && !listenerLoopback {
		logger.Printf("warning: listen address %s is not loopback; Tokens and Page content travel in plaintext", *listenAddress)
	}
	if tokens.Count() == 0 {
		logger.Printf("warning: no Tokens loaded; remote Uploads are disabled")
	}
	logger.Printf("listen=%s destination=%s tokens-file=%s max-upload-bytes=%d tokens=%d tls=%s csp=%s",
		*listenAddress, publicRoot, tokensPath, uploadLimit, tokens.Count(), onOff(tlsEnabled), cspState)

	listen := runtime.listen
	if listen == nil {
		listen = net.Listen
	}
	listener, err := listen("tcp", *listenAddress)
	if err != nil {
		fmt.Fprintf(runtime.stderr, "pages serve: listen: %v\n", err)
		return 1
	}

	httpServer := &http.Server{
		Addr:              *listenAddress,
		Handler:           pageServer,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          logger,
	}

	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	go func() {
		for range hup {
			reloadInputs(logger, runtime.fileSystem, pageServer, tokensPath, certificates, certificatePath, keyPath)
		}
	}()

	go func() {
		<-shutdownContext.Done()
		shutdownTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownTimeout)
	}()

	var serveError error
	if tlsEnabled {
		serveError = httpServer.ServeTLS(listener, "", "")
	} else {
		serveError = httpServer.Serve(listener)
	}
	if !errors.Is(serveError, http.ErrServerClosed) {
		fmt.Fprintf(runtime.stderr, "pages serve: %v\n", serveError)
		return 1
	}
	return 0
}

// reloadInputs reloads the tokens and certificate pair independently. A
// failed load keeps the previous value for that input.
func reloadInputs(logger *log.Logger, fileSystem filesystem.FS, pageServer *pages.Server, tokensPath string, certificates *certificateStore, certificatePath, keyPath string) {
	if err := pageServer.ReloadTokens(tokensPath); err != nil {
		logger.Printf("reload Tokens: %v", err)
	} else {
		logger.Printf("reloaded Tokens from %s", tokensPath)
	}
	if certificates == nil {
		return
	}
	certificatePEM, certificateErr := fileSystem.ReadFile(certificatePath)
	if certificateErr == nil {
		keyPEM, keyErr := fileSystem.ReadFile(keyPath)
		if keyErr != nil {
			certificateErr = keyErr
		} else {
			certificateErr = certificates.reload(certificatePEM, keyPEM)
		}
	}
	if certificateErr != nil {
		logger.Printf("reload certificate: %v", certificateErr)
	} else {
		logger.Printf("reloaded certificate from %s", certificatePath)
	}
}

func envOr(environment func(string) string, name, fallback string) string {
	if value := environment(name); value != "" {
		return value
	}
	return fallback
}

func onOff(value bool) string {
	if value {
		return "on"
	}
	return "off"
}
