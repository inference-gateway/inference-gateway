package client

import (
	"crypto/tls"
	"fmt"
	"net/http"

	otelhttp "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	mtls "github.com/inference-gateway/inference-gateway/internal/platform/mtls"
)

const (
	tlsVersion13         = "TLS13"
	envClientTLSCaPath   = "CLIENT_TLS_CA_PATH"
	envClientTLSCertPath = "CLIENT_TLS_CERT_PATH"
	envClientTLSKeyPath  = "CLIENT_TLS_KEY_PATH"
)

//go:generate mockgen -source=client.go -destination=../../tests/mocks/providers/client.go -package=providersmocks
type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

// SpanNameFormatter names outbound client spans "<METHOD> <host><path>" (e.g.
// "GET api.openai.com/v1/models") instead of otelhttp's default bare method.
// The host keeps a gateway -> provider call distinguishable from the gateway's
// own inbound "<METHOD> <path>" server span for the same path. Provider hosts
// and API paths are a fixed, small set, so the name is cardinality-safe.
func SpanNameFormatter() otelhttp.Option {
	return otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		if r == nil || r.URL == nil || r.URL.Path == "" {
			return r.Method
		}
		return r.Method + " " + r.URL.Host + r.URL.Path
	})
}

type ClientImpl struct {
	scheme   string
	hostname string
	port     string
	client   *http.Client
}

func NewHTTPClient(cfg *ClientConfig, scheme, hostname, port string) (Client, error) {
	tlsConfig, err := newTLSConfig(cfg)
	if err != nil {
		return nil, err
	}

	httpTransport := &http.Transport{
		MaxIdleConns:          cfg.ClientMaxIdleConns,
		MaxIdleConnsPerHost:   cfg.ClientMaxIdleConnsPerHost,
		IdleConnTimeout:       cfg.ClientIdleConnTimeout,
		TLSClientConfig:       tlsConfig,
		ForceAttemptHTTP2:     true,
		DisableCompression:    cfg.ClientDisableCompression,
		ResponseHeaderTimeout: cfg.ClientResponseHeaderTimeout,
		ExpectContinueTimeout: cfg.ClientExpectContinueTimeout,
	}

	httpClient := &http.Client{
		Transport: otelhttp.NewTransport(httpTransport, SpanNameFormatter()),
	}

	return &ClientImpl{
		scheme:   scheme,
		hostname: hostname,
		port:     port,
		client:   httpClient,
	}, nil
}

// newTLSConfig builds the TLS config for provider calls: an extra CA trusted on
// top of the system roots, and a client certificate for providers behind mTLS.
func newTLSConfig(cfg *ClientConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.ClientTlsMinVersion == tlsVersion13 {
		tlsConfig.MinVersion = tls.VersionTLS13
	}

	if cfg.ClientTlsCaPath != "" {
		roots, err := mtls.SystemCertPoolWith(cfg.ClientTlsCaPath)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", envClientTLSCaPath, err)
		}
		tlsConfig.RootCAs = roots
	}

	certificate, err := mtls.KeyPair(cfg.ClientTlsCertPath, cfg.ClientTlsKeyPath)
	if err != nil {
		return nil, fmt.Errorf("%s/%s: %w", envClientTLSCertPath, envClientTLSKeyPath, err)
	}
	if certificate != nil {
		tlsConfig.Certificates = []tls.Certificate{*certificate}
	}

	return tlsConfig, nil
}

func (c *ClientImpl) Do(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme == "" {
		req.URL.Scheme = c.scheme
	}
	if req.URL.Host == "" {
		req.URL.Host = c.hostname + ":" + c.port
	}

	return c.client.Do(req)
}
