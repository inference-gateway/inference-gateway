// Code generated from OpenAPI schema. DO NOT EDIT.
package client

import (
	"time"
)

type ClientConfig struct {
	ClientMaxIdleConns          int           `env:"CLIENT_MAX_IDLE_CONNS, default=20" description:"Maximum idle connections"`
	ClientMaxIdleConnsPerHost   int           `env:"CLIENT_MAX_IDLE_CONNS_PER_HOST, default=20" description:"Maximum idle connections per host"`
	ClientIdleConnTimeout       time.Duration `env:"CLIENT_IDLE_CONN_TIMEOUT, default=30s" description:"Idle connection timeout"`
	ClientTlsMinVersion         string        `env:"CLIENT_TLS_MIN_VERSION, default=TLS12" description:"Minimum TLS version"`
	ClientDisableCompression    bool          `env:"CLIENT_DISABLE_COMPRESSION, default=true" description:"Disable compression for faster streaming"`
	ClientResponseHeaderTimeout time.Duration `env:"CLIENT_RESPONSE_HEADER_TIMEOUT, default=10s" description:"Response header timeout"`
	ClientExpectContinueTimeout time.Duration `env:"CLIENT_EXPECT_CONTINUE_TIMEOUT, default=1s" description:"Expect continue timeout"`
	ClientTlsCaPath             string        `env:"CLIENT_TLS_CA_PATH, default=" description:"Extra CA bundle trusted for provider calls, added on top of the system roots"`
	ClientTlsCertPath           string        `env:"CLIENT_TLS_CERT_PATH, default=" description:"Client certificate presented to providers that require mutual TLS"`
	ClientTlsKeyPath            string        `env:"CLIENT_TLS_KEY_PATH, default=" description:"Private key for CLIENT_TLS_CERT_PATH"`
}
