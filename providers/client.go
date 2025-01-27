package providers

import (
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"time"
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

func NewClient(scheme, hostname, port string, timeout time.Duration, transport *http.Transport) Client {
	return &ClientImpl{
		scheme:   scheme,
		hostname: hostname,
		port:     port,
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				// Increase connection pool for parallel requests
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				// Add reasonable timeouts
				IdleConnTimeout:   timeout,
				DisableKeepAlives: false,
				// TLS configuration for HTTPS
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
				// Enable HTTP/2 support
				ForceAttemptHTTP2: true,
			},
		},
	}
}

func NewTransport(timeout time.Duration) *http.Transport {
	return &http.Transport{
		// Increase connection pool for parallel requests
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		// Add reasonable timeouts
		IdleConnTimeout:   timeout,
		DisableKeepAlives: false,
		// TLS configuration for HTTPS
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		// Enable HTTP/2 support
		ForceAttemptHTTP2: true,
	}
}

func (c *ClientImpl) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

func (c *ClientImpl) Get(url string) (*http.Response, error) {
	fullURL := c.scheme + "://" + c.hostname + ":" + c.port + url
	return c.client.Get(fullURL)
}

func (c *ClientImpl) Post(url string, bodyType string, body string) (*http.Response, error) {
	fullURL := c.scheme + "://" + c.hostname + ":" + c.port + url
	req, err := http.NewRequest("POST", fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", bodyType)
	req.Body = io.NopCloser(strings.NewReader(body))
	return c.client.Do(req)
}
