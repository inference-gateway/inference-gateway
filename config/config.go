package config

import (
	"context"
	"strings"
	"time"

	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/sethvargo/go-envconfig"
)

func (c *Config) GetProviders() []providers.Provider {
	providerList := make([]providers.Provider, 0, len(c.Providers))
	for _, provider := range c.Providers {
		providerList = append(providerList, &providers.ProviderImpl{
			ID:           provider.ID,
			Name:         provider.Name,
			URL:          provider.URL,
			Token:        provider.Token,
			AuthType:     provider.AuthType,
			ExtraHeaders: provider.ExtraHeaders,
		})
	}
	return providerList
}

func (c *Config) GetProvider(name string) providers.Provider {
	if provider, ok := c.Providers[name]; ok {
		return &providers.ProviderImpl{
			ID:           provider.ID,
			Name:         provider.Name,
			URL:          provider.URL,
			Token:        provider.Token,
			AuthType:     provider.AuthType,
			ExtraHeaders: provider.ExtraHeaders,
		}
	}
	return nil
}

func (c *Config) SupportedProvider(name string) bool {
	_, ok := c.Providers[name]
	return ok
}

// Base provider configuration
type BaseProviderConfig struct {
	ID           string
	Name         string
	URL          string
	Token        string
	AuthType     string
	ExtraHeaders map[string][]string
	Endpoints    struct {
		List     string
		Generate string
	}
}

func (p *BaseProviderConfig) GetExtraHeaders() map[string][]string {
	return p.ExtraHeaders
}

func (p *BaseProviderConfig) EndpointList() string {
	return p.Endpoints.List
}

func (p *BaseProviderConfig) EndpointGenerate() string {
	return p.Endpoints.Generate
}

// Config holds the configuration for the Inference Gateway.
//
//go:generate go run ../cmd/generate/main.go -type=Env -output=../examples/docker-compose/.env.example
//go:generate go run ../cmd/generate/main.go -type=ConfigMap -output=../examples/kubernetes/basic/inference-gateway/configmap.yaml
//go:generate go run ../cmd/generate/main.go -type=Secret -output=../examples/kubernetes/basic/inference-gateway/secret.yaml
//go:generate go run ../cmd/generate/main.go -type=ConfigMap -output=../examples/kubernetes/hybrid/inference-gateway/configmap.yaml
//go:generate go run ../cmd/generate/main.go -type=Secret -output=../examples/kubernetes/hybrid/inference-gateway/secret.yaml
//go:generate go run ../cmd/generate/main.go -type=ConfigMap -output=../examples/kubernetes/authentication/inference-gateway/configmap.yaml
//go:generate go run ../cmd/generate/main.go -type=Secret -output=../examples/kubernetes/authentication/inference-gateway/secret.yaml
//go:generate go run ../cmd/generate/main.go -type=ConfigMap -output=../examples/kubernetes/agent/inference-gateway/configmap.yaml
//go:generate go run ../cmd/generate/main.go -type=Secret -output=../examples/kubernetes/agent/inference-gateway/secret.yaml
//go:generate go run ../cmd/generate/main.go -type=MD -output=../Configurations.md
type Config struct {
	// General settings
	ApplicationName string `env:"APPLICATION_NAME, default=inference-gateway" description:"The name of the application"`
	Environment     string `env:"ENVIRONMENT, default=production" description:"The environment in which the application is running"`
	EnableTelemetry bool   `env:"ENABLE_TELEMETRY, default=false" description:"Enable telemetry for the server"`
	EnableAuth      bool   `env:"ENABLE_AUTH, default=false" description:"Enable authentication"`

	// Auth settings
	OIDC *OIDC `env:", prefix=OIDC_" description:"The OIDC configuration"`

	// Server settings
	Server *ServerConfig `env:", prefix=SERVER_" description:"The configuration for the server"`

	// Providers settings
	Providers map[string]*BaseProviderConfig
}

// OIDC holds the configuration for the OIDC provider
type OIDC struct {
	IssuerURL    string `env:"ISSUER_URL, default=http://keycloak:8080/realms/inference-gateway-realm" description:"The OIDC issuer URL"`
	ClientID     string `env:"CLIENT_ID, default=inference-gateway-client" type:"secret" description:"The OIDC client ID"`
	ClientSecret string `env:"CLIENT_SECRET" type:"secret" description:"The OIDC client secret"`
}

// ServerConfig holds the configuration for the server
type ServerConfig struct {
	Host         string        `env:"HOST, default=0.0.0.0" description:"The host address for the server"`
	Port         string        `env:"PORT, default=8080" description:"The port on which the server will listen"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT, default=30s" description:"The server read timeout"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT, default=30s" description:"The server write timeout"`
	IdleTimeout  time.Duration `env:"IDLE_TIMEOUT, default=120s" description:"The server idle timeout"`
	TLSCertPath  string        `env:"TLS_CERT_PATH" description:"The path to the TLS certificate"`
	TLSKeyPath   string        `env:"TLS_KEY_PATH" description:"The path to the TLS key"`
}

// Load loads the configuration from environment variables
func (cfg *Config) Load(lookuper envconfig.Lookuper) (Config, error) {
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target:   cfg,
		Lookuper: lookuper,
	}); err != nil {
		return Config{}, err
	}

	// Set provider defaults if not configured
	defaultProviders := map[string]BaseProviderConfig{
		providers.AnthropicID: {
			ID:       providers.AnthropicID,
			Name:     "Anthropic",
			URL:      "https://api.anthropic.com",
			AuthType: "xheader",
			ExtraHeaders: map[string][]string{
				"anthropic-version": {"2023-06-01"},
			},
		},
		providers.OpenaiID: {
			ID:       providers.OpenaiID,
			Name:     "Openai",
			URL:      "https://api.openai.com",
			AuthType: "bearer",
		},
		providers.GoogleID: {
			ID:       providers.GoogleID,
			Name:     "Google",
			URL:      "https://generativelanguage.googleapis.com",
			AuthType: "query",
		},
		providers.CloudflareID: {
			ID:       providers.CloudflareID,
			Name:     "Cloudflare",
			URL:      "https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}",
			AuthType: "bearer",
		},
		providers.CohereID: {
			ID:       providers.CohereID,
			Name:     "Cohere",
			URL:      "https://api.cohere.com",
			AuthType: "bearer",
		},
		providers.GroqID: {
			ID:       providers.GroqID,
			Name:     "Groq",
			URL:      "https://api.groq.com",
			AuthType: "bearer",
		},
		providers.OllamaID: {
			ID:       providers.OllamaID,
			Name:     "Ollama",
			URL:      "http://ollama:8080",
			AuthType: "none",
		},
	}

	// Initialize Providers map if nil
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]*BaseProviderConfig)
	}

	// Set defaults for each provider
	for id, defaults := range defaultProviders {
		if _, exists := cfg.Providers[id]; !exists {
			providerCfg := defaults // Create copy
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
