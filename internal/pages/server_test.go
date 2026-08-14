package pages

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/suraciii/pages/internal/filesystem"
)

func TestServerPublishesHTMLUnderVerifiedIdentity(t *testing.T) {
	server, publicRoot := newTestServer(t, namedTestTokens("bumble", "secret-one"), 1024)

	response := publishRequest(t, server, "ticket-status", "bumble.secret-one", []byte("<!doctype html><title>status</title>"))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}

	page, err := server.fileSystem.ReadFile(filepath.Join(publicRoot, "@bumble", "ticket-status", "index.html"))
	if err != nil {
		t.Fatalf("read published page: %v", err)
	}
	if got, want := string(page), "<!doctype html><title>status</title>"; got != want {
		t.Fatalf("page = %q, want %q", got, want)
	}
}

func TestServerSeparatesDefaultAndNamedSameSlug(t *testing.T) {
	server, publicRoot := newTestServer(t, Tokens{
		Token:      "default-secret",
		Identities: map[string]string{"bumble": "secret-one"},
	}, 1024)

	if response := publishRequest(t, server, "overview", "default-secret", []byte("default page")); response.Code != http.StatusNoContent {
		t.Fatalf("default status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if response := publishRequest(t, server, "overview", "bumble.secret-one", []byte("bumble page")); response.Code != http.StatusNoContent {
		t.Fatalf("bumble status = %d, want %d", response.Code, http.StatusNoContent)
	}

	assertPageContent(t, server.fileSystem, publicRoot, "", "overview", "default page")
	assertPageContent(t, server.fileSystem, publicRoot, "bumble", "overview", "bumble page")
}

func TestServerRejectsBadTokenWithoutReplacingExistingPage(t *testing.T) {
	server, publicRoot := newTestServer(t, namedTestTokens("bumble", "secret-one"), 1024)
	if response := publishRequest(t, server, "overview", "bumble.secret-one", []byte("original")); response.Code != http.StatusNoContent {
		t.Fatalf("initial status = %d, want %d", response.Code, http.StatusNoContent)
	}

	response := publishRequest(t, server, "overview", "bumble.wrong-secret", []byte("replacement"))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}

	assertPageContent(t, server.fileSystem, publicRoot, "bumble", "overview", "original")
	assertStagingEmpty(t, server.fileSystem, publicRoot)
}

func TestServerRejectsTokenWithTamperedIdentity(t *testing.T) {
	server, publicRoot := newTestServer(t, Tokens{Identities: map[string]string{
		"bumble": "secret-one",
		"fizz":   "secret-two",
	}}, 1024)
	if response := publishRequest(t, server, "overview", "bumble.secret-one", []byte("original")); response.Code != http.StatusNoContent {
		t.Fatalf("initial status = %d, want %d", response.Code, http.StatusNoContent)
	}

	response := publishRequest(t, server, "overview", "fizz.secret-one", []byte("replacement"))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}

	assertPageContent(t, server.fileSystem, publicRoot, "bumble", "overview", "original")
	if _, err := server.fileSystem.Stat(filepath.Join(publicRoot, "@fizz", "overview", "index.html")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("fizz page exists after rejected identity tampering: %v", err)
	}
}

func TestServerRejectsInvalidUTF8WithoutReplacingExistingPage(t *testing.T) {
	server, publicRoot := newTestServer(t, namedTestTokens("bumble", "secret-one"), 1024)
	if response := publishRequest(t, server, "overview", "bumble.secret-one", []byte("original")); response.Code != http.StatusNoContent {
		t.Fatalf("initial status = %d, want %d", response.Code, http.StatusNoContent)
	}

	response := publishRequest(t, server, "overview", "bumble.secret-one", []byte{0xff, 0xfe})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	assertPageContent(t, server.fileSystem, publicRoot, "bumble", "overview", "original")
	assertStagingEmpty(t, server.fileSystem, publicRoot)
}

func TestServerRejectsOversizedBodyWithoutReplacingExistingPage(t *testing.T) {
	server, publicRoot := newTestServer(t, namedTestTokens("bumble", "secret-one"), 16)
	if response := publishRequest(t, server, "overview", "bumble.secret-one", []byte("original")); response.Code != http.StatusNoContent {
		t.Fatalf("initial status = %d, want %d", response.Code, http.StatusNoContent)
	}

	response := publishRequest(t, server, "overview", "bumble.secret-one", []byte("this body is too large"))
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}

	assertPageContent(t, server.fileSystem, publicRoot, "bumble", "overview", "original")
}

func TestServerRejectsIdentityClaimInUploadPath(t *testing.T) {
	server, _ := newTestServer(t, namedTestTokens("bumble", "secret-one"), 1024)

	request := httptest.NewRequest(http.MethodPost, "/fizz/overview", bytes.NewReader([]byte("page")))
	request.Header.Set("Authorization", "Bearer bumble.secret-one")
	request.Header.Set("Content-Type", "text/html; charset=utf-8")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestServerHealthzNeedsNoAuthentication(t *testing.T) {
	server, _ := newTestServer(t, namedTestTokens("bumble", "secret-one"), 1024)

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestServerReloadTokensSwapsAndKeepsOnFailure(t *testing.T) {
	server, _ := newTestServer(t, namedTestTokens("bumble", "secret-one"), 1024)
	tokensFile := "/config/tokens.json"
	if err := server.fileSystem.MkdirAll(filepath.Dir(tokensFile), 0o700); err != nil {
		t.Fatalf("create config: %v", err)
	}
	if err := server.fileSystem.WriteFile(tokensFile, []byte(`{"identities":{"fizz":"secret-two"}}`), 0o600); err != nil {
		t.Fatalf("write tokens file: %v", err)
	}

	if err := server.ReloadTokens(tokensFile); err != nil {
		t.Fatalf("reload tokens: %v", err)
	}
	if response := publishRequest(t, server, "overview", "bumble.secret-one", []byte("old token")); response.Code != http.StatusUnauthorized {
		t.Fatalf("old token status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if response := publishRequest(t, server, "overview", "fizz.secret-two", []byte("new token")); response.Code != http.StatusNoContent {
		t.Fatalf("new token status = %d, want %d", response.Code, http.StatusNoContent)
	}

	if err := server.fileSystem.WriteFile(tokensFile, []byte("not json"), 0o600); err != nil {
		t.Fatalf("corrupt tokens file: %v", err)
	}
	if err := server.ReloadTokens(tokensFile); err == nil {
		t.Fatal("corrupt reload succeeded, want error")
	}
	if response := publishRequest(t, server, "overview", "fizz.secret-two", []byte("still valid")); response.Code != http.StatusNoContent {
		t.Fatalf("status after failed reload = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestNewServerRecoversDisplacedPageAtStartup(t *testing.T) {
	fileSystem := filesystem.NewMemory("/workspace")
	publicRoot := "/public"
	oldDir := filepath.Join(publicRoot, ".pages", "staging", "@bumble", "old-cafe-report")
	if err := fileSystem.MkdirAll(oldDir, 0o700); err != nil {
		t.Fatalf("create displaced page: %v", err)
	}
	if err := fileSystem.WriteFile(filepath.Join(oldDir, "index.html"), []byte("old page"), 0o644); err != nil {
		t.Fatalf("write displaced page: %v", err)
	}
	if _, err := NewServer(ServerConfig{
		PublicRoot:     publicRoot,
		Tokens:         namedTestTokens("bumble", "secret-one"),
		MaxUploadBytes: 1024,
		FileSystem:     fileSystem,
	}); err != nil {
		t.Fatalf("new server: %v", err)
	}

	assertPageContent(t, fileSystem, publicRoot, "bumble", "report", "old page")
	assertStagingEmpty(t, fileSystem, publicRoot)
}

func TestServerRejectsEmptyChunkedBody(t *testing.T) {
	server, publicRoot := newTestServer(t, namedTestTokens("bumble", "secret-one"), 1024)
	if response := publishRequest(t, server, "overview", "bumble.secret-one", []byte("original")); response.Code != http.StatusNoContent {
		t.Fatalf("initial status = %d, want %d", response.Code, http.StatusNoContent)
	}
	request := httptest.NewRequest(http.MethodPost, "/overview", bytes.NewReader(nil))
	request.Header.Set("Authorization", "Bearer bumble.secret-one")
	request.Header.Set("Content-Type", "text/html; charset=utf-8")
	request.ContentLength = -1
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	assertPageContent(t, server.fileSystem, publicRoot, "bumble", "overview", "original")
	assertStagingEmpty(t, server.fileSystem, publicRoot)
}

func TestUploadContentType(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{value: "text/html", valid: true},
		{value: "text/html; charset=utf-8", valid: true},
		{value: "text/html; charset=UTF-8", valid: true},
		{value: "application/zip", valid: true},
		{value: "text/html; charset=iso-8859-1"},
		{value: "text/html; parameter=value"},
		{value: "text/html; charset=utf-8; parameter=value"},
		{value: "application/zip; parameter=value"},
		{value: "text/plain"},
		{value: "not a content type"},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			if got := isUploadContentType(test.value); got != test.valid {
				t.Fatalf("isUploadContentType(%q) = %t, want %t", test.value, got, test.valid)
			}
		})
	}
}

func newTestServer(t *testing.T, tokens Tokens, maxBytes int64) (*Server, string) {
	t.Helper()
	fileSystem := filesystem.NewMemory("/workspace")
	publicRoot := "/public"
	server, err := NewServer(ServerConfig{
		PublicRoot:     publicRoot,
		Tokens:         tokens,
		MaxUploadBytes: maxBytes,
		FileSystem:     fileSystem,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server, publicRoot
}

func publishRequest(t *testing.T, server *Server, slug, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/"+slug, bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "text/html; charset=utf-8")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func assertPageContent(t *testing.T, fileSystem filesystem.FS, publicRoot, identity, slug, want string) {
	t.Helper()
	page, err := fileSystem.ReadFile(filepath.Join(publicRoot, IdentityScope(identity), slug, "index.html"))
	if err != nil {
		t.Fatalf("read page: %v", err)
	}
	if got := string(page); got != want {
		t.Fatalf("page = %q, want %q", got, want)
	}
}

func assertStagingEmpty(t *testing.T, fileSystem filesystem.FS, publicRoot string) {
	t.Helper()
	staging := filepath.Join(publicRoot, ".pages", "staging")
	entries, err := fileSystem.ReadDir(staging)
	if err != nil {
		t.Fatalf("read staging: %v", err)
	}
	for _, scope := range entries {
		children, err := fileSystem.ReadDir(filepath.Join(staging, scope.Name()))
		if err != nil {
			t.Fatalf("read staging scope %q: %v", scope.Name(), err)
		}
		if len(children) != 0 {
			t.Fatalf("staging scope %q has %d leftovers", scope.Name(), len(children))
		}
	}
}

func namedTestTokens(identity, secret string) Tokens {
	return Tokens{Identities: map[string]string{identity: secret}}
}
