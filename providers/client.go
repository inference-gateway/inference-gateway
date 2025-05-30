// Code generated from OpenAPI schema. DO NOT EDIT.
package providers

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sethvargo/go-envconfig"
)

//go:generate mockgen -source=client.go -destination=../tests/mocks/client.go -package=mocks
type Client interface {
	Do(req *http.Request) (*http.Response, error)
	Get(url string) (*http.Response, error)
	Post(url string, bodyType string, body string) (*http.Response, error)
}

type ClientImpl struct {
	scheme   string
	hostname string
	port     string
	client   *http.Client
}

type ClientConfig struct {
	ClientTimeout               time.Duration `env:"CLIENT_TIMEOUT, default=30s" description:"Client timeout"`
	ClientMaxIdleConns          int           `env:"CLIENT_MAX_IDLE_CONNS, default=20" description:"Maximum idle connections"`
	ClientMaxIdleConnsPerHost   int           `env:"CLIENT_MAX_IDLE_CONNS_PER_HOST, default=20" description:"Maximum idle connections per host"`
	ClientIdleConnTimeout       time.Duration `env:"CLIENT_IDLE_CONN_TIMEOUT, default=30s" description:"Idle connection timeout"`
	ClientTlsMinVersion         string        `env:"CLIENT_TLS_MIN_VERSION, default=TLS12" description:"Minimum TLS version"`
	ClientDisableCompression    bool          `env:"CLIENT_DISABLE_COMPRESSION, default=true" description:"Disable compression for faster streaming"`
	ClientResponseHeaderTimeout time.Duration `env:"CLIENT_RESPONSE_HEADER_TIMEOUT, default=10s" description:"Response header timeout"`
	ClientExpectContinueTimeout time.Duration `env:"CLIENT_EXPECT_CONTINUE_TIMEOUT, default=1s" description:"Expect continue timeout"`
}

func NewClientConfig() (*ClientConfig, error) {
	var cfg ClientConfig
	if err := envconfig.Process(context.Background(), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func NewHTTPClient(cfg *ClientConfig, scheme, hostname, port string) Client {
	var tlsMinVersion uint16 = tls.VersionTLS12
	if cfg.ClientTlsMinVersion == "TLS13" {
		tlsMinVersion = tls.VersionTLS13
	}

	httpClient := &http.Client{
		Timeout: cfg.ClientTimeout,
		Transport: &http.Transport{
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
		},
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
