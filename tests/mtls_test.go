package tests

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	api "github.com/inference-gateway/inference-gateway/api"
	config "github.com/inference-gateway/inference-gateway/config"
	inprocess "github.com/inference-gateway/inference-gateway/internal/platform/inprocess"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	client "github.com/inference-gateway/inference-gateway/providers/client"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	testCertValidity = time.Hour
	testCertHost     = "127.0.0.1"
)

type testCA struct {
	certificate *x509.Certificate
	key         *ecdsa.PrivateKey
	pemPath     string
}

func newTestCA(t *testing.T, dir, name string) *testCA {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-testCertValidity),
		NotAfter:              time.Now().Add(testCertValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	certificate, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	pemPath := filepath.Join(dir, name+"-ca.crt")
	require.NoError(t, os.WriteFile(pemPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))

	return &testCA{certificate: certificate, key: key, pemPath: pemPath}
}

func (ca *testCA) issue(t *testing.T, dir, name string, usage x509.ExtKeyUsage) (certificate tls.Certificate, certPath, keyPath string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-testCertValidity),
		NotAfter:     time.Now().Add(testCertValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		IPAddresses:  []net.IP{net.ParseIP(testCertHost)},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.certificate, &key.PublicKey, ca.key)
	require.NoError(t, err)

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	certPath = filepath.Join(dir, name+".crt")
	keyPath = filepath.Join(dir, name+".key")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))

	certificate, err = tls.LoadX509KeyPair(certPath, keyPath)
	require.NoError(t, err)
	certificate.Certificate = append(certificate.Certificate, ca.certificate.Raw)

	return certificate, certPath, keyPath
}

// TestServerMutualTLS covers the inbound side: with SERVER_TLS_CLIENT_CA_PATH set
// the gateway requires a client certificate signed by that CA.
func TestServerMutualTLS(t *testing.T) {
	dir := t.TempDir()
	clientsCA := newTestCA(t, dir, "clients")
	serverCA := newTestCA(t, dir, "server")
	_, serverCertPath, serverKeyPath := serverCA.issue(t, dir, "server", x509.ExtKeyUsageServerAuth)
	clientCert, _, _ := clientsCA.issue(t, dir, "client", x509.ExtKeyUsageClientAuth)
	otherCert, _, _ := serverCA.issue(t, dir, "other-client", x509.ExtKeyUsageClientAuth)

	cfg := &config.ServerConfig{
		TlsCertPath:     serverCertPath,
		TlsKeyPath:      serverKeyPath,
		TlsClientCaPath: clientsCA.pemPath,
	}
	serverTLS, err := cfg.TLSConfig()
	require.NoError(t, err)
	assert.Equal(t, tls.RequireAndVerifyClientCert, serverTLS.ClientAuth, "a client ca makes the client certificate mandatory")
	require.Len(t, serverTLS.Certificates, 1, "the server certificate is loaded at startup")

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server.TLS = serverTLS
	server.StartTLS()
	defer server.Close()

	roots := x509.NewCertPool()
	roots.AddCert(serverCA.certificate)

	tests := []struct {
		name         string
		certificates []tls.Certificate
		expectError  bool
	}{
		{name: "certificate signed by the client ca is accepted", certificates: []tls.Certificate{clientCert}},
		{name: "certificate from another ca is rejected", certificates: []tls.Certificate{otherCert}, expectError: true},
		{name: "no client certificate is rejected", expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
				MinVersion:   tls.VersionTLS12,
				RootCAs:      roots,
				Certificates: tt.certificates,
			}}}
			defer httpClient.CloseIdleConnections()

			resp, err := httpClient.Get(server.URL)
			if tt.expectError {
				require.Error(t, err, "the tls handshake must fail")
				return
			}
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

// TestProviderBehindMutualTLS covers the outbound side: a provider that requires
// a client certificate and is signed by a private CA.
func TestProviderBehindMutualTLS(t *testing.T) {
	dir := t.TempDir()
	providersCA := newTestCA(t, dir, "providers")
	providerCert, _, _ := providersCA.issue(t, dir, "provider", x509.ExtKeyUsageServerAuth)
	_, clientCertPath, clientKeyPath := providersCA.issue(t, dir, "gateway-client", x509.ExtKeyUsageClientAuth)

	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(providersCA.certificate)

	provider := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	provider.TLS = &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{providerCert},
		ClientCAs:    clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	provider.StartTLS()
	defer provider.Close()

	tests := []struct {
		name        string
		cfg         *client.ClientConfig
		expectError bool
	}{
		{
			name: "extra ca and client certificate reach the provider",
			cfg: &client.ClientConfig{
				ClientTlsCaPath:   providersCA.pemPath,
				ClientTlsCertPath: clientCertPath,
				ClientTlsKeyPath:  clientKeyPath,
			},
		},
		{
			name:        "without a client certificate the provider rejects the handshake",
			cfg:         &client.ClientConfig{ClientTlsCaPath: providersCA.pemPath},
			expectError: true,
		},
		{
			name:        "without the extra ca the provider is untrusted",
			cfg:         &client.ClientConfig{ClientTlsCertPath: clientCertPath, ClientTlsKeyPath: clientKeyPath},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpClient, err := client.NewHTTPClient(tt.cfg, inprocess.NewListener(), testCertHost, "0")
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodGet, provider.URL, nil)
			require.NoError(t, err)

			resp, err := httpClient.Do(req)
			if tt.expectError {
				require.Error(t, err, "the tls handshake must fail")
				return
			}
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

// TestProxyToProviderBehindMutualTLS covers the path a provider call actually
// takes out of the gateway: the reverse proxy behind /proxy/{provider}.
func TestProxyToProviderBehindMutualTLS(t *testing.T) {
	dir := t.TempDir()
	providersCA := newTestCA(t, dir, "providers")
	providerCert, _, _ := providersCA.issue(t, dir, "provider", x509.ExtKeyUsageServerAuth)
	_, clientCertPath, clientKeyPath := providersCA.issue(t, dir, "gateway-client", x509.ExtKeyUsageClientAuth)

	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(providersCA.certificate)

	provider := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	provider.TLS = &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{providerCert},
		ClientCAs:    clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	provider.StartTLS()
	defer provider.Close()

	tests := []struct {
		name       string
		cfg        *client.ClientConfig
		statusCode int
	}{
		{
			name: "extra ca and client certificate reach the provider",
			cfg: &client.ClientConfig{
				ClientTlsCaPath:   providersCA.pemPath,
				ClientTlsCertPath: clientCertPath,
				ClientTlsKeyPath:  clientKeyPath,
			},
			statusCode: http.StatusOK,
		},
		{
			name:       "without a client certificate the provider rejects the handshake",
			cfg:        &client.ClientConfig{ClientTlsCaPath: providersCA.pemPath},
			statusCode: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, err := logger.NewLogger("test")
			require.NoError(t, err)

			providerCfg := map[types.Provider]*registry.ProviderConfig{
				constants.OpenaiID: {
					ID:       constants.OpenaiID,
					Name:     constants.OpenaiDisplayName,
					URL:      provider.URL,
					Token:    "test-key",
					AuthType: constants.AuthTypeBearer,
				},
			}
			cfg := config.Config{
				Server:    &config.ServerConfig{},
				Client:    tt.cfg,
				Providers: providerCfg,
			}
			router := api.NewRouter(cfg, log, registry.NewProviderRegistry(providerCfg, log), nil, nil, nil, nil, nil, nil)

			mux := http.NewServeMux()
			mux.HandleFunc("/proxy/{provider}/{path...}", router.ProxyHandler)
			gateway := httptest.NewServer(mux)
			defer gateway.Close()

			resp, err := http.Get(gateway.URL + "/proxy/openai/models")
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, tt.statusCode, resp.StatusCode)
		})
	}
}

// TestProviderCallsSkipTheServerTLS points the provider client at the gateway's
// own mTLS listener, with a server certificate and client settings that could
// never pass that listener's handshake. The /proxy hop still lists the models,
// because it reaches the gateway in process.
func TestProviderCallsSkipTheServerTLS(t *testing.T) {
	dir := t.TempDir()
	clientsCA := newTestCA(t, dir, "clients")
	serverCA := newTestCA(t, dir, "server")
	_, serverCertPath, serverKeyPath := serverCA.issue(t, dir, "server", x509.ExtKeyUsageServerAuth)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-4o","object":"model","created":0,"owned_by":"openai"}]}`))
	}))
	defer upstream.Close()

	log, err := logger.NewLogger("test")
	require.NoError(t, err)
	providerCfg := map[types.Provider]*registry.ProviderConfig{
		constants.OpenaiID: {
			ID:       constants.OpenaiID,
			Name:     constants.OpenaiDisplayName,
			URL:      upstream.URL,
			Token:    "test-key",
			AuthType: constants.AuthTypeBearer,
		},
	}
	serverCfg := &config.ServerConfig{TlsCertPath: serverCertPath, TlsKeyPath: serverKeyPath, TlsClientCaPath: clientsCA.pemPath}
	cfg := config.Config{Server: serverCfg, Client: &client.ClientConfig{}, Providers: providerCfg}
	providers := registry.NewProviderRegistry(providerCfg, log)

	selfListener := inprocess.NewListener()
	mux := http.NewServeMux()
	gateway := httptest.NewUnstartedServer(mux)
	gateway.TLS, err = serverCfg.TLSConfig()
	require.NoError(t, err)
	gateway.StartTLS()
	defer gateway.Close()
	selfServer := &http.Server{Handler: mux}
	defer selfServer.Close()
	go func() { _ = selfServer.Serve(selfListener) }()

	host, port, err := net.SplitHostPort(gateway.Listener.Addr().String())
	require.NoError(t, err)
	httpClient, err := client.NewHTTPClient(cfg.Client, selfListener, host, port)
	require.NoError(t, err)
	router := api.NewRouter(cfg, log, providers, httpClient, nil, nil, nil, nil, nil)
	mux.HandleFunc("/proxy/{provider}/{path...}", router.ProxyHandler)

	provider, err := providers.BuildProvider(constants.OpenaiID, httpClient)
	require.NoError(t, err)
	models, err := provider.ListModels(t.Context())
	require.NoError(t, err)
	require.Len(t, models.Data, 1)
	assert.Equal(t, "openai/gpt-4o", models.Data[0].ID)
}

// TestClientTLSConfigErrors covers the startup validation of the outbound
// settings, which happens when the provider http client is built.
func TestClientTLSConfigErrors(t *testing.T) {
	dir := t.TempDir()
	ca := newTestCA(t, dir, "providers")
	_, certPath, keyPath := ca.issue(t, dir, "gateway-client", x509.ExtKeyUsageClientAuth)
	emptyPath := filepath.Join(dir, "empty.pem")
	require.NoError(t, os.WriteFile(emptyPath, []byte("not a certificate"), 0o600))

	tests := []struct {
		name        string
		cfg         *client.ClientConfig
		errContains string
	}{
		{
			name:        "certificate without its key",
			cfg:         &client.ClientConfig{ClientTlsCertPath: certPath},
			errContains: "CLIENT_TLS_CERT_PATH/CLIENT_TLS_KEY_PATH",
		},
		{
			name:        "key without its certificate",
			cfg:         &client.ClientConfig{ClientTlsKeyPath: keyPath},
			errContains: "CLIENT_TLS_CERT_PATH/CLIENT_TLS_KEY_PATH",
		},
		{
			name:        "missing ca bundle",
			cfg:         &client.ClientConfig{ClientTlsCaPath: filepath.Join(dir, "absent.crt")},
			errContains: "CLIENT_TLS_CA_PATH",
		},
		{
			name:        "ca bundle without certificates",
			cfg:         &client.ClientConfig{ClientTlsCaPath: emptyPath},
			errContains: "contains no certificates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpClient, err := client.NewHTTPClient(tt.cfg, inprocess.NewListener(), testCertHost, "0")
			require.Error(t, err)
			assert.Nil(t, httpClient)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}
