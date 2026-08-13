package pages

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestServerPublishesHTMLUnderVerifiedIdentity(t *testing.T) {
	server, publicRoot := newTestServer(t, Tokens{"bumble": "secret-one"}, 1024)

	response := publishRequest(t, server, "ticket-status", "bumble.secret-one", []byte("<!doctype html><title>status</title>"))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}

	page, err := os.ReadFile(filepath.Join(publicRoot, "bumble", "ticket-status", "index.html"))
	if err != nil {
		t.Fatalf("read published page: %v", err)
	}
	if got, want := string(page), "<!doctype html><title>status</title>"; got != want {
		t.Fatalf("page = %q, want %q", got, want)
	}
}

func TestServerSeparatesSameSlugByIdentity(t *testing.T) {
	server, publicRoot := newTestServer(t, Tokens{
		"bumble": "secret-one",
		"fizz":   "secret-two",
	}, 1024)

	if response := publishRequest(t, server, "overview", "bumble.secret-one", []byte("bumble page")); response.Code != http.StatusNoContent {
		t.Fatalf("bumble status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if response := publishRequest(t, server, "overview", "fizz.secret-two", []byte("fizz page")); response.Code != http.StatusNoContent {
		t.Fatalf("fizz status = %d, want %d", response.Code, http.StatusNoContent)
	}

	for identity, want := range map[string]string{"bumble": "bumble page", "fizz": "fizz page"} {
		page, err := os.ReadFile(filepath.Join(publicRoot, identity, "overview", "index.html"))
		if err != nil {
			t.Fatalf("read %s page: %v", identity, err)
		}
		if got := string(page); got != want {
			t.Fatalf("%s page = %q, want %q", identity, got, want)
		}
	}
}

func TestServerRejectsBadTokenWithoutReplacingExistingPage(t *testing.T) {
	server, publicRoot := newTestServer(t, Tokens{"bumble": "secret-one"}, 1024)
	if response := publishRequest(t, server, "overview", "bumble.secret-one", []byte("original")); response.Code != http.StatusNoContent {
		t.Fatalf("initial status = %d, want %d", response.Code, http.StatusNoContent)
	}

	response := publishRequest(t, server, "overview", "bumble.wrong-secret", []byte("replacement"))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}

	assertPageContent(t, publicRoot, "bumble", "overview", "original")
}

func TestServerRejectsTokenWithTamperedIdentity(t *testing.T) {
	server, publicRoot := newTestServer(t, Tokens{
		"bumble": "secret-one",
		"fizz":   "secret-two",
	}, 1024)
	if response := publishRequest(t, server, "overview", "bumble.secret-one", []byte("original")); response.Code != http.StatusNoContent {
		t.Fatalf("initial status = %d, want %d", response.Code, http.StatusNoContent)
	}

	response := publishRequest(t, server, "overview", "fizz.secret-one", []byte("replacement"))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}

	assertPageContent(t, publicRoot, "bumble", "overview", "original")
	if _, err := os.Stat(filepath.Join(publicRoot, "fizz", "overview", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("fizz page exists after rejected identity tampering: %v", err)
	}
}

func TestServerRejectsInvalidUTF8WithoutReplacingExistingPage(t *testing.T) {
	server, publicRoot := newTestServer(t, Tokens{"bumble": "secret-one"}, 1024)
	if response := publishRequest(t, server, "overview", "bumble.secret-one", []byte("original")); response.Code != http.StatusNoContent {
		t.Fatalf("initial status = %d, want %d", response.Code, http.StatusNoContent)
	}

	response := publishRequest(t, server, "overview", "bumble.secret-one", []byte{0xff, 0xfe})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	assertPageContent(t, publicRoot, "bumble", "overview", "original")
}

func TestServerRejectsOversizedBodyWithoutReplacingExistingPage(t *testing.T) {
	server, publicRoot := newTestServer(t, Tokens{"bumble": "secret-one"}, 16)
	if response := publishRequest(t, server, "overview", "bumble.secret-one", []byte("original")); response.Code != http.StatusNoContent {
		t.Fatalf("initial status = %d, want %d", response.Code, http.StatusNoContent)
	}

	response := publishRequest(t, server, "overview", "bumble.secret-one", []byte("this body is too large"))
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}

	assertPageContent(t, publicRoot, "bumble", "overview", "original")
}

func TestServerRejectsIdentityClaimInUploadPath(t *testing.T) {
	server, _ := newTestServer(t, Tokens{"bumble": "secret-one"}, 1024)

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
	server, _ := newTestServer(t, Tokens{"bumble": "secret-one"}, 1024)

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestServerReloadTokensSwapsAndKeepsOnFailure(t *testing.T) {
	server, _ := newTestServer(t, Tokens{"bumble": "secret-one"}, 1024)
	tokensFile := filepath.Join(t.TempDir(), "tokens.json")
	if err := os.WriteFile(tokensFile, []byte(`{"fizz": "secret-two"}`), 0o600); err != nil {
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

	if err := os.WriteFile(tokensFile, []byte("not json"), 0o600); err != nil {
		t.Fatalf("corrupt tokens file: %v", err)
	}
	if err := server.ReloadTokens(tokensFile); err == nil {
		t.Fatal("corrupt reload succeeded, want error")
	}
	if response := publishRequest(t, server, "overview", "fizz.secret-two", []byte("still valid")); response.Code != http.StatusNoContent {
		t.Fatalf("status after failed reload = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestServerRejectsEmptyChunkedBody(t *testing.T) {
	server, _ := newTestServer(t, Tokens{"bumble": "secret-one"}, 1024)
	request := httptest.NewRequest(http.MethodPost, "/overview", bytes.NewReader(nil))
	request.Header.Set("Authorization", "Bearer bumble.secret-one")
	request.Header.Set("Content-Type", "text/html; charset=utf-8")
	request.ContentLength = -1
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func newTestServer(t *testing.T, tokens Tokens, maxBytes int64) (*Server, string) {
	t.Helper()
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	server, err := NewServer(ServerConfig{
		PublicRoot:     publicRoot,
		Tokens:         tokens,
		MaxUploadBytes: maxBytes,
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

func assertPageContent(t *testing.T, publicRoot, identity, slug, want string) {
	t.Helper()
	page, err := os.ReadFile(filepath.Join(publicRoot, identity, slug, "index.html"))
	if err != nil {
		t.Fatalf("read page: %v", err)
	}
	if got := string(page); got != want {
		t.Fatalf("page = %q, want %q", got, want)
	}
}
