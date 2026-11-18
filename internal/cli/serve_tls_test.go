package cli

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func testCertificate(t *testing.T, commonName string) ([]byte, []byte) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certificatePEM, keyPEM
}

func TestCertificateStoreReloadsAndKeepsPreviousOnFailure(t *testing.T) {
	firstCertificate, firstKey := testCertificate(t, "first.example")
	secondCertificate, secondKey := testCertificate(t, "second.example")
	store, err := newCertificateStore(firstCertificate, firstKey)
	if err != nil {
		t.Fatalf("new certificate store: %v", err)
	}
	first, err := store.getCertificate(nil)
	if err != nil {
		t.Fatalf("get first certificate: %v", err)
	}
	if err := store.reload(secondCertificate, secondKey); err != nil {
		t.Fatalf("reload certificate: %v", err)
	}
	second, err := store.getCertificate(nil)
	if err != nil {
		t.Fatalf("get second certificate: %v", err)
	}
	if second == first || string(second.Certificate[0]) == string(first.Certificate[0]) {
		t.Fatal("certificate was not replaced")
	}
	if err := store.reload([]byte("not a certificate"), []byte("not a key")); err == nil {
		t.Fatal("invalid reload succeeded")
	}
	stillSecond, err := store.getCertificate(nil)
	if err != nil {
		t.Fatalf("get certificate after failed reload: %v", err)
	}
	if string(stillSecond.Certificate[0]) != string(second.Certificate[0]) {
		t.Fatal("failed reload replaced the certificate")
	}
}
