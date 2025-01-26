package config

import (
	"context"
	"strings"
	"time"

	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/sethvargo/go-envconfig"
)

// Config holds the configuration for the Inference Gateway
type Config struct {
	// General settings
	ApplicationName string `env:"APPLICATION_NAME, default=inference-gateway" description:"The name of the application"`
	Environment     string `env:"ENVIRONMENT, default=production" description:"The environment"`
	EnableTelemetry bool   `env:"ENABLE_TELEMETRY, default=false" description:"Enable telemetry"`
	EnableAuth      bool   `env:"ENABLE_AUTH, default=false" description:"Enable authentication"`
	// OIDC settings
	OIDC *OIDC `env:", prefix=OIDC_" description:"OIDC configuration"`
	// Server settings
	Server *ServerConfig `env:", prefix=SERVER_" description:"Server configuration"`

	// Providers map
	Providers map[string]*providers.Config
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
		cfg.Providers = make(map[string]*providers.Config)
	}

	// Set defaults for each provider
	for id, defaults := range providers.Registry {
		if _, exists := cfg.Providers[id]; !exists {
			providerCfg := defaults
			url, ok := lookuper.Lookup(strings.ToUpper(id) + "_API_URL")
			if ok {
				providerCfg.URL = url
			}

			token, ok := lookuper.Lookup(strings.ToUpper(id) + "_API_KEY")
			if !ok {
				println("Warn: provider " + id + " is not configured")
			}
			providerCfg.Token = token
			cfg.Providers[id] = &providerCfg
		}
	}

	return *cfg, nil
}
