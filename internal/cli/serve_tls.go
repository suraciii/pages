package cli

import (
	"crypto/tls"
	"sync"
)

// certificateStore holds the active certificate so SIGHUP can replace it
// without replacing the TLS config or the listener.
type certificateStore struct {
	mu          sync.RWMutex
	certificate *tls.Certificate
}

func newCertificateStore(certificatePEM, keyPEM []byte) (*certificateStore, error) {
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &certificateStore{certificate: &certificate}, nil
}

func (store *certificateStore) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.certificate, nil
}

func (store *certificateStore) reload(certificatePEM, keyPEM []byte) error {
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		return err
	}
	store.mu.Lock()
	store.certificate = &certificate
	store.mu.Unlock()
	return nil
}
