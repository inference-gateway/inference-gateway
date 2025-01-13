package config

import (
	"context"
	"time"

	"github.com/sethvargo/go-envconfig"
)

// Config holds the configuration for the Inference Gateway.
type Config struct {
	// General settings
	ApplicationName  string `env:"APPLICATION_NAME, default=inference-gateway"`
	EnableTelemetry  bool   `env:"ENABLE_TELEMETRY, default=false"`
	Environment      string `env:"ENVIRONMENT, default=production"`
	EnableAuth       bool   `env:"ENABLE_AUTH, default=false"`
	OIDCIssuerURL    string `env:"OIDC_ISSUER_URL, default=http://keycloak:8080/realms/inference-gateway-realm"`
	OIDCClientID     string `env:"OIDC_CLIENT_ID, default=inference-gateway-client"`
	OIDCClientSecret string `env:"OIDC_CLIENT_SECRET"`

	// Server settings
	ServerHost         string        `env:"SERVER_HOST, default=127.0.0.1"`
	ServerPort         string        `env:"SERVER_PORT, default=8080"`
	ServerReadTimeout  time.Duration `env:"SERVER_READ_TIMEOUT, default=30s"`
	ServerWriteTimeout time.Duration `env:"SERVER_WRITE_TIMEOUT, default=30s"`
	ServerIdleTimeout  time.Duration `env:"SERVER_IDLE_TIMEOUT, default=120s"`
	ServerTLSCertPath  string        `env:"SERVER_TLS_CERT_PATH"`
	ServerTLSKeyPath   string        `env:"SERVER_TLS_KEY_PATH"`

	// API URLs and keys
	OllamaAPIURL      string `env:"OLLAMA_API_URL, default=http://ollama:8080"`
	GroqAPIURL        string `env:"GROQ_API_URL, default=https://api.groq.com"`
	GroqAPIKey        string `env:"GROQ_API_KEY"`
	OpenaiAPIURL      string `env:"OPENAI_API_URL, default=https://api.openai.com"`
	OpenaiAPIKey      string `env:"OPENAI_API_KEY"`
	GoogleAIStudioURL string `env:"GOOGLE_AISTUDIO_API_URL, default=https://generativelanguage.googleapis.com"`
	GoogleAIStudioKey string `env:"GOOGLE_AISTUDIO_API_KEY"`
	CloudflareAPIURL  string `env:"CLOUDFLARE_API_URL, default=https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}"`
	CloudflareAPIKey  string `env:"CLOUDFLARE_API_KEY"`
}

// Load loads the configuration from environment variables.
func (cfg *Config) Load() (Config, error) {
	if err := envconfig.Process(context.Background(), cfg); err != nil {
		return Config{}, err
	}
	return *cfg, nil
}
