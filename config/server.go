package config

import (
	"crypto/tls"
	"fmt"

	mtls "github.com/inference-gateway/inference-gateway/internal/platform/mtls"
)

const (
	envServerTLSCertPath     = "SERVER_TLS_CERT_PATH"
	envServerTLSKeyPath      = "SERVER_TLS_KEY_PATH"
	envServerTLSClientCAPath = "SERVER_TLS_CLIENT_CA_PATH"
)

// serverMinTLSVersion is the floor for inbound connections. The gateway has no
// setting for it, unlike the outbound CLIENT_TLS_MIN_VERSION.
const serverMinTLSVersion = tls.VersionTLS12

// TLSEnabled reports whether the gateway serves HTTPS, which it does once both
// a certificate and its key are configured.
func (s *ServerConfig) TLSEnabled() bool {
	return s != nil && s.TlsCertPath != "" && s.TlsKeyPath != ""
}

// ValidateTLS rejects a half-configured server TLS setup before the gateway
// starts listening, rather than silently serving plaintext or without mTLS.
func (s *ServerConfig) ValidateTLS() error {
	if s == nil {
		return nil
	}
	if err := mtls.ValidateKeyPairPaths(s.TlsCertPath, s.TlsKeyPath); err != nil {
		return fmt.Errorf("%s/%s: %w", envServerTLSCertPath, envServerTLSKeyPath, err)
	}
	if s.TlsClientCaPath != "" && !s.TLSEnabled() {
		return fmt.Errorf("%s requires server tls: set %s and %s", envServerTLSClientCAPath, envServerTLSCertPath, envServerTLSKeyPath)
	}
	return nil
}

// TLSConfig builds the TLS config for the inbound server, loading the
// certificate up front so a bad path fails startup. With a client CA bundle
// configured the gateway requires and verifies a client certificate signed by
// it, so a request without one fails the handshake.
func (s *ServerConfig) TLSConfig() (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: serverMinTLSVersion}

	certificate, err := mtls.KeyPair(s.TlsCertPath, s.TlsKeyPath)
	if err != nil {
		return nil, fmt.Errorf("%s/%s: %w", envServerTLSCertPath, envServerTLSKeyPath, err)
	}
	if certificate != nil {
		cfg.Certificates = []tls.Certificate{*certificate}
	}

	if s.TlsClientCaPath == "" {
		return cfg, nil
	}

	clientCAs, err := mtls.CertPool(s.TlsClientCaPath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", envServerTLSClientCAPath, err)
	}
	cfg.ClientCAs = clientCAs
	cfg.ClientAuth = tls.RequireAndVerifyClientCert
	return cfg, nil
}

// defaultMaxRequestBodySize is the fallback body-size limit used when
// MaxRequestBodySize is unset (e.g. a config built without env parsing).
const defaultMaxRequestBodySize = 10 << 20 // 10 MiB

// ResolveMaxRequestBodySize returns the configured request body limit, falling
// back to defaultMaxRequestBodySize when unset or non-positive. It is the single
// source of the fallback shared by every handler that reads a request body.
func (s *ServerConfig) ResolveMaxRequestBodySize() int {
	if s != nil && s.MaxRequestBodySize > 0 {
		return s.MaxRequestBodySize
	}
	return defaultMaxRequestBodySize
}
