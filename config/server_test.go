package config_test

import (
	"crypto/tls"
	"testing"

	assert "github.com/stretchr/testify/assert"

	config "github.com/inference-gateway/inference-gateway/config"
)

// TestResolveMaxRequestBodySize verifies the configured limit is honored and the
// default kicks in for unset/non-positive values (e.g. a config built without env parsing).
func TestResolveMaxRequestBodySize(t *testing.T) {
	const defaultSize = 10 << 20 // 10 MiB
	assert.Equal(t, defaultSize, (&config.ServerConfig{MaxRequestBodySize: 0}).ResolveMaxRequestBodySize(), "unset falls back to default")
	assert.Equal(t, defaultSize, (&config.ServerConfig{MaxRequestBodySize: -1}).ResolveMaxRequestBodySize(), "negative falls back to default")
	assert.Equal(t, 1234, (&config.ServerConfig{MaxRequestBodySize: 1234}).ResolveMaxRequestBodySize(), "configured value is honored")
	assert.Equal(t, defaultSize, (*config.ServerConfig)(nil).ResolveMaxRequestBodySize(), "nil receiver falls back to default")
}

// TestValidateTLS verifies a half-configured server TLS setup is rejected at startup.
func TestValidateTLS(t *testing.T) {
	const certPath = "/etc/gateway/server.crt"
	const keyPath = "/etc/gateway/server.key"
	const clientCAPath = "/etc/gateway/clients-ca.crt"

	tests := []struct {
		name        string
		server      *config.ServerConfig
		errContains string
	}{
		{name: "no tls at all", server: &config.ServerConfig{}},
		{name: "certificate and key", server: &config.ServerConfig{TlsCertPath: certPath, TlsKeyPath: keyPath}},
		{
			name:   "mutual tls",
			server: &config.ServerConfig{TlsCertPath: certPath, TlsKeyPath: keyPath, TlsClientCaPath: clientCAPath},
		},
		{
			name:        "certificate without its key",
			server:      &config.ServerConfig{TlsCertPath: certPath},
			errContains: "SERVER_TLS_CERT_PATH/SERVER_TLS_KEY_PATH",
		},
		{
			name:        "key without its certificate",
			server:      &config.ServerConfig{TlsKeyPath: keyPath},
			errContains: "SERVER_TLS_CERT_PATH/SERVER_TLS_KEY_PATH",
		},
		{
			name:        "client ca without server tls",
			server:      &config.ServerConfig{TlsClientCaPath: clientCAPath},
			errContains: "SERVER_TLS_CLIENT_CA_PATH requires server tls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.server.ValidateTLS()
			if tt.errContains == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.errContains)
		})
	}
}

// TestTLSConfigWithoutClientCA verifies plain TLS does not ask for a client certificate.
func TestTLSConfigWithoutClientCA(t *testing.T) {
	tlsConfig, err := (&config.ServerConfig{}).TLSConfig()
	assert.NoError(t, err)
	assert.Equal(t, tls.NoClientCert, tlsConfig.ClientAuth)
	assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
}
