package pages

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/suraciii/pages/internal/filesystem"
)

// ServerConfig contains the local filesystem and authorization boundary.
type ServerConfig struct {
	PublicRoot     string
	Tokens         Tokens
	MaxUploadBytes int64
	FileSystem     filesystem.FS
}

// Server publishes standalone HTML pages into an identity namespace.
type Server struct {
	stager         *Stager
	fileSystem     filesystem.FS
	maxUploadBytes int64

	tokensMu sync.RWMutex
	tokens   Tokens
}

func NewServer(config ServerConfig) (*Server, error) {
	if config.PublicRoot == "" {
		return nil, errors.New("public root is required")
	}
	if config.MaxUploadBytes <= 0 {
		return nil, errors.New("maximum upload bytes must be positive")
	}
	if err := config.Tokens.validate(true); err != nil {
		return nil, err
	}
	fileSystem := config.FileSystem
	if fileSystem == nil {
		fileSystem = filesystem.OS
	}
	stager, err := NewStager(config.PublicRoot, fileSystem)
	if err != nil {
		return nil, err
	}
	if err := stager.Recover(); err != nil {
		return nil, err
	}
	return &Server{
		stager:         stager,
		fileSystem:     fileSystem,
		tokens:         config.Tokens,
		maxUploadBytes: config.MaxUploadBytes,
	}, nil
}

// ReloadTokens replaces the token set after a successful load. A failed
// load keeps the previous set.
func (server *Server) ReloadTokens(path string) error {
	reloaded, err := LoadTokens(server.fileSystem, path)
	if err != nil {
		return err
	}
	server.tokensMu.Lock()
	server.tokens = reloaded
	server.tokensMu.Unlock()
	return nil
}

func (server *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet && request.URL.Path == "/healthz" {
		writer.WriteHeader(http.StatusOK)
		return
	}
	if request.Method != http.MethodPost {
		http.NotFound(writer, request)
		return
	}

	slug, valid := uploadSlug(request.URL.Path)
	if !valid {
		http.NotFound(writer, request)
		return
	}

	server.tokensMu.RLock()
	identity, authenticated := server.tokens.Authenticate(request.Header.Get("Authorization"))
	server.tokensMu.RUnlock()
	if !authenticated {
		http.Error(writer, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if !isUploadContentType(request.Header.Get("Content-Type")) || request.ContentLength == 0 {
		http.Error(writer, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if request.ContentLength > server.maxUploadBytes {
		http.Error(writer, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
		return
	}

	if err := server.publish(writer, request, identity, slug); err != nil {
		switch {
		case errors.Is(err, errInvalidUTF8), errors.Is(err, errEmptyBody), errors.Is(err, errInvalidZip):
			http.Error(writer, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		case isMaxBytesError(err):
			http.Error(writer, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
		default:
			http.Error(writer, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		}
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

var errEmptyBody = errors.New("body is empty")

func (server *Server) publish(writer http.ResponseWriter, request *http.Request, identity, slug string) (result error) {
	stagedDir, err := server.stager.StageDir(identity)
	if err != nil {
		return err
	}
	defer func() {
		if result != nil {
			_ = server.stager.fs.RemoveAll(stagedDir)
		}
	}()

	mediaType, _, _ := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if strings.EqualFold(mediaType, "application/zip") {
		return server.publishZip(writer, request, identity, slug, stagedDir)
	}
	return server.publishHTML(writer, request, identity, slug, stagedDir)
}

func (server *Server) publishHTML(writer http.ResponseWriter, request *http.Request, identity, slug, stagedDir string) error {
	indexFile, err := server.stager.fs.OpenFile(filepath.Join(stagedDir, "index.html"), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create staged page: %w", err)
	}

	limitedBody := http.MaxBytesReader(writer, request.Body, server.maxUploadBytes)
	defer limitedBody.Close()
	validator := &utf8Validator{}
	bytesWritten, err := io.Copy(indexFile, io.TeeReader(limitedBody, validator))
	if err != nil {
		return fmt.Errorf("write upload: %w", err)
	}
	if bytesWritten == 0 {
		return errEmptyBody
	}
	if !validator.Valid() {
		return errInvalidUTF8
	}
	if err := indexFile.Close(); err != nil {
		return fmt.Errorf("close page: %w", err)
	}
	return server.stager.SwapHTML(identity, slug, stagedDir)
}

func (server *Server) publishZip(writer http.ResponseWriter, request *http.Request, identity, slug, stagedDir string) error {
	uploadFile, err := server.stager.UploadFile(identity)
	if err != nil {
		return err
	}
	defer func() {
		_ = server.stager.fs.Remove(uploadFile)
	}()

	upload, err := server.stager.fs.OpenFile(uploadFile, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open upload body: %w", err)
	}
	limitedBody := http.MaxBytesReader(writer, request.Body, server.maxUploadBytes)
	defer limitedBody.Close()
	bytesWritten, err := io.Copy(upload, limitedBody)
	closeError := upload.Close()
	if err != nil {
		return fmt.Errorf("write upload: %w", err)
	}
	if closeError != nil {
		return fmt.Errorf("close upload body: %w", closeError)
	}
	if bytesWritten == 0 {
		return errEmptyBody
	}
	if err := StageZip(server.stager.fs, uploadFile, stagedDir, server.maxUploadBytes); err != nil {
		return err
	}
	return server.stager.SwapZip(identity, slug, stagedDir)
}

func uploadSlug(path string) (string, bool) {
	const prefix = "/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	slug := strings.TrimPrefix(path, prefix)
	return slug, ValidName(slug)
}

func isUploadContentType(contentType string) bool {
	mediaType, parameters, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	if strings.EqualFold(mediaType, "text/html") {
		charset, found := parameters["charset"]
		return !found || strings.EqualFold(charset, "utf-8")
	}
	return strings.EqualFold(mediaType, "application/zip")
}

func isMaxBytesError(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}
