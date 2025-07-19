// Code generated from OpenAPI schema. DO NOT EDIT.
package config

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/sethvargo/go-envconfig"
)

// Config holds the configuration for the Inference Gateway
type Config struct {
	// General settings
	Environment     string `env:"ENVIRONMENT, default=production" description:"The environment"`
	EnableTelemetry bool   `env:"ENABLE_TELEMETRY, default=false" description:"Enable telemetry"`
	EnableAuth      bool   `env:"ENABLE_AUTH, default=false" description:"Enable authentication"`
	AllowedModels   string `env:"ALLOWED_MODELS" description:"Comma-separated list of models to allow. If empty, all models will be available"`
	// MCP settings
	MCP *MCPConfig `env:", prefix=MCP_" description:"MCP configuration"`
	// A2A settings
	A2A *A2AConfig `env:", prefix=A2A_" description:"A2A configuration"`
	// OIDC settings
	OIDC *OIDC `env:", prefix=OIDC_" description:"OIDC configuration"`
	// Server settings
	Server *ServerConfig `env:", prefix=SERVER_" description:"Server configuration"`
	// Client settings
	Client *ClientConfig `env:", prefix=CLIENT_" description:"Client configuration"`

	// Providers map
	Providers map[providers.Provider]*providers.Config
}

// MCP configuration
type MCPConfig struct {
	Enable                bool          `env:"ENABLE, default=false" description:"Enable MCP"`
	Expose                bool          `env:"EXPOSE, default=false" description:"Expose MCP tools endpoint"`
	Servers               string        `env:"SERVERS" description:"List of MCP servers"`
	ClientTimeout         time.Duration `env:"CLIENT_TIMEOUT, default=5s" description:"MCP client HTTP timeout"`
	DialTimeout           time.Duration `env:"DIAL_TIMEOUT, default=3s" description:"MCP client dial timeout"`
	TlsHandshakeTimeout   time.Duration `env:"TLS_HANDSHAKE_TIMEOUT, default=3s" description:"MCP client TLS handshake timeout"`
	ResponseHeaderTimeout time.Duration `env:"RESPONSE_HEADER_TIMEOUT, default=3s" description:"MCP client response header timeout"`
	ExpectContinueTimeout time.Duration `env:"EXPECT_CONTINUE_TIMEOUT, default=1s" description:"MCP client expect continue timeout"`
	RequestTimeout        time.Duration `env:"REQUEST_TIMEOUT, default=5s" description:"MCP client request timeout for initialize and tool calls"`
}

// A2A configuration
type A2AConfig struct {
	Enable                 bool          `env:"ENABLE, default=false" description:"Enable A2A protocol support"`
	Expose                 bool          `env:"EXPOSE, default=false" description:"Expose A2A agents list cards endpoint"`
	Agents                 string        `env:"AGENTS" description:"Comma-separated list of A2A agent URLs"`
	ClientTimeout          time.Duration `env:"CLIENT_TIMEOUT, default=30s" description:"A2A client timeout"`
	PollingEnable          bool          `env:"POLLING_ENABLE, default=true" description:"Enable task status polling"`
	PollingInterval        time.Duration `env:"POLLING_INTERVAL, default=1s" description:"Interval between polling requests"`
	PollingTimeout         time.Duration `env:"POLLING_TIMEOUT, default=30s" description:"Maximum time to wait for task completion"`
	MaxPollAttempts        int           `env:"MAX_POLL_ATTEMPTS, default=30" description:"Maximum number of polling attempts"`
	MaxRetries             int           `env:"MAX_RETRIES, default=3" description:"Maximum number of connection retry attempts"`
	RetryInterval          time.Duration `env:"RETRY_INTERVAL, default=5s" description:"Interval between connection retry attempts"`
	InitialBackoff         time.Duration `env:"INITIAL_BACKOFF, default=1s" description:"Initial backoff duration for exponential backoff retry"`
	EnableReconnect        bool          `env:"ENABLE_RECONNECT, default=true" description:"Enable automatic reconnection for failed agents"`
	ReconnectInterval      time.Duration `env:"RECONNECT_INTERVAL, default=30s" description:"Interval between reconnection attempts"`
	DisableHealthcheckLogs bool          `env:"DISABLE_HEALTHCHECK_LOGS, default=true" description:"Disable health check log messages to reduce noise"`
}

// OIDC configuration
type OIDC struct {
	IssuerUrl    string `env:"ISSUER_URL, default=http://keycloak:8080/realms/inference-gateway-realm" description:"OIDC issuer URL"`
	ClientId     string `env:"CLIENT_ID, default=inference-gateway-client" type:"secret" description:"OIDC client ID"`
	ClientSecret string `env:"CLIENT_SECRET" type:"secret" description:"OIDC client secret"`
}

// Server configuration
type ServerConfig struct {
	Host         string        `env:"HOST, default=0.0.0.0" description:"Server host"`
	Port         string        `env:"PORT, default=8080" description:"Server port"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT, default=30s" description:"Read timeout"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT, default=30s" description:"Write timeout"`
	IdleTimeout  time.Duration `env:"IDLE_TIMEOUT, default=120s" description:"Idle timeout"`
	TlsCertPath  string        `env:"TLS_CERT_PATH" description:"TLS certificate path"`
	TlsKeyPath   string        `env:"TLS_KEY_PATH" description:"TLS key path"`
}

// Client configuration
type ClientConfig struct {
	Timeout               time.Duration `env:"TIMEOUT, default=30s" description:"Client timeout"`
	MaxIdleConns          int           `env:"MAX_IDLE_CONNS, default=20" description:"Maximum idle connections"`
	MaxIdleConnsPerHost   int           `env:"MAX_IDLE_CONNS_PER_HOST, default=20" description:"Maximum idle connections per host"`
	IdleConnTimeout       time.Duration `env:"IDLE_CONN_TIMEOUT, default=30s" description:"Idle connection timeout"`
	TlsMinVersion         string        `env:"TLS_MIN_VERSION, default=TLS12" description:"Minimum TLS version"`
	DisableCompression    bool          `env:"DISABLE_COMPRESSION, default=true" description:"Disable compression for faster streaming"`
	ResponseHeaderTimeout time.Duration `env:"RESPONSE_HEADER_TIMEOUT, default=10s" description:"Response header timeout"`
	ExpectContinueTimeout time.Duration `env:"EXPECT_CONTINUE_TIMEOUT, default=1s" description:"Expect continue timeout"`
}

// Load configuration
func (cfg *Config) Load(lookuper envconfig.Lookuper) (Config, error) {
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target:   cfg,
		Lookuper: lookuper,
	}); err != nil {
		return Config{}, err
	}

	// Initialize Providers map if nil
	if cfg.Providers == nil {
		cfg.Providers = make(map[providers.Provider]*providers.Config)
	}

	// Set defaults for each provider
	for id, defaults := range providers.Registry {
		if _, exists := cfg.Providers[id]; !exists {
			providerCfg := defaults
			url, ok := lookuper.Lookup(strings.ToUpper(string(id)) + "_API_URL")
			if ok {
				providerCfg.URL = url
			}

			token, ok := lookuper.Lookup(strings.ToUpper(string(id)) + "_API_KEY")
			if (!ok || token == "") && id != providers.OllamaID {
				t := time.Now().UTC().Format(time.RFC3339)
				log.SetFlags(0)
				log.Printf("{\"level\":\"notice\",\"timestamp\":\"%s\",\"caller\":\"config/config.go:103\",\"msg\":\"provider is not configured\",\"provider\":\"%s\"}", t, string(id))
			}
			providerCfg.Token = token
			cfg.Providers[id] = providerCfg
		}
	}

	return *cfg, nil
}

// The string representation of Config
func (cfg *Config) String() string {
	return fmt.Sprintf(
		"Config{ApplicationName:%s, Version:%s Environment:%s, EnableTelemetry:%t, EnableAuth:%t, "+
			"MCP:%+v, A2A:%+v, OIDC:%+v, Server:%+v, Client:%+v, Providers:%+v}",
		APPLICATION_NAME,
		VERSION,
		cfg.Environment,
		cfg.EnableTelemetry,
		cfg.EnableAuth,
		cfg.MCP,
		cfg.A2A,
		cfg.OIDC,
		cfg.Server,
		cfg.Client,
		cfg.Providers,
	)
}
