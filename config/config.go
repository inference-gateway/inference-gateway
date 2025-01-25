package config

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/sethvargo/go-envconfig"
)

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
	Environment     string `env:"ENVIRONMENT, default=production" description:"The environment"`
	EnableTelemetry bool   `env:"ENABLE_TELEMETRY, default=false" description:"Enable telemetry"`
	EnableAuth      bool   `env:"ENABLE_AUTH, default=false" description:"Enable authentication"`

	// Auth settings
	OIDC *OIDC `env:", prefix=OIDC_" description:"OIDC configuration"`

	// Server settings
	Server *ServerConfig `env:", prefix=SERVER_" description:"Server configuration"`

	// Providers map
	Providers map[string]*BaseProviderConfig
}

// OIDC configuration
type OIDC struct {
	IssuerURL    string `env:"ISSUER_URL, default=http://keycloak:8080/realms/inference-gateway-realm" description:"OIDC issuer URL"`
	ClientID     string `env:"CLIENT_ID, default=inference-gateway-client" type:"secret" description:"OIDC client ID"`
	ClientSecret string `env:"CLIENT_SECRET" type:"secret" description:"OIDC client secret"`
}

// Server configuration
type ServerConfig struct {
	Host         string        `env:"HOST, default=0.0.0.0" description:"Server host"`
	Port         string        `env:"PORT, default=8080" description:"Server port"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT, default=30s" description:"Read timeout"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT, default=30s" description:"Write timeout"`
	IdleTimeout  time.Duration `env:"IDLE_TIMEOUT, default=120s" description:"Idle timeout"`
	TLSCertPath  string        `env:"TLS_CERT_PATH" description:"TLS certificate path"`
	TLSKeyPath   string        `env:"TLS_KEY_PATH" description:"TLS key path"`
}

// GetProviders returns a list of providers
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

// GetProvider returns a provider by id
func (c *Config) GetProvider(id string) (providers.Provider, error) {
	provider, ok := c.Providers[id]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", id)
	}
	return &providers.ProviderImpl{
		ID:           provider.ID,
		Name:         provider.Name,
		URL:          provider.URL,
		Token:        provider.Token,
		AuthType:     provider.AuthType,
		ExtraHeaders: provider.ExtraHeaders,
	}, nil
}

// Load configuration
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
			Name:     providers.AnthropicDisplayName,
			URL:      providers.AnthropicDefaultBaseURL,
			AuthType: providers.AuthTypeXHeader,
			ExtraHeaders: map[string][]string{
				"anthropic-version": {"2023-06-01"},
			},
		},
		providers.CloudflareID: {
			ID:       providers.CloudflareID,
			Name:     providers.CloudflareDisplayName,
			URL:      providers.CloudflareDefaultBaseURL,
			AuthType: providers.AuthTypeBearer,
		},
		providers.CohereID: {
			ID:       providers.CohereID,
			Name:     providers.CohereDisplayName,
			URL:      providers.CohereDefaultBaseURL,
			AuthType: providers.AuthTypeBearer,
		},
		providers.GoogleID: {
			ID:       providers.GoogleID,
			Name:     providers.GoogleDisplayName,
			URL:      providers.GoogleDefaultBaseURL,
			AuthType: providers.AuthTypeQuery,
		},
		providers.GroqID: {
			ID:       providers.GroqID,
			Name:     providers.GroqDisplayName,
			URL:      providers.GroqDefaultBaseURL,
			AuthType: providers.AuthTypeBearer,
		},
		providers.OllamaID: {
			ID:       providers.OllamaID,
			Name:     providers.OllamaDisplayName,
			URL:      providers.OllamaDefaultBaseURL,
			AuthType: providers.AuthTypeNone,
		},
		providers.OpenaiID: {
			ID:       providers.OpenaiID,
			Name:     providers.OpenaiDisplayName,
			URL:      providers.OpenaiDefaultBaseURL,
			AuthType: providers.AuthTypeBearer,
		},
	}

	// Initialize Providers map if nil
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]*BaseProviderConfig)
	}

	// Set defaults for each provider
	for id, defaults := range defaultProviders {
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
