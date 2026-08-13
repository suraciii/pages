package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suraciii/pages/internal/filesystem"
	"github.com/suraciii/pages/internal/pages"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func testHTTPClient(handler func(*http.Request) *http.Response) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return handler(request), nil
	})}
}

func response(status int, contentType string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

func memoryPage(t *testing.T, path, content string) *filesystem.Memory {
	t.Helper()
	fileSystem := filesystem.NewMemory("/workspace")
	if err := fileSystem.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if err := fileSystem.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write page: %v", err)
	}
	return fileSystem
}

func TestPublisherUploadsAndVerifiesIdentityPage(t *testing.T) {
	filePath := "/source/page.html"
	fileSystem := memoryPage(t, filePath, "<!doctype html><h1>Ticket</h1>")
	var uploadedBody string
	client := testHTTPClient(func(request *http.Request) *http.Response {
		switch request.Method {
		case http.MethodPost:
			if got, want := request.URL.Path, "/ticket-status"; got != want {
				t.Errorf("upload path = %q, want %q", got, want)
			}
			if got, want := request.Header.Get("Authorization"), "Bearer bumble.secret-one"; got != want {
				t.Errorf("authorization = %q, want %q", got, want)
			}
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Errorf("read body: %v", err)
			}
			uploadedBody = string(body)
			return response(http.StatusNoContent, "")
		case http.MethodGet:
			if got, want := request.URL.Path, "/@bumble/ticket-status/"; got != want {
				t.Errorf("verify path = %q, want %q", got, want)
			}
			return response(http.StatusOK, "text/html; charset=utf-8")
		default:
			return response(http.StatusMethodNotAllowed, "")
		}
	})
	publisher := Publisher{BaseURL: "https://pages.example.com", Token: "bumble.secret-one", HTTPClient: client, FileSystem: fileSystem}

	publicURL, err := publisher.Publish(context.Background(), filePath, "ticket-status")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if got, want := publicURL, "https://pages.example.com/@bumble/ticket-status/"; got != want {
		t.Fatalf("public URL = %q, want %q", got, want)
	}
	if got, want := uploadedBody, "<!doctype html><h1>Ticket</h1>"; got != want {
		t.Fatalf("uploaded body = %q, want %q", got, want)
	}
}

func TestPublisherFailsWhenPublicPageIsNotHTML(t *testing.T) {
	fileSystem := memoryPage(t, "/source/page.html", "page")
	client := testHTTPClient(func(request *http.Request) *http.Response {
		if request.Method == http.MethodPost {
			return response(http.StatusNoContent, "")
		}
		return response(http.StatusOK, "text/plain")
	})
	publisher := Publisher{BaseURL: "https://pages.example.com", Token: "bumble.secret-one", HTTPClient: client, FileSystem: fileSystem}

	if _, err := publisher.Publish(context.Background(), "/source/page.html", "ticket-status"); err == nil {
		t.Fatal("publish succeeded, want verification error")
	}
}

func TestPublisherUploadsZipWithZipContentType(t *testing.T) {
	fileSystem := memoryPage(t, "/source/page.zip", "zip bytes")
	var uploadedContentType string
	client := testHTTPClient(func(request *http.Request) *http.Response {
		if request.Method == http.MethodPost {
			uploadedContentType = request.Header.Get("Content-Type")
			return response(http.StatusNoContent, "")
		}
		return response(http.StatusOK, "text/html; charset=utf-8")
	})
	publisher := Publisher{BaseURL: "https://pages.example.com", Token: "bumble.secret-one", HTTPClient: client, FileSystem: fileSystem}

	publicURL, err := publisher.Publish(context.Background(), "/source/page.zip", "ticket-status")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if uploadedContentType != "application/zip" {
		t.Fatalf("content type = %q", uploadedContentType)
	}
	if publicURL != "https://pages.example.com/@bumble/ticket-status/" {
		t.Fatalf("public URL = %q", publicURL)
	}
}

func TestPublisherRejectsUnsupportedExtension(t *testing.T) {
	fileSystem := memoryPage(t, "/source/page.txt", "page")
	publisher := Publisher{BaseURL: "https://pages.example.com", Token: "bumble.secret-one", FileSystem: fileSystem}
	if _, err := publisher.Publish(context.Background(), "/source/page.txt", "ticket-status"); err == nil {
		t.Fatal("publish succeeded, want extension error")
	}
}

func TestNormalizeBaseURLRejectsQueryAndFragment(t *testing.T) {
	for _, rawURL := range []string{
		"https://pages.example.com?tenant=one",
		"https://pages.example.com/docs#upload",
	} {
		if _, err := normalizeBaseURL(rawURL); err == nil {
			t.Fatalf("normalizeBaseURL(%q) succeeded", rawURL)
		}
	}
}

func TestNormalizeBaseURLAcceptsCaseInsensitiveHTTPS(t *testing.T) {
	parsed, err := normalizeBaseURL("HTTPS://pages.example.com/docs")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if parsed.Host != "pages.example.com" || parsed.Path != "/docs" {
		t.Fatalf("parsed URL = %s", parsed)
	}
}

func TestPublisherPublishesThroughServerAndStaticRoute(t *testing.T) {
	fileSystem := memoryPage(t, "/source/page.html", "<!doctype html><h1>Published</h1>")
	publicRoot := "/public"
	pageServer, err := pages.NewServer(pages.ServerConfig{
		PublicRoot:     publicRoot,
		Tokens:         pages.Tokens{Identities: map[string]string{"bumble": "secret-one"}},
		MaxUploadBytes: 1024,
		FileSystem:     fileSystem,
	})
	if err != nil {
		t.Fatalf("new pages server: %v", err)
	}
	client := inProcessPagesClient(t, pageServer, fileSystem, publicRoot)
	publisher := Publisher{BaseURL: "https://pages.example.com", Token: "bumble.secret-one", HTTPClient: client, FileSystem: fileSystem}

	publicURL, err := publisher.Publish(context.Background(), "/source/page.html", "overview")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if publicURL != "https://pages.example.com/@bumble/overview/" {
		t.Fatalf("public URL = %q", publicURL)
	}
	page, err := fileSystem.ReadFile("/public/@bumble/overview/index.html")
	if err != nil || string(page) != "<!doctype html><h1>Published</h1>" {
		t.Fatalf("page = %q, %v", page, err)
	}
}

func TestPublisherPublishesDefaultAndNamedThroughOneOrigin(t *testing.T) {
	fileSystem := filesystem.NewMemory("/workspace")
	publicRoot := "/public"
	pageServer, err := pages.NewServer(pages.ServerConfig{
		PublicRoot: publicRoot,
		Tokens: pages.Tokens{
			Token:      "default-secret",
			Identities: map[string]string{"bumble": "secret-one"},
		},
		MaxUploadBytes: 1024,
		FileSystem:     fileSystem,
	})
	if err != nil {
		t.Fatalf("new pages server: %v", err)
	}
	client := inProcessPagesClient(t, pageServer, fileSystem, publicRoot)

	for token, wantURL := range map[string]string{
		"default-secret":    "https://pages.example.com/report/",
		"bumble.secret-one": "https://pages.example.com/@bumble/report/",
	} {
		filePath := filepath.Join("/source", strings.ReplaceAll(token, ".", "-")+".html")
		if err := fileSystem.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			t.Fatalf("create source: %v", err)
		}
		if err := fileSystem.WriteFile(filePath, []byte("<!doctype html><h1>Published</h1>"), 0o644); err != nil {
			t.Fatalf("write page: %v", err)
		}
		publicURL, err := (Publisher{BaseURL: "https://pages.example.com", Token: token, HTTPClient: client, FileSystem: fileSystem}).Publish(context.Background(), filePath, "report")
		if err != nil {
			t.Fatalf("publish %q: %v", token, err)
		}
		if publicURL != wantURL {
			t.Fatalf("public URL = %q, want %q", publicURL, wantURL)
		}
	}
}

func TestPublisherVerifiesDefaultIdentityPage(t *testing.T) {
	fileSystem := memoryPage(t, "/source/page.html", "page")
	client := testHTTPClient(func(request *http.Request) *http.Response {
		if request.Method == http.MethodPost {
			if got := request.Header.Get("Authorization"); got != "Bearer default-secret" {
				t.Errorf("authorization = %q", got)
			}
			return response(http.StatusNoContent, "")
		}
		if got, want := request.URL.Path, "/report/"; got != want {
			t.Errorf("verify path = %q, want %q", got, want)
		}
		return response(http.StatusOK, "text/html")
	})
	publicURL, err := (Publisher{BaseURL: "https://pages.example.com", Token: "default-secret", HTTPClient: client, FileSystem: fileSystem}).Publish(context.Background(), "/source/page.html", "report")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if publicURL != "https://pages.example.com/report/" {
		t.Fatalf("public URL = %q", publicURL)
	}
}

func inProcessPagesClient(t *testing.T, server http.Handler, fileSystem filesystem.FS, publicRoot string) *http.Client {
	t.Helper()
	return testHTTPClient(func(request *http.Request) *http.Response {
		if request.Method == http.MethodPost {
			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, request)
			return recorder.Result()
		}
		path := filepath.Join(publicRoot, filepath.FromSlash(strings.TrimPrefix(request.URL.Path, "/")), "index.html")
		body, err := fileSystem.ReadFile(path)
		if err != nil {
			return response(http.StatusNotFound, "text/plain")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}
	})
}
