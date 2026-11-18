package pages

import (
	"bytes"
	"io/fs"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/suraciii/pages/internal/filesystem"
)

func readRequest(t *testing.T, server *Server, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func TestServerServesPageAssetsAndSecurityHeaders(t *testing.T) {
	server, publicRoot := newTestServer(t, namedTestTokens("bumble", "secret-one"), 1024)
	publishResponse := publishRequest(t, server, "overview", "bumble.secret-one", []byte("<!doctype html><h1>Overview</h1>"))
	if publishResponse.Code != http.StatusNoContent {
		t.Fatalf("publish status = %d", publishResponse.Code)
	}
	if got := publishResponse.Header().Get("Content-Security-Policy"); got != DefaultContentSecurityPolicy {
		t.Fatalf("upload CSP = %q", got)
	}
	if err := server.fileSystem.WriteFile(filepath.Join(publicRoot, "@bumble", "overview", "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	wantPage := "<!doctype html><h1>Overview</h1>"
	page := readRequest(t, server, http.MethodGet, "/@bumble/overview/")
	if page.Code != http.StatusOK || page.Body.String() != wantPage {
		t.Fatalf("page status = %d body = %q", page.Code, page.Body.String())
	}
	if got := page.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("page content type = %q", got)
	}
	if got := page.Header().Get("Content-Length"); got != strconv.Itoa(len(wantPage)) {
		t.Fatalf("page content length = %q", got)
	}
	for header, want := range map[string]string{
		"Content-Security-Policy": DefaultContentSecurityPolicy,
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"X-Frame-Options":         "DENY",
		"Cache-Control":           "no-store",
	} {
		if got := page.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}

	asset := readRequest(t, server, http.MethodGet, "/@bumble/overview/app.js")
	if asset.Code != http.StatusOK || asset.Body.String() != "console.log(1)" {
		t.Fatalf("asset status = %d body = %q", asset.Code, asset.Body.String())
	}
	if got := asset.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/javascript") && !strings.HasPrefix(got, "application/javascript") {
		t.Fatalf("asset content type = %q", got)
	}

	head := readRequest(t, server, http.MethodHead, "/@bumble/overview/")
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD status = %d body = %q", head.Code, head.Body.String())
	}
	if got := head.Header().Get("Content-Length"); got != strconv.Itoa(len(wantPage)) {
		t.Fatalf("HEAD content length = %q", got)
	}
}

func TestServerServesNamedIdentityIndexAndZipAssets(t *testing.T) {
	server, _ := newTestServer(t, namedTestTokens("bumble", "secret-one"), 1024)
	body := zipBody(t,
		fileEntry("index.html", "<h1>zip page</h1>"),
		fileEntry("asset.css", "body { color: black }"),
	)
	request := httptest.NewRequest(http.MethodPost, "/zip-page", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer bumble.secret-one")
	request.Header.Set("Content-Type", "application/zip")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("zip publish status = %d", response.Code)
	}

	index := readRequest(t, server, http.MethodGet, "/@bumble/")
	if index.Code != http.StatusOK || !strings.HasPrefix(index.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("identity index status = %d content type = %q", index.Code, index.Header().Get("Content-Type"))
	}

	page := readRequest(t, server, http.MethodGet, "/@bumble/zip-page/")
	if page.Code != http.StatusOK || page.Body.String() != "<h1>zip page</h1>" {
		t.Fatalf("zip page status = %d body = %q", page.Code, page.Body.String())
	}
	asset := readRequest(t, server, http.MethodGet, "/@bumble/zip-page/asset.css")
	if asset.Code != http.StatusOK || asset.Body.String() != "body { color: black }" || !strings.HasPrefix(asset.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("zip asset status = %d content type = %q body = %q", asset.Code, asset.Header().Get("Content-Type"), asset.Body.String())
	}
}

func TestServerServesRootIndexAndRedirectsDirectories(t *testing.T) {
	server, _ := newTestServer(t, namedTestTokens("bumble", "secret-one"), 1024)
	root := readRequest(t, server, http.MethodGet, "/")
	if root.Code != http.StatusOK || root.Body.Len() == 0 {
		t.Fatalf("root status = %d body = %q", root.Code, root.Body.String())
	}

	redirect := readRequest(t, server, http.MethodGet, "/index.html")
	if redirect.Code != http.StatusOK {
		t.Fatalf("root file status = %d, want %d", redirect.Code, http.StatusOK)
	}

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Host = "attacker.example"
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing page status = %d, want %d", response.Code, http.StatusNotFound)
	}

	server.fileSystem.MkdirAll("/public/healthz", 0o755)
	server.fileSystem.WriteFile("/public/healthz/index.html", []byte("health page"), 0o644)
	redirect = readRequest(t, server, http.MethodGet, "/healthz")
	if redirect.Code != http.StatusMovedPermanently || redirect.Header().Get("Location") != "/healthz/" {
		t.Fatalf("redirect status = %d location = %q", redirect.Code, redirect.Header().Get("Location"))
	}
	if strings.Contains(redirect.Header().Get("Location"), "attacker.example") {
		t.Fatalf("redirect used Host: %q", redirect.Header().Get("Location"))
	}
	if got := redirect.Header().Get("Content-Security-Policy"); got != DefaultContentSecurityPolicy {
		t.Fatalf("redirect CSP = %q", got)
	}
	page := readRequest(t, server, http.MethodGet, "/healthz/")
	if page.Code != http.StatusOK || page.Body.String() != "health page" {
		t.Fatalf("health page status = %d body = %q", page.Code, page.Body.String())
	}
}

func TestServerRejectsNonCanonicalAndDotPaths(t *testing.T) {
	server, _ := newTestServer(t, namedTestTokens("bumble", "secret-one"), 1024)
	for _, path := range []string{
		"/.pages",
		"/.pages/staging",
		"/../index.html",
		"/%2e%2e/index.html",
		"/report/../index.html",
		"/report//index.html",
		"/report\\index.html",
		"/__pages/healthz/",
		"/__pages/other",
		"/@Bad/",
		"/@bumble/../index.html",
		"/report/%00",
	} {
		t.Run(path, func(t *testing.T) {
			response := readRequest(t, server, http.MethodGet, path)
			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
			}
			if got := response.Header().Get("Content-Security-Policy"); got != DefaultContentSecurityPolicy {
				t.Fatalf("404 CSP = %q", got)
			}
		})
	}

	for name, path := range map[string]string{
		"invalid UTF-8": "/" + string([]byte{0xff}),
		"control":       "/report/\x01",
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.URL.Path = path
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
			}
		})
	}
}

func TestServerServesPageRecords(t *testing.T) {
	server, _ := newTestServer(t, Tokens{
		Token:      "default-secret",
		Identities: map[string]string{"bumble": "secret-one"},
	}, 1024)
	for _, token := range []string{"default-secret", "bumble.secret-one"} {
		if response := publishRequest(t, server, "report", token, []byte("page")); response.Code != http.StatusNoContent {
			t.Fatalf("publish %q status = %d", token, response.Code)
		}
	}
	for _, path := range []string{"/index.json", "/@bumble/index.json"} {
		record := readRequest(t, server, http.MethodGet, path)
		if record.Code != http.StatusOK || !strings.Contains(record.Body.String(), `"slug": "report"`) {
			t.Fatalf("%s status = %d body = %q", path, record.Code, record.Body.String())
		}
		if !strings.HasPrefix(record.Header().Get("Content-Type"), "application/json") {
			t.Fatalf("%s content type = %q", path, record.Header().Get("Content-Type"))
		}
	}
}

func TestServerReadRepresentation(t *testing.T) {
	server, publicRoot := newTestServer(t, namedTestTokens("bumble", "secret-one"), 1024)
	if response := publishRequest(t, server, "report", "bumble.secret-one", []byte("page")); response.Code != http.StatusNoContent {
		t.Fatalf("publish status = %d", response.Code)
	}
	if err := server.fileSystem.MkdirAll(filepath.Join(publicRoot, "empty"), 0o755); err != nil {
		t.Fatalf("create empty directory: %v", err)
	}
	if err := server.fileSystem.WriteFile(filepath.Join(publicRoot, "@bumble", "report", "data.unknown-ext"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write unknown asset: %v", err)
	}

	if response := readRequest(t, server, http.MethodGet, "/empty/"); response.Code != http.StatusNotFound {
		t.Fatalf("empty directory status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if response := readRequest(t, server, http.MethodGet, "/@bumble/report/index.html/"); response.Code != http.StatusNotFound {
		t.Fatalf("file with trailing slash status = %d, want %d", response.Code, http.StatusNotFound)
	}

	unknown := readRequest(t, server, http.MethodGet, "/@bumble/report/data.unknown-ext")
	if unknown.Code != http.StatusOK || unknown.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("unknown asset status = %d content type = %q", unknown.Code, unknown.Header().Get("Content-Type"))
	}
	for _, header := range []string{"Content-Encoding", "Accept-Ranges", "ETag", "Last-Modified"} {
		if got := unknown.Header().Get(header); got != "" {
			t.Fatalf("unexpected %s = %q", header, got)
		}
	}
	for header, value := range map[string]string{
		"Range":         "bytes=0-1",
		"If-None-Match": `"old"`,
	} {
		request := httptest.NewRequest(http.MethodGet, "/@bumble/report/", nil)
		request.Header.Set(header, value)
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Body.String() != "page" {
			t.Fatalf("%s status = %d body = %q", header, response.Code, response.Body.String())
		}
	}
}

func TestServerHealthHeadHasSecurityHeaders(t *testing.T) {
	server, _ := newTestServer(t, namedTestTokens("bumble", "secret-one"), 1024)
	response := readRequest(t, server, http.MethodHead, "/__pages/healthz")
	if response.Code != http.StatusOK || response.Body.Len() != 0 {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Security-Policy"); got != DefaultContentSecurityPolicy {
		t.Fatalf("health CSP = %q", got)
	}
}

func TestServerRejectsSymlinksAndSpecialFilesBelowPublicRoot(t *testing.T) {
	publicRoot := "/public"
	fileSystem := filesystem.NewMemory("/workspace")
	for _, directory := range []string{"report", "alias"} {
		if err := fileSystem.MkdirAll(filepath.Join(publicRoot, directory), 0o755); err != nil {
			t.Fatalf("create %s: %v", directory, err)
		}
		if err := fileSystem.WriteFile(filepath.Join(publicRoot, directory, "index.html"), []byte(directory), 0o644); err != nil {
			t.Fatalf("write %s page: %v", directory, err)
		}
	}
	if err := fileSystem.WriteFile(filepath.Join(publicRoot, "report", "secret"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	if err := fileSystem.WriteFile(filepath.Join(publicRoot, "report", "fifo"), []byte("pipe"), 0o644); err != nil {
		t.Fatalf("write special file: %v", err)
	}
	server, err := NewServer(ServerConfig{
		PublicRoot:     publicRoot,
		Tokens:         namedTestTokens("bumble", "secret-one"),
		MaxUploadBytes: 1024,
		FileSystem: specialFileSystem{
			FS: fileSystem,
			special: map[string]fs.FileMode{
				"/public/report/secret": fs.ModeSymlink,
				"/public/report/fifo":   fs.ModeNamedPipe,
				"/public/alias":         fs.ModeSymlink,
			},
		},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	if response := readRequest(t, server, http.MethodGet, "/report/"); response.Code != http.StatusOK {
		t.Fatalf("real page status = %d", response.Code)
	}
	for _, path := range []string{"/report/secret", "/report/fifo", "/alias/", "/alias/index.html"} {
		if response := readRequest(t, server, http.MethodGet, path); response.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want %d", path, response.Code, http.StatusNotFound)
		}
	}
}

type specialFileSystem struct {
	filesystem.FS
	special map[string]fs.FileMode
}

func (fileSystem specialFileSystem) Lstat(path string) (fs.FileInfo, error) {
	if mode, found := fileSystem.special[path]; found {
		return specialFileInfo{mode: mode}, nil
	}
	return fileSystem.FS.Lstat(path)
}

type specialFileInfo struct {
	mode fs.FileMode
}

func (specialFileInfo) Name() string           { return "special" }
func (specialFileInfo) Size() int64            { return 0 }
func (info specialFileInfo) Mode() fs.FileMode { return info.mode }
func (specialFileInfo) ModTime() time.Time     { return time.Unix(0, 0) }
func (specialFileInfo) IsDir() bool            { return false }
func (specialFileInfo) Sys() any               { return nil }

func TestNewServerLogsRefreshFailureAndStillServesHealth(t *testing.T) {
	fileSystem := filesystem.NewMemory("/workspace")
	if err := fileSystem.MkdirAll("/public", 0o755); err != nil {
		t.Fatalf("create public root: %v", err)
	}
	if err := fileSystem.WriteFile("/public/index.html", []byte("manual index"), 0o644); err != nil {
		t.Fatalf("write foreign index: %v", err)
	}
	var logs bytes.Buffer
	server, err := NewServer(ServerConfig{
		PublicRoot:     "/public",
		MaxUploadBytes: 1024,
		FileSystem:     fileSystem,
		Logger:         log.New(&logs, "", 0),
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if !strings.Contains(logs.String(), "refresh page index") {
		t.Fatalf("startup log = %q", logs.String())
	}
	if response := readRequest(t, server, http.MethodGet, "/__pages/healthz"); response.Code != http.StatusOK {
		t.Fatalf("health status = %d", response.Code)
	}
}

func TestServerRejectsMultilineContentSecurityPolicy(t *testing.T) {
	_, err := NewServer(ServerConfig{
		PublicRoot:            "/public",
		MaxUploadBytes:        1024,
		ContentSecurityPolicy: "default-src 'none'\nscript-src *",
		FileSystem:            filesystem.NewMemory("/workspace"),
	})
	if err == nil {
		t.Fatal("multiline policy accepted")
	}
}

func TestServerUsesCustomContentSecurityPolicy(t *testing.T) {
	fileSystem := filesystem.NewMemory("/workspace")
	server, err := NewServer(ServerConfig{
		PublicRoot:            "/public",
		Tokens:                namedTestTokens("bumble", "secret-one"),
		MaxUploadBytes:        1024,
		ContentSecurityPolicy: "default-src 'none'; script-src 'self'",
		FileSystem:            fileSystem,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	response := readRequest(t, server, http.MethodGet, "/")
	if got := response.Header().Get("Content-Security-Policy"); got != "default-src 'none'; script-src 'self'" {
		t.Fatalf("CSP = %q", got)
	}
}

func TestServerMethodNotAllowedAndEmptyTokens(t *testing.T) {
	fileSystem := filesystem.NewMemory("/workspace")
	server, err := NewServer(ServerConfig{
		PublicRoot:     "/public",
		MaxUploadBytes: 1024,
		FileSystem:     fileSystem,
	})
	if err != nil {
		t.Fatalf("new server without tokens: %v", err)
	}
	response := readRequest(t, server, http.MethodPut, "/report")
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("method status = %d allow = %q", response.Code, response.Header().Get("Allow"))
	}

	upload := publishRequest(t, server, "report", "any-secret", []byte("page"))
	if upload.Code != http.StatusUnauthorized {
		t.Fatalf("upload status = %d, want %d", upload.Code, http.StatusUnauthorized)
	}
}

func TestServerReadDoesNotChangePublicRoot(t *testing.T) {
	server, publicRoot := newTestServer(t, namedTestTokens("bumble", "secret-one"), 1024)
	if response := publishRequest(t, server, "report", "bumble.secret-one", []byte("page")); response.Code != http.StatusNoContent {
		t.Fatalf("publish status = %d", response.Code)
	}
	rootIndex := filepath.Join(publicRoot, "index.html")
	rootRecord := filepath.Join(publicRoot, "index.json")
	indexBefore, err := server.fileSystem.ReadFile(rootIndex)
	if err != nil {
		t.Fatalf("read root index: %v", err)
	}
	recordBefore, err := server.fileSystem.ReadFile(rootRecord)
	if err != nil {
		t.Fatalf("read root record: %v", err)
	}
	for _, path := range []string{"/", "/index.json", "/@bumble/report/", "/@bumble/report/index.html"} {
		if response := readRequest(t, server, http.MethodGet, path); response.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, response.Code)
		}
	}
	indexAfter, err := server.fileSystem.ReadFile(rootIndex)
	if err != nil || string(indexAfter) != string(indexBefore) {
		t.Fatalf("root index changed after reads: %v", err)
	}
	recordAfter, err := server.fileSystem.ReadFile(rootRecord)
	if err != nil || string(recordAfter) != string(recordBefore) {
		t.Fatalf("root record changed after reads: %v", err)
	}
}

func TestServerDoesNotListDirectories(t *testing.T) {
	server, _ := newTestServer(t, namedTestTokens("bumble", "secret-one"), 1024)
	if response := readRequest(t, server, http.MethodGet, "/missing/"); response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	root := readRequest(t, server, http.MethodGet, "/")
	if bytes.Contains(root.Body.Bytes(), []byte(".pages")) {
		t.Fatalf("root index exposes .pages: %s", root.Body.String())
	}
}
