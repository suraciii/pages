package pages

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type zipEntry struct {
	name    string
	content string
	mode    os.FileMode
	isDir   bool
}

func fileEntry(name, content string) zipEntry {
	return zipEntry{name: name, content: content}
}

func dirEntry(name string) zipEntry {
	return zipEntry{name: name, isDir: true}
}

func symlinkEntry(name string) zipEntry {
	return zipEntry{name: name, mode: os.ModeSymlink}
}

func zipBody(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		switch {
		case entry.isDir:
			header.SetMode(os.ModeDir | 0o755)
		case entry.mode == os.ModeSymlink:
			header.SetMode(os.ModeSymlink)
		default:
			header.SetMode(0o644)
		}
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", entry.name, err)
		}
		if _, err := io.WriteString(file, entry.content); err != nil {
			t.Fatalf("write zip entry %q: %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buffer.Bytes()
}

func publishZipRequest(t *testing.T, server *Server, slug, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/"+slug, bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/zip")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func assertStagingEmpty(t *testing.T, publicRoot string) {
	t.Helper()
	staging := filepath.Join(publicRoot, ".pages", "staging")
	leftovers := 0
	_ = filepath.WalkDir(staging, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == staging {
			return nil
		}
		if entry.IsDir() {
			children, readError := os.ReadDir(path)
			if readError == nil && len(children) == 0 {
				return nil
			}
		}
		leftovers++
		return nil
	})
	if leftovers != 0 {
		t.Fatalf("staging has %d leftover entries", leftovers)
	}
}

func TestServerPublishesZipDirectoryPage(t *testing.T) {
	server, publicRoot := newTestServer(t, Tokens{"bumble": "secret-one"}, 4096)
	body := zipBody(t, fileEntry("index.html", "<h1>report</h1>"), fileEntry("img/chart.png", "png"))

	response := publishZipRequest(t, server, "report", "bumble.secret-one", body)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}

	page, err := os.ReadFile(filepath.Join(publicRoot, "bumble", "report", "index.html"))
	if err != nil || string(page) != "<h1>report</h1>" {
		t.Fatalf("index.html = %q, %v", page, err)
	}
	asset, err := os.ReadFile(filepath.Join(publicRoot, "bumble", "report", "img", "chart.png"))
	if err != nil || string(asset) != "png" {
		t.Fatalf("chart.png = %q, %v", asset, err)
	}
	assertStagingEmpty(t, publicRoot)
}

func TestServerPublishesZipSkippingDirectoryEntries(t *testing.T) {
	server, publicRoot := newTestServer(t, Tokens{"bumble": "secret-one"}, 4096)
	body := zipBody(t, fileEntry("index.html", "page"), dirEntry("img/"), fileEntry("img/chart.png", "png"))

	response := publishZipRequest(t, server, "report", "bumble.secret-one", body)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if _, err := os.Stat(filepath.Join(publicRoot, "bumble", "report", "img", "chart.png")); err != nil {
		t.Fatalf("extracted asset missing: %v", err)
	}
}

func TestServerRejectsZipWithoutRootIndexHTML(t *testing.T) {
	server, publicRoot := newTestServer(t, Tokens{"bumble": "secret-one"}, 4096)
	if response := publishRequest(t, server, "report", "bumble.secret-one", []byte("original")); response.Code != http.StatusNoContent {
		t.Fatalf("initial status = %d", response.Code)
	}

	body := zipBody(t, fileEntry("report.html", "page"))
	response := publishZipRequest(t, server, "report", "bumble.secret-one", body)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	assertPageContent(t, publicRoot, "bumble", "report", "original")
	assertStagingEmpty(t, publicRoot)
}

func TestServerRejectsZipWithSymlinkEntry(t *testing.T) {
	server, _ := newTestServer(t, Tokens{"bumble": "secret-one"}, 4096)
	body := zipBody(t, fileEntry("index.html", "page"), symlinkEntry("link"))

	response := publishZipRequest(t, server, "report", "bumble.secret-one", body)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestServerRejectsZipWithBadNames(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "a\\b", ".hidden", "a/../b"} {
		t.Run(name, func(t *testing.T) {
			server, _ := newTestServer(t, Tokens{"bumble": "secret-one"}, 4096)
			body := zipBody(t, fileEntry("index.html", "page"), fileEntry(name, "x"))

			response := publishZipRequest(t, server, "report", "bumble.secret-one", body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestServerRejectsZipWithNonUTF8Name(t *testing.T) {
	server, _ := newTestServer(t, Tokens{"bumble": "secret-one"}, 4096)
	body := zipBody(t, fileEntry("index.html", "page"), fileEntry("bad\xffname", "x"))

	response := publishZipRequest(t, server, "report", "bumble.secret-one", body)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestServerRejectsZipWithDuplicateNames(t *testing.T) {
	server, _ := newTestServer(t, Tokens{"bumble": "secret-one"}, 4096)
	body := zipBody(t, fileEntry("index.html", "page"), fileEntry("index.html", "again"))

	response := publishZipRequest(t, server, "report", "bumble.secret-one", body)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestServerRejectsZipWithTooManyEntries(t *testing.T) {
	server, _ := newTestServer(t, Tokens{"bumble": "secret-one"}, 1<<20)
	entries := []zipEntry{fileEntry("index.html", "page")}
	for i := 0; i < maxZipEntries; i++ {
		entries = append(entries, fileEntry("asset-"+strings.Repeat("x", i)+"-"+string(rune('a'+i%26)), "x"))
	}
	body := zipBody(t, entries...)

	response := publishZipRequest(t, server, "report", "bumble.secret-one", body)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestServerRejectsZipWithOversizedUncompressedTotal(t *testing.T) {
	server, _ := newTestServer(t, Tokens{"bumble": "secret-one"}, 512)
	compressed := strings.Repeat("a", 2000)
	body := zipBody(t,
		fileEntry("index.html", compressed),
		fileEntry("asset.bin", compressed),
		fileEntry("data.bin", compressed),
	)

	response := publishZipRequest(t, server, "report", "bumble.secret-one", body)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestServerReplacesDirectoryPage(t *testing.T) {
	server, publicRoot := newTestServer(t, Tokens{"bumble": "secret-one"}, 4096)
	first := zipBody(t, fileEntry("index.html", "version one"), fileEntry("img/a.png", "a"))
	second := zipBody(t, fileEntry("index.html", "version two"), fileEntry("css/style.css", "b"))

	if response := publishZipRequest(t, server, "report", "bumble.secret-one", first); response.Code != http.StatusNoContent {
		t.Fatalf("first status = %d", response.Code)
	}
	if response := publishZipRequest(t, server, "report", "bumble.secret-one", second); response.Code != http.StatusNoContent {
		t.Fatalf("second status = %d", response.Code)
	}

	page, err := os.ReadFile(filepath.Join(publicRoot, "bumble", "report", "index.html"))
	if err != nil || string(page) != "version two" {
		t.Fatalf("index.html = %q, %v", page, err)
	}
	if _, err := os.Stat(filepath.Join(publicRoot, "bumble", "report", "img", "a.png")); !os.IsNotExist(err) {
		t.Fatalf("stale asset from first version still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(publicRoot, "bumble", "report", "css", "style.css")); err != nil {
		t.Fatalf("asset from second version missing: %v", err)
	}
	assertStagingEmpty(t, publicRoot)
}

func TestServerConcurrentZipPublishesSameSlug(t *testing.T) {
	server, publicRoot := newTestServer(t, Tokens{"bumble": "secret-one"}, 4096)
	first := zipBody(t, fileEntry("index.html", "first"), fileEntry("a.txt", "a"))
	second := zipBody(t, fileEntry("index.html", "second"), fileEntry("b.txt", "b"))

	results := make(chan *httptest.ResponseRecorder, 2)
	go func() {
		results <- publishZipRequest(t, server, "report", "bumble.secret-one", first)
	}()
	go func() {
		results <- publishZipRequest(t, server, "report", "bumble.secret-one", second)
	}()
	for range 2 {
		if response := <-results; response.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
		}
	}

	page, err := os.ReadFile(filepath.Join(publicRoot, "bumble", "report", "index.html"))
	if err != nil {
		t.Fatalf("read page: %v", err)
	}
	content := string(page)
	if content != "first" && content != "second" {
		t.Fatalf("page = %q, want first or second", content)
	}
	assertStagingEmpty(t, publicRoot)
}
