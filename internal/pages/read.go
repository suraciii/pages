package pages

import (
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	healthPath = "/__pages/healthz"

	// DefaultContentSecurityPolicy blocks scripts and external resources. It
	// allows the inline style of the generated Page Index and same-origin or
	// data images, fonts, and styles.
	DefaultContentSecurityPolicy = "default-src 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
)

func (server *Server) setSecurityHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Content-Security-Policy", server.contentSecurityPolicy)
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	writer.Header().Set("X-Frame-Options", "DENY")
	writer.Header().Set("Cache-Control", "no-store")
}

func (server *Server) serveRead(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == healthPath {
		writer.WriteHeader(http.StatusOK)
		return
	}
	target, redirect, status := server.resolveReadPath(request.URL.Path)
	if redirect != "" {
		http.Redirect(writer, request, redirect, http.StatusMovedPermanently)
		return
	}
	if status != 0 {
		http.Error(writer, http.StatusText(status), status)
		return
	}
	server.serveFile(writer, request, target)
}

// resolveReadPath maps a canonical request path to a regular file. It returns
// either a file path, a redirect Location, or an HTTP error status.
func (server *Server) resolveReadPath(requestPath string) (target, redirect string, status int) {
	segments, trailingSlash, ok := readSegments(requestPath)
	if !ok {
		return "", "", http.StatusNotFound
	}
	if len(segments) == 0 {
		target, status := server.regularFileUnderRoot(filepath.Join(server.stager.publicRoot, "index.html"))
		return target, "", status
	}

	current := server.stager.publicRoot
	for index, segment := range segments {
		current = filepath.Join(current, segment)
		info, err := server.fileSystem.Lstat(current)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return "", "", http.StatusNotFound
			}
			return "", "", http.StatusInternalServerError
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return "", "", http.StatusNotFound
		}
		if index == 0 && strings.HasPrefix(segment, "@") && !info.IsDir() {
			return "", "", http.StatusNotFound
		}
		if index < len(segments)-1 {
			if !info.IsDir() {
				return "", "", http.StatusNotFound
			}
			continue
		}
		if info.IsDir() {
			if !trailingSlash {
				return "", requestPath + "/", 0
			}
			target, status := server.regularFileUnderRoot(filepath.Join(current, "index.html"))
			return target, "", status
		}
		if trailingSlash || !info.Mode().IsRegular() {
			return "", "", http.StatusNotFound
		}
		return current, "", 0
	}
	return "", "", http.StatusNotFound
}

func (server *Server) regularFileUnderRoot(path string) (string, int) {
	info, err := server.fileSystem.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", http.StatusNotFound
		}
		return "", http.StatusInternalServerError
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", http.StatusNotFound
	}
	return path, 0
}

// readSegments splits a canonical request path. It rejects invalid UTF-8,
// control characters, empty segments, dot segments, and backslashes.
func readSegments(requestPath string) (segments []string, trailingSlash, ok bool) {
	if !strings.HasPrefix(requestPath, "/") || !utf8.ValidString(requestPath) {
		return nil, false, false
	}
	for index := range len(requestPath) {
		if requestPath[index] < 0x20 || requestPath[index] == 0x7f {
			return nil, false, false
		}
	}
	if requestPath == "/" {
		return nil, true, true
	}
	raw := strings.TrimPrefix(requestPath, "/")
	trailingSlash = strings.HasSuffix(raw, "/")
	if trailingSlash {
		raw = strings.TrimSuffix(raw, "/")
	}
	if raw == "" {
		return nil, false, false
	}
	segments = strings.Split(raw, "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." || strings.HasPrefix(segment, ".") || strings.Contains(segment, "\\") {
			return nil, false, false
		}
	}
	if strings.HasPrefix(segments[0], "@") && !ValidName(strings.TrimPrefix(segments[0], "@")) {
		return nil, false, false
	}
	if segments[0] == "__pages" {
		return nil, false, false
	}
	return segments, trailingSlash, true
}

func (server *Server) serveFile(writer http.ResponseWriter, request *http.Request, path string) {
	file, err := server.fileSystem.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(writer, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if info.IsDir() || !info.Mode().IsRegular() {
		http.Error(writer, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	extension := strings.ToLower(filepath.Ext(path))
	contentType := ""
	if extension == ".html" || extension == ".htm" {
		contentType = "text/html; charset=utf-8"
	} else {
		contentType = mime.TypeByExtension(extension)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	writer.Header().Set("Content-Type", contentType)
	writer.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	writer.WriteHeader(http.StatusOK)
	if request.Method == http.MethodHead {
		return
	}
	if _, err := io.Copy(writer, file); err != nil {
		server.logger.Printf("serve %s: %v", request.URL.Path, err)
	}
}
