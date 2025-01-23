package config

import (
	"context"
	"time"

	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/sethvargo/go-envconfig"
)

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
	ApplicationName  string `env:"APPLICATION_NAME, default=inference-gateway" description:"The name of the application"`
	EnableTelemetry  bool   `env:"ENABLE_TELEMETRY, default=false" description:"Enable telemetry for the server"`
	Environment      string `env:"ENVIRONMENT, default=production" description:"The environment in which the application is running"`
	EnableAuth       bool   `env:"ENABLE_AUTH, default=false" description:"Enable authentication"`
	OIDCIssuerURL    string `env:"OIDC_ISSUER_URL, default=http://keycloak:8080/realms/inference-gateway-realm" description:"The OIDC issuer URL"`
	OIDCClientID     string `env:"OIDC_CLIENT_ID, default=inference-gateway-client" type:"secret" description:"The OIDC client ID"`
	OIDCClientSecret string `env:"OIDC_CLIENT_SECRET" type:"secret" description:"The OIDC client secret"`

	// Server settings
	ServerHost         string        `env:"SERVER_HOST, default=0.0.0.0" description:"The host address for the server"`
	ServerPort         string        `env:"SERVER_PORT, default=8080" description:"The port on which the server will listen"`
	ServerReadTimeout  time.Duration `env:"SERVER_READ_TIMEOUT, default=30s" description:"The server read timeout"`
	ServerWriteTimeout time.Duration `env:"SERVER_WRITE_TIMEOUT, default=30s" description:"The server write timeout"`
	ServerIdleTimeout  time.Duration `env:"SERVER_IDLE_TIMEOUT, default=120s" description:"The server idle timeout"`
	ServerTLSCertPath  string        `env:"SERVER_TLS_CERT_PATH" description:"The path to the TLS certificate"`
	ServerTLSKeyPath   string        `env:"SERVER_TLS_KEY_PATH" description:"The path to the TLS key"`

	// API URLs and keys
	OllamaAPIURL      string `env:"OLLAMA_API_URL, default=http://ollama:8080" description:"The URL for Ollama API"`
	GroqAPIURL        string `env:"GROQ_API_URL, default=https://api.groq.com" description:"The URL for Groq Cloud API"`
	GroqAPIKey        string `env:"GROQ_API_KEY" type:"secret" description:"The Access token for Groq Cloud API"`
	OpenaiAPIURL      string `env:"OPENAI_API_URL, default=https://api.openai.com" description:"The URL for OpenAI API"`
	OpenaiAPIKey      string `env:"OPENAI_API_KEY" type:"secret" description:"The Access token for OpenAI API"`
	GoogleAIStudioURL string `env:"GOOGLE_AISTUDIO_API_URL, default=https://generativelanguage.googleapis.com" description:"The URL for Google AI Studio API"`
	GoogleAIStudioKey string `env:"GOOGLE_AISTUDIO_API_KEY" type:"secret" description:"The Access token for Google AI Studio API"`
	CloudflareAPIURL  string `env:"CLOUDFLARE_API_URL, default=https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}" description:"The URL for Cloudflare API"`
	CloudflareAPIKey  string `env:"CLOUDFLARE_API_KEY" type:"secret" description:"The Access token for Cloudflare API"`
	CohereAPIURL      string `env:"COHERE_API_URL, default=https://api.cohere.com" description:"The URL for Cohere API"`
	CohereAPIKey      string `env:"COHERE_API_KEY" type:"secret" description:"The Access token for Cohere API"`
	AnthropicAPIURL   string `env:"ANTHROPIC_API_URL, default=https://api.anthropic.com" description:"The URL for Anthropic API"`
	AnthropicAPIKey   string `env:"ANTHROPIC_API_KEY" type:"secret" description:"The Access token for Anthropic API"`
}

func (cfg *Config) Providers() map[string]providers.Provider {
	return map[string]providers.Provider{
		"ollama":     {ID: "ollama", Name: "Ollama", URL: cfg.OllamaAPIURL, ProxyURL: "http://localhost:8080/proxy/ollama", Token: ""},
		"groq":       {ID: "groq", Name: "Groq", URL: cfg.GroqAPIURL, ProxyURL: "http://localhost:8080/proxy/groq", Token: cfg.GroqAPIKey},
		"openai":     {ID: "openai", Name: "OpenAI", URL: cfg.OpenaiAPIURL, ProxyURL: "http://localhost:8080/proxy/openai", Token: cfg.OpenaiAPIKey},
		"google":     {ID: "google", Name: "Google", URL: cfg.GoogleAIStudioURL, ProxyURL: "http://localhost:8080/proxy/google", Token: cfg.GoogleAIStudioKey},
		"cloudflare": {ID: "cloudflare", Name: "Cloudflare", URL: cfg.CloudflareAPIURL, ProxyURL: "http://localhost:8080/proxy/cloudflare", Token: cfg.CloudflareAPIKey},
		"cohere":     {ID: "cohere", Name: "Cohere", URL: cfg.CohereAPIURL, ProxyURL: "http://localhost:8080/proxy/cohere", Token: cfg.CohereAPIKey},
		"anthropic":  {ID: "anthropic", Name: "Anthropic", URL: cfg.AnthropicAPIURL, ProxyURL: "http://localhost:8080/proxy/anthropic", Token: cfg.AnthropicAPIKey},
	}
}

var listEndpoints = map[string]string{
	"ollama":     "/v1/models",
	"groq":       "/openai/v1/models",
	"openai":     "/v1/models",
	"google":     "/v1beta/models",
	"cloudflare": "/ai/finetunes/public",
	"cohere":     "/v1/models",
	"anthropic":  "/v1/models",
}

func (cfg *Config) GetEndpointsListModels() map[string]string {
	return listEndpoints
}

var generateEndpoints = map[string]string{
	"ollama":     "/api/generate",
	"groq":       "/openai/v1/chat/completions",
	"openai":     "/v1/completions",
	"google":     "/v1beta/models/{model}:generateContent",
	"cloudflare": "/ai/run/@cf/meta/{model}",
	"cohere":     "/v2/chat",
	"anthropic":  "/v1/messages",
}

func (cfg *Config) GetEndpointsGenerateTokens(providerID string) string {
	return generateEndpoints[providerID]
}

// Load loads the configuration from environment variables.
func (cfg *Config) Load() (Config, error) {
	if err := envconfig.Process(context.Background(), cfg); err != nil {
		return Config{}, err
	}
	return *cfg, nil
}
