package client

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/suraciii/pages/internal/pages"
)

// Publisher uploads one standalone HTML file and verifies its public URL.
type Publisher struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func (publisher Publisher) Publish(ctx context.Context, filePath, slug string) (string, error) {
	if !pages.ValidName(slug) {
		return "", fmt.Errorf("invalid slug %q", slug)
	}
	identity, err := pages.TokenIdentity(publisher.Token)
	if err != nil {
		return "", err
	}
	baseURL, err := normalizeBaseURL(publisher.BaseURL)
	if err != nil {
		return "", err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open page file: %w", err)
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat page file: %w", err)
	}
	if fileInfo.Size() == 0 {
		return "", fmt.Errorf("page file is empty")
	}

	contentType := "text/html; charset=utf-8"
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".zip":
		contentType = "application/zip"
	case ".html":
	default:
		return "", fmt.Errorf("unsupported page extension %q", filepath.Ext(filePath))
	}

	uploadURL := joinURL(baseURL, "pages", slug)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, file)
	if err != nil {
		return "", fmt.Errorf("build upload request: %w", err)
	}
	request.ContentLength = fileInfo.Size()
	request.Header.Set("Authorization", "Bearer "+publisher.Token)
	request.Header.Set("Content-Type", contentType)

	httpClient := publisher.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("upload page: %w", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return "", fmt.Errorf("upload page: unexpected status %s", response.Status)
	}

	publicURL := joinURL(baseURL, "pages", identity, slug) + "/"
	verifyRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, publicURL, nil)
	if err != nil {
		return "", fmt.Errorf("build verification request: %w", err)
	}
	verification, err := httpClient.Do(verifyRequest)
	if err != nil {
		return "", fmt.Errorf("verify page: %w", err)
	}
	defer verification.Body.Close()
	if verification.StatusCode != http.StatusOK || !isHTML(verification.Header.Get("Content-Type")) {
		return "", fmt.Errorf("verify page: expected 200 text/html, got %s with %q", verification.Status, verification.Header.Get("Content-Type"))
	}
	return publicURL, nil
}

func normalizeBaseURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("base URL must be an absolute HTTP(S) URL")
	}
	return parsed, nil
}

func joinURL(baseURL *url.URL, segments ...string) string {
	copy := *baseURL
	parts := append([]string{copy.Path}, segments...)
	copy.Path = path.Join(parts...)
	copy.RawPath = ""
	return strings.TrimRight(copy.String(), "/")
}

func isHTML(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.EqualFold(mediaType, "text/html")
}
