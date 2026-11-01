package client

import (
	"crypto/tls"
	"fmt"
	"net/http"

	otelhttp "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	inprocess "github.com/inference-gateway/inference-gateway/internal/platform/inprocess"
	mtls "github.com/inference-gateway/inference-gateway/internal/platform/mtls"
)

const (
	selfScheme           = "http"
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
	hostname string
	port     string
	self     *http.Client
	upstream *http.Client
}

// NewHTTPClient builds the client for provider calls. A request with a host goes
// to it with the outbound TLS settings. A request without one is the gateway
// calling its own /proxy route, which reaches gateway in process, so the server's
// TLS and client certificates never apply to it. hostname and port only name it.
func NewHTTPClient(cfg *ClientConfig, gateway *inprocess.Listener, hostname, port string) (Client, error) {
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

	selfTransport := httpTransport.Clone()
	selfTransport.DialContext = gateway.DialContext

	return &ClientImpl{
		hostname: hostname,
		port:     port,
		self:     &http.Client{Transport: otelhttp.NewTransport(selfTransport, SpanNameFormatter())},
		upstream: &http.Client{Transport: otelhttp.NewTransport(httpTransport, SpanNameFormatter())},
	}, nil
}

// NewProxyTransport builds the round tripper the reverse proxy uses to reach a
// provider, so a provider call carries the same outbound TLS settings as a
// request made through NewHTTPClient.
func NewProxyTransport(cfg *ClientConfig) (http.RoundTripper, error) {
	tlsConfig, err := newTLSConfig(cfg)
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig

	return otelhttp.NewTransport(transport, SpanNameFormatter()), nil
}

// newTLSConfig builds the TLS config for provider calls: an extra CA trusted on
// top of the system roots, and a client certificate for providers behind mTLS.
func newTLSConfig(cfg *ClientConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg == nil {
		return tlsConfig, nil
	}
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
	if req.URL.Host != "" {
		return c.upstream.Do(req)
	}
	req.URL.Scheme = selfScheme
	req.URL.Host = c.hostname + ":" + c.port

	return c.self.Do(req)
}
