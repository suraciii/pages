package cli

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/suraciii/pages/internal/filesystem"
	"github.com/suraciii/pages/internal/pages"
)

func TestReloadInputsKeepsFailedInputAndAppliesOther(t *testing.T) {
	fileSystem := filesystem.NewMemory("/workspace")
	firstCertificate, firstKey := testCertificate(t, "first.example")
	secondCertificate, secondKey := testCertificate(t, "second.example")
	for path, data := range map[string][]byte{
		"/tokens.json": []byte(`{"token":"old-secret"}`),
		"/cert.pem":    firstCertificate,
		"/key.pem":     firstKey,
	} {
		if err := fileSystem.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	pageServer, err := pages.NewServer(pages.ServerConfig{
		PublicRoot:     "/public",
		Tokens:         pages.Tokens{Token: "old-secret"},
		MaxUploadBytes: 1024,
		FileSystem:     fileSystem,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	certificates, err := newCertificateStore(firstCertificate, firstKey)
	if err != nil {
		t.Fatalf("new certificate store: %v", err)
	}
	secondStore, err := newCertificateStore(secondCertificate, secondKey)
	if err != nil {
		t.Fatalf("new second certificate store: %v", err)
	}
	secondLoaded, err := secondStore.getCertificate(nil)
	if err != nil {
		t.Fatalf("get expected certificate: %v", err)
	}
	logger := log.New(io.Discard, "", 0)

	// A failed token reload keeps the old Tokens while the certificate reload
	// still applies.
	if err := fileSystem.WriteFile("/tokens.json", []byte("not json"), 0o600); err != nil {
		t.Fatalf("corrupt tokens: %v", err)
	}
	if err := fileSystem.WriteFile("/cert.pem", secondCertificate, 0o600); err != nil {
		t.Fatalf("write second certificate: %v", err)
	}
	if err := fileSystem.WriteFile("/key.pem", secondKey, 0o600); err != nil {
		t.Fatalf("write second key: %v", err)
	}
	reloadInputs(logger, fileSystem, pageServer, "/tokens.json", certificates, "/cert.pem", "/key.pem")

	certificate, err := certificates.getCertificate(nil)
	if err != nil {
		t.Fatalf("get certificate: %v", err)
	}
	if string(certificate.Certificate[0]) != string(secondLoaded.Certificate[0]) {
		t.Fatal("certificate reload did not apply")
	}
	if status := uploadWithToken(pageServer, "old-secret", "old-token-page"); status != http.StatusNoContent {
		t.Fatalf("old token status = %d after failed reload", status)
	}

	// A successful token reload applies while a failed certificate reload keeps
	// the previous certificate.
	if err := fileSystem.WriteFile("/tokens.json", []byte(`{"token":"new-secret"}`), 0o600); err != nil {
		t.Fatalf("write new tokens: %v", err)
	}
	if err := fileSystem.WriteFile("/cert.pem", []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("corrupt certificate: %v", err)
	}
	if err := fileSystem.WriteFile("/key.pem", []byte("not a key"), 0o600); err != nil {
		t.Fatalf("corrupt key: %v", err)
	}
	reloadInputs(logger, fileSystem, pageServer, "/tokens.json", certificates, "/cert.pem", "/key.pem")

	certificate, err = certificates.getCertificate(nil)
	if err != nil {
		t.Fatalf("get certificate after failed reload: %v", err)
	}
	if string(certificate.Certificate[0]) != string(secondLoaded.Certificate[0]) {
		t.Fatal("failed certificate reload replaced the certificate")
	}
	if status := uploadWithToken(pageServer, "old-secret", "old-token-page-2"); status != http.StatusUnauthorized {
		t.Fatalf("old token status = %d after successful reload", status)
	}
	if status := uploadWithToken(pageServer, "new-secret", "new-token-page"); status != http.StatusNoContent {
		t.Fatalf("new token status = %d after successful reload", status)
	}
}

func uploadWithToken(server http.Handler, token, slug string) int {
	request := httptest.NewRequest(http.MethodPost, "/"+slug, strings.NewReader("page"))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "text/html; charset=utf-8")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response.Code
}
