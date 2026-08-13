package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/suraciii/pages/internal/pages"
)

func TestPublisherUploadsAndVerifiesIdentityPage(t *testing.T) {
	var uploadedBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodPost:
			if got, want := request.URL.Path, "/pages/ticket-status"; got != want {
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
			writer.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			if got, want := request.URL.Path, "/pages/bumble/ticket-status/"; got != want {
				t.Errorf("verify path = %q, want %q", got, want)
			}
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			writer.WriteHeader(http.StatusOK)
		default:
			writer.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	filePath := filepath.Join(t.TempDir(), "page.html")
	if err := os.WriteFile(filePath, []byte("<!doctype html><h1>Ticket</h1>"), 0o600); err != nil {
		t.Fatalf("write page: %v", err)
	}
	publisher := Publisher{BaseURL: server.URL, Token: "bumble.secret-one", HTTPClient: server.Client()}

	publicURL, err := publisher.Publish(context.Background(), filePath, "ticket-status")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if got, want := publicURL, server.URL+"/pages/bumble/ticket-status/"; got != want {
		t.Fatalf("public URL = %q, want %q", got, want)
	}
	if got, want := uploadedBody, "<!doctype html><h1>Ticket</h1>"; got != want {
		t.Fatalf("uploaded body = %q, want %q", got, want)
	}
}

func TestPublisherFailsWhenPublicPageIsNotHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		writer.Header().Set("Content-Type", "text/plain")
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	filePath := filepath.Join(t.TempDir(), "page.html")
	if err := os.WriteFile(filePath, []byte("page"), 0o600); err != nil {
		t.Fatalf("write page: %v", err)
	}
	publisher := Publisher{BaseURL: server.URL, Token: "bumble.secret-one", HTTPClient: server.Client()}

	if _, err := publisher.Publish(context.Background(), filePath, "ticket-status"); err == nil {
		t.Fatal("publish succeeded, want verification error")
	}
}

func TestPublisherUploadsZipWithZipContentType(t *testing.T) {
	var uploadedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodPost:
			uploadedContentType = request.Header.Get("Content-Type")
			writer.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			writer.WriteHeader(http.StatusOK)
		default:
			writer.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	filePath := filepath.Join(t.TempDir(), "page.zip")
	if err := os.WriteFile(filePath, []byte("zip bytes"), 0o600); err != nil {
		t.Fatalf("write page: %v", err)
	}
	publisher := Publisher{BaseURL: server.URL, Token: "bumble.secret-one", HTTPClient: server.Client()}

	publicURL, err := publisher.Publish(context.Background(), filePath, "ticket-status")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if got, want := uploadedContentType, "application/zip"; got != want {
		t.Fatalf("content type = %q, want %q", got, want)
	}
	if got, want := publicURL, server.URL+"/pages/bumble/ticket-status/"; got != want {
		t.Fatalf("public URL = %q, want %q", got, want)
	}
}

func TestPublisherRejectsUnsupportedExtension(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "page.txt")
	if err := os.WriteFile(filePath, []byte("page"), 0o600); err != nil {
		t.Fatalf("write page: %v", err)
	}
	publisher := Publisher{BaseURL: "https://pages.example.com", Token: "bumble.secret-one"}

	if _, err := publisher.Publish(context.Background(), filePath, "ticket-status"); err == nil {
		t.Fatal("publish succeeded, want extension error")
	}
}

func TestPublisherPublishesThroughServerAndStaticRoute(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	pageServer, err := pages.NewServer(pages.ServerConfig{
		PublicRoot:     publicRoot,
		Tokens:         pages.Tokens{"bumble": "secret-one"},
		MaxUploadBytes: 1024,
	})
	if err != nil {
		t.Fatalf("new pages server: %v", err)
	}
	staticPages := http.StripPrefix("/pages", http.FileServer(http.Dir(publicRoot)))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			pageServer.ServeHTTP(writer, request)
			return
		}
		staticPages.ServeHTTP(writer, request)
	}))
	defer server.Close()

	filePath := filepath.Join(root, "page.html")
	if err := os.WriteFile(filePath, []byte("<!doctype html><h1>Published</h1>"), 0o600); err != nil {
		t.Fatalf("write page: %v", err)
	}
	publisher := Publisher{BaseURL: server.URL, Token: "bumble.secret-one", HTTPClient: server.Client()}

	publicURL, err := publisher.Publish(context.Background(), filePath, "overview")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if got, want := publicURL, server.URL+"/pages/bumble/overview/"; got != want {
		t.Fatalf("public URL = %q, want %q", got, want)
	}
	page, err := os.ReadFile(filepath.Join(publicRoot, "bumble", "overview", "index.html"))
	if err != nil {
		t.Fatalf("read published page: %v", err)
	}
	if got, want := string(page), "<!doctype html><h1>Published</h1>"; got != want {
		t.Fatalf("page = %q, want %q", got, want)
	}
}
