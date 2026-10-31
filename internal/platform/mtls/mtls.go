package mtls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

// ErrCertWithoutKey and ErrKeyWithoutCert report a half-configured key pair:
// one of the two paths is set and the other is not.
var (
	ErrCertWithoutKey = errors.New("certificate path is set without its key path")
	ErrKeyWithoutCert = errors.New("key path is set without its certificate path")
)

// CertPool reads a PEM CA bundle into a pool of its own, trusting nothing else.
func CertPool(path string) (*x509.CertPool, error) {
	return appendBundle(x509.NewCertPool(), path)
}

// SystemCertPoolWith reads a PEM CA bundle into a copy of the system roots, so
// the extra CA is trusted on top of the public ones rather than instead of them.
func SystemCertPoolWith(path string) (*x509.CertPool, error) {
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("reading system cert pool: %w", err)
	}
	return appendBundle(roots, path)
}

// ValidateKeyPairPaths rejects a half-configured key pair without touching the
// filesystem, so a misconfiguration can be reported before any file is read.
func ValidateKeyPairPaths(certPath, keyPath string) error {
	switch {
	case certPath != "" && keyPath == "":
		return ErrCertWithoutKey
	case keyPath != "" && certPath == "":
		return ErrKeyWithoutCert
	}
	return nil
}

// KeyPair loads a certificate and its key, returning nil when neither path is
// set. Exactly one of the two paths is a configuration error.
func KeyPair(certPath, keyPath string) (*tls.Certificate, error) {
	if err := ValidateKeyPairPaths(certPath, keyPath); err != nil {
		return nil, err
	}
	if certPath == "" {
		return nil, nil
	}

	certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("loading key pair %s: %w", certPath, err)
	}
	return &certificate, nil
}

func appendBundle(pool *x509.CertPool, path string) (*x509.CertPool, error) {
	bundle, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading ca bundle %s: %w", path, err)
	}
	if !pool.AppendCertsFromPEM(bundle) {
		return nil, fmt.Errorf("ca bundle %s contains no certificates", path)
	}
	return pool, nil
}
