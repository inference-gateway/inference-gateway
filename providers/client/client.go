package client

import (
	"crypto/tls"
	"io"
	"net/http"
	"strings"

	otelhttp "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

//go:generate mockgen -source=client.go -destination=../../tests/mocks/providers/client.go -package=providersmocks
type Client interface {
	Do(req *http.Request) (*http.Response, error)
	Get(url string) (*http.Response, error)
	Post(url string, bodyType string, body string) (*http.Response, error)
}

// SpanNameFormatter names outbound client spans "<METHOD> <path>" (e.g.
// "GET /v1/models") instead of otelhttp's default bare method, matching how
// otelgin names inbound server spans. Provider API paths are a fixed, small
// set, so using the path as a span name is cardinality-safe.
func SpanNameFormatter() otelhttp.Option {
	return otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		if r == nil || r.URL == nil || r.URL.Path == "" {
			return r.Method
		}
		return r.Method + " " + r.URL.Path
	})
}

type ClientImpl struct {
	scheme   string
	hostname string
	port     string
	client   *http.Client
}

func NewHTTPClient(cfg *ClientConfig, scheme, hostname, port string) Client {
	var tlsMinVersion uint16 = tls.VersionTLS12
	if cfg.ClientTlsMinVersion == "TLS13" {
		tlsMinVersion = tls.VersionTLS13
	}

	httpTransport := &http.Transport{
		MaxIdleConns:        cfg.ClientMaxIdleConns,
		MaxIdleConnsPerHost: cfg.ClientMaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.ClientIdleConnTimeout,
		TLSClientConfig: &tls.Config{
			MinVersion: tlsMinVersion,
		},
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
	}
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

func (c *ClientImpl) Get(url string) (*http.Response, error) {
	fullURL := c.scheme + "://" + c.hostname + ":" + c.port + "/" + strings.TrimPrefix(url, "/")
	return c.client.Get(fullURL)
}

func (c *ClientImpl) Post(url string, bodyType string, body string) (*http.Response, error) {
	fullURL := c.scheme + "://" + c.hostname + ":" + c.port + "/" + strings.TrimPrefix(url, "/")
	req, err := http.NewRequest("POST", fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", bodyType)
	req.Body = io.NopCloser(strings.NewReader(body))
	return c.client.Do(req)
}
